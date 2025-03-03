package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Size constants
const (
	MB                 = 1 << 20
	CommandDescription = "temp-file-registry is temporary file registry provided through an HTTP web API."
	UsageDummy         = "########"
	UrlPathPrefix      = "/temp-file-registry/api/v1"
)

var (
	// Command options ( the -h, --help option is defined by default in the flag package )
	commandOptionMaxLength = 0
	// Define boot arguments.
	argsPort           = defineFlagValue("p", "port", "Port", 8888).(*int)
	argsFileExpiration = defineFlagValue("e", "expiration-minutes", "Default file expiration (minutes)", 10).(*int)
	argsMaxFileSize    = defineFlagValue("m", "max-file-size-mb", "Max file size (MB)", int64(1024)).(*int64)
	argsLogLevel       = defineFlagValue("l", "log-level", "Log level (0:Panic, 1:Info, 2:Debug)", 2).(*int)
	// Define application logic variables.
	fileRegistryMap = map[string]FileRegistry{}
	mutex           sync.Mutex
	// Define logger: date, time, microseconds, directory and file path are always outputted.
	logger         = log.New(os.Stdout, "[Logger] ", log.Llongfile|log.LstdFlags)
	loggerLogLevel = Debug
)

type LogLevel int

const (
	Panic LogLevel = iota
	Info
	Debug
)

// Level based logging in Golang https://www.linkedin.com/pulse/level-based-logging-golang-vivek-dasgupta
func logging(loglevel LogLevel, logLogger *log.Logger, v ...interface{}) {
	if loggerLogLevel < loglevel {
		return
	}
	level := func() string {
		switch loggerLogLevel {
		case Panic:
			return "Panic"
		case Info:
			return "Info"
		case Debug:
			return "Debug"
		default:
			return "Unknown"
		}
	}()
	logLogger.Println(append([]interface{}{"[" + level + "]"}, v...)...)
	if loggerLogLevel == Panic {
		logLogger.Panic(v...)
	}
}

type FileRegistry struct {
	key                 string
	expiryTimeMinutes   string
	expiredAt           time.Time
	multipartFile       multipart.File
	multipartFileHeader *multipart.FileHeader
}

func (fr FileRegistry) String() string {
	return fmt.Sprintf("key:%v, expiryTimeMinutes:%v, expiredAt:%v, multipartFileHeader.Header:%v", fr.key, fr.expiryTimeMinutes, fr.expiredAt, fr.multipartFileHeader.Header)
}

func init() {
	formatUsage(CommandDescription, &commandOptionMaxLength, new(bytes.Buffer))
}

func main() {

	//-------------------------
	// 引数のパース
	flag.Parse()

	// set log level
	loggerLogLevel = LogLevel(*argsLogLevel)

	//-------------------------
	// 各パスの処理
	// upload
	http.HandleFunc(UrlPathPrefix+"/upload", handleUpload)
	// download
	http.HandleFunc(UrlPathPrefix+"/download", handleDownload)

	//-------------------------
	// 期限切れのファイルを削除するgoroutineを起動
	go cleanExpiredFile()

	//-------------------------
	// Listen開始
	logging(Info, logger, "server(http)", *argsPort)
	logging(Info, logger, `Start application:
######## ######## ##     ## ########     ######## #### ##       ########    ########  ########  ######   ####  ######  ######## ########  ##    ## 
   ##    ##       ###   ### ##     ##    ##        ##  ##       ##          ##     ## ##       ##    ##   ##  ##    ##    ##    ##     ##  ##  ##  
   ##    ##       #### #### ##     ##    ##        ##  ##       ##          ##     ## ##       ##         ##  ##          ##    ##     ##   ####   
   ##    ######   ## ### ## ########     ######    ##  ##       ######      ########  ######   ##   ####  ##   ######     ##    ########     ##    
   ##    ##       ##     ## ##           ##        ##  ##       ##          ##   ##   ##       ##    ##   ##        ##    ##    ##   ##      ##    
   ##    ##       ##     ## ##           ##        ##  ##       ##          ##    ##  ##       ##    ##   ##  ##    ##    ##    ##    ##     ##    
   ##    ######## ##     ## ##           ##       #### ######## ########    ##     ## ########  ######   ####  ######     ##    ##     ##    ##`)
	if err := http.ListenAndServe(":"+strconv.Itoa(*argsPort), nil); err != nil {
		logging(Panic, logger, err)
	}
}

// Upload処理：POSTされたファイルをアプリ内部のmapに保持する
func handleUpload(w http.ResponseWriter, r *http.Request) {
	logging(Debug, logger, r.RemoteAddr, r.RequestURI, r.Header)
	// - [How can I handle http requests of different methods to / in Go? - Stack Overflow](https://stackoverflow.com/questions/15240884/how-can-i-handle-http-requests-of-different-methods-to-in-go)
	if allowedHttpMethod := http.MethodPost; r.Method != allowedHttpMethod {
		responseJson(w, 405, `{"message":"Method Not Allowed. (Only `+allowedHttpMethod+` is allowed)"}`)
		return
	}

	// go - golang - How to check multipart.File information - Stack Overflow
	// https://stackoverflow.com/questions/17129797/golang-how-to-check-multipart-file-information
	if err := r.ParseMultipartForm(*argsMaxFileSize * MB); err != nil {
		responseJson(w, 400, `{"message":"`+err.Error()+`"}`)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, *argsMaxFileSize*MB)

	key := r.FormValue("key")
	expiryTimeMinutes := r.FormValue("expiryTimeMinutes")
	fileExpirationMinutes := *argsFileExpiration
	// expiryTimeMinutesで指定された数値がintにキャストできる値だった場合は、ファイルの期限を指定分に設定する
	if expiryTimeMinutesInt, err := strconv.Atoi(expiryTimeMinutes); err == nil {
		fileExpirationMinutes = expiryTimeMinutesInt
	}
	file, fileHeader, _ := r.FormFile("file")
	mutex.Lock()
	defer func() { mutex.Unlock() }()
	fileRegistryMap[key] = FileRegistry{
		key:                 key,
		expiryTimeMinutes:   expiryTimeMinutes,
		expiredAt:           time.Now().Add(time.Duration(fileExpirationMinutes) * time.Minute),
		multipartFile:       file,
		multipartFileHeader: fileHeader,
	}
	responseBody := `{"message":"` + fileRegistryMap[key].String() + `"}`
	logging(Debug, logger, responseBody)
	responseJson(w, 200, responseBody)
}

// Download処理：key指定されたファイルをレスポンスする
func handleDownload(w http.ResponseWriter, r *http.Request) {
	logging(Debug, logger, r.RemoteAddr, r.RequestURI, r.Header)
	if allowedHttpMethod := http.MethodGet; r.Method != allowedHttpMethod {
		responseJson(w, 405, `{"message":"Method Not Allowed. (Only `+allowedHttpMethod+` is allowed)"}`)
		return
	}
	key := r.URL.Query().Get("key")
	deleteFlag := r.URL.Query().Get("delete")
	mutex.Lock()
	defer func() { mutex.Unlock() }()
	if _, ok := fileRegistryMap[key]; !ok {
		responseJson(w, 404, `{"message":"file not found."}`)
		return
	}
	logging(Debug, logger, fileRegistryMap[key].String())
	w.WriteHeader(200)
	w.Header().Set("Content-Type", fileRegistryMap[key].multipartFileHeader.Header.Get("Content-Type"))
	w.Header().Set("Content-Disposition", "attachment; filename="+fileRegistryMap[key].multipartFileHeader.Filename)
	io.Copy(w, fileRegistryMap[key].multipartFile)
	// reset
	fileRegistryMap[key].multipartFile.Seek(0, io.SeekStart)
	// if specified "delete" parameter, target file will be deleted after response.
	if deleteFlag == "true" {
		delete(fileRegistryMap, key)
	}
}

// 期限切れのファイルをお掃除する
func cleanExpiredFile() {
	for {
		time.Sleep(time.Minute * 1)
		func() {
			mutex.Lock()
			defer func() { mutex.Unlock() }()
			for key, fileRegistry := range fileRegistryMap {
				if fileRegistry.expiredAt.Before(time.Now()) {
					logging(Debug, logger, "[File cleaner goroutine] File expired. >>", fileRegistry.String())
					delete(fileRegistryMap, key)
				}
			}
		}()
	}
}

func responseJson(w http.ResponseWriter, statusCode int, bodyJson string) {
	w.WriteHeader(statusCode)
	w.Header().Add("Content-Type", "application/json")
	fmt.Fprint(w, bodyJson)
}

// =======================================
// Internal Utils
// =======================================

// Helper function for flag
func defineFlagValue(short, long, description string, defaultValue any) (f any) {
	switch defaultValue.(type) {
	case string:
		f = flag.String(short, "", UsageDummy)
		flag.StringVar(f.(*string), long, defaultValue.(string), description)
	case int:
		f = flag.Int(short, 0, UsageDummy)
		flag.IntVar(f.(*int), long, defaultValue.(int), description)
	case int64:
		f = flag.Int64(short, 0, UsageDummy)
		flag.Int64Var(f.(*int64), long, defaultValue.(int64), description)
	case bool:
		f = flag.Bool(short, false, UsageDummy)
		flag.BoolVar(f.(*bool), long, defaultValue.(bool), description)
	default:
		panic("unsupported flag type")
	}
	return
}

func formatUsage(description string, maxLength *int, buffer *bytes.Buffer) {
	// Get default flags usage
	func() { flag.CommandLine.SetOutput(buffer); flag.Usage(); flag.CommandLine.SetOutput(os.Stderr) }()
	re := regexp.MustCompile("(-\\S+)( *\\S*)+\n*\\s+" + UsageDummy + ".*\n*\\s+(-\\S+)( *\\S*)+\n\\s+(.+)")
	usageFirst := strings.Replace(strings.Replace(strings.Split(buffer.String(), "\n")[0], ":", " [OPTIONS] [-h, --help]", -1), " of ", ": ", -1) + "\n\nDescription:\n  " + description + "\n\nOptions:\n"
	usageOptions := re.FindAllString(buffer.String(), -1)
	for _, v := range usageOptions {
		*maxLength = max(*maxLength, len(re.ReplaceAllString(v, "$1, -$3$4")))
	}
	usageOptionsRep := make([]string, 0)
	for _, v := range usageOptions {
		usageOptionsRep = append(usageOptionsRep, fmt.Sprintf("%-6s%-"+strconv.Itoa(*maxLength)+"s", re.ReplaceAllString(v, "  $1,"), re.ReplaceAllString(v, "-$3$4"))+re.ReplaceAllString(v, "$5\n"))
	}
	flag.Usage = func() { _, _ = fmt.Fprintf(flag.CommandLine.Output(), usageFirst+strings.Join(usageOptionsRep, "")) }
}
