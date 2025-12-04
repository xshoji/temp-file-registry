package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Size constants
const (
	MB            = 1 << 20
	UsageDummy    = "########"
	UrlPathPrefix = "/temp-file-registry/api/v1"
)

var (
	CommandDescription     = "temp-file-registry is temporary file registry provided through an HTTP web API."
	commandOptionMaxLength = 0
	// Command options (the -h and --help options are defined by default in the standard flag package)
	argsPort           = defineFlagValue("p", "port" /*               */, "Port" /*                                                       */, 8888, flag.Int, flag.IntVar)
	argsFileExpiration = defineFlagValue("e", "expiration-minutes" /* */, "Default file expiration (minutes)" /*                          */, 10, flag.Int, flag.IntVar)
	argsMaxFileSize    = defineFlagValue("m", "max-file-size-mb" /*   */, "Max file size (MB)" /*                                         */, int64(1024), flag.Int64, flag.Int64Var)
	argsLogLevel       = defineFlagValue("l", "log-level" /*          */, "Log level (-4:Debug, 0:Info, 4:Warn, 8:Error) (default 0)" /* */, 0, flag.Int, flag.IntVar)

	// Define application logic variables.
	fileRegistryMap = map[string]FileRegistry{}
	mutex           sync.Mutex
)

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
	// Time format = datetime + microsec, output file name: true
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Llongfile)

	// Set custom usage for flag
	flag.Usage = customUsage(os.Stderr, CommandDescription, strconv.Itoa(commandOptionMaxLength))
}

func main() {

	//-------------------------
	// 引数のパース
	flag.Parse()

	// Set log level
	slog.SetLogLoggerLevel(slog.Level(*argsLogLevel))

	slog.Info("\n[ Command options ]\n" + getOptionsUsage(strconv.Itoa(commandOptionMaxLength), true))

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
	slog.Info(fmt.Sprintf("server(http): %d", *argsPort))
	slog.Info("Start application:")
	printApplicationLogo()
	if err := http.ListenAndServe(":"+strconv.Itoa(*argsPort), nil); err != nil {
		slog.Error(err.Error())
	}
}

// Upload処理：POSTされたファイルをアプリ内部のmapに保持する
func handleUpload(w http.ResponseWriter, r *http.Request) {
	slog.Debug(r.RemoteAddr, r.RequestURI, r.Header)
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
	slog.Debug(responseBody)
	responseJson(w, 200, responseBody)
}

// Download処理：key指定されたファイルをレスポンスする
func handleDownload(w http.ResponseWriter, r *http.Request) {
	slog.Debug(r.RemoteAddr, r.RequestURI, r.Header)
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
	slog.Debug(fileRegistryMap[key].String())
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
					slog.Debug("[File cleaner goroutine] File expired. >>", "registry", fileRegistry.String())
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
// flag Utils
// =======================================

// Helper function for flag
func defineFlagValue[T comparable](short, long, description string, defaultValue T, flagFunc func(name string, value T, usage string) *T, flagVarFunc func(p *T, name string, value T, usage string)) *T {
	flagUsage := short + UsageDummy + description
	var zero T
	if defaultValue != zero {
		flagUsage = flagUsage + fmt.Sprintf(" (default %v)", defaultValue)
	}
	commandOptionMaxLength = max(commandOptionMaxLength, len(long)+8)
	f := flagFunc(long, defaultValue, flagUsage)
	flagVarFunc(f, short, defaultValue, UsageDummy)
	return f
}

// Custom usage message
func customUsage(output io.Writer, description, fieldWidth string) func() {
	return func() {
		fmt.Fprintf(output, "Usage: %s [OPTIONS]\n\n", func() string { e, _ := os.Executable(); return filepath.Base(e) }())
		fmt.Fprintf(output, "Description:\n  %s\n\n", description)
		fmt.Fprintf(output, "Options:\n%s", getOptionsUsage(fieldWidth, false))
	}
}

// Get options usage message
func getOptionsUsage(fieldWidth string, currentValue bool) string {
	optionUsages := make([]string, 0)
	flag.VisitAll(func(f *flag.Flag) {
		if f.Usage == UsageDummy {
			return
		}
		value := strings.NewReplacer("*flag.boolValue", "", "*flag.", "<", "Value", ">").Replace(fmt.Sprintf("%T", f.Value))
		if currentValue {
			value = f.Value.String()
		}
		format := "  -%-1s, --%-" + fieldWidth + "s %s\n"
		short := strings.Split(f.Usage, UsageDummy)[0]
		mainUsage := strings.Split(f.Usage, UsageDummy)[1]
		optionUsages = append(optionUsages, fmt.Sprintf(format, short, f.Name+" "+value, mainUsage))
	})
	return strings.Join(optionUsages, "")
}

func printApplicationLogo() {
	slog.Info("████████╗ ███████╗ ███╗   ███╗  ██████╗    ████████ ██╗  ██╗      ████████╗ ")
	slog.Info("╚══██╔══╝ ██╔════╝ ████╗ ████║  ██╔══██╗   ██╔════╝ ██║  ██║      ██╔════╝  ")
	slog.Info("   ██║    █████╗   ██╔████╔██║  ██████╔╝   ███████╗ ██║  ██║      ███████╗  ")
	slog.Info("   ██║    ██╔══╝   ██║╚██╔╝██║  ██╔═══╝    ██╔════╝ ██║  ██║      ██╔════╝  ")
	slog.Info("   ██║    ███████╗ ██║ ╚═╝ ██║  ██║        ██║      ██║  ████████ ████████╗ ")
	slog.Info("   ╚═╝    ╚══════╝ ╚═╝     ╚═╝  ╚═╝        ╚═╝      ╚═╝  ╚══════╝ ╚═══════╝ ")
	slog.Info("")
	slog.Info("      ██████╗ ███████╗ ██████╗ ██╗███████╗████████╗██████╗ ██╗   ██╗  ")
	slog.Info("      ██╔══██╗██╔════╝██╔════╝ ██║██╔════╝╚══██╔══╝██╔══██╗╚██╗ ██╔╝  ")
	slog.Info("      ██████╔╝█████╗  ██║  ███╗██║███████╗   ██║   ██████╔╝ ╚████╔╝   ")
	slog.Info("      ██╔══██╗██╔══╝  ██║   ██║██║╚════██║   ██║   ██╔══██╗  ╚██╔╝    ")
	slog.Info("      ██║  ██║███████╗╚██████╔╝██║███████║   ██║   ██║  ██║   ██║     ")
	slog.Info("      ╚═╝  ╚═╝╚══════╝ ╚═════╝ ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝   ╚═╝     ")
}
