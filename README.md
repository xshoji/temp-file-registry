## temp-file-registry

temp-file-registry is temporary file registry written by golang.

```
$ go run main.go -h
Usage: /var/folders/_q/dpw924t12bj25568xfxcd2wm0000gn/T/go-build106028678/b001/exe/main [OPTIONS] [-h, --help]

Description:
  temp-file-registry is temporary file registry provided through an HTTP web API.

Options:
  -e, --expiration-minutes int    Default file expiration (minutes) (default 10)
  -l, --log-level int             Log level (0:Panic, 1:Info, 2:Debug) (default 2)
  -m, --max-file-size-mb int64    Max file size (MB) (default 1024)
  -p, --port int                  Port (default 8888)

# execute
go run main.go


# build
APP=/tmp/app; go build -ldflags="-s -w" -o ${APP} main.go; chmod +x ${APP}
# APP=/tmp/tfr; GOOS=linux GOARCH=amd64   go build -ldflags="-s -w" -o ${APP} main.go; chmod +x ${APP} # linux
# APP=/tmp/tfr; GOOS=darwin GOARCH=amd64  go build -ldflags="-s -w" -o ${APP} main.go; chmod +x ${APP} # macOS
# APP=/tmp/tfr; GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ${APP} main.go; chmod +x ${APP} # windows


# start
/tmp/app
```

## API

### Upload

```
curl --location --request POST 'http://localhost:8888/temp-file-registry/api/v1/upload' \
--form 'key="kioveyzrrt287opddhk9"' \
--form 'file=@"/private/tmp/app"'
{"message":"key:kioveyzrrt287opddhk9, expiryTimeMinutes:10, fileHeader:map[Content-Disposition:[form-data; name="file"; filename="app"] Content-Type:[application/octet-stream]]"}
```

### Download

```
# delete: if "true" specified, target file will be deleted after response.
curl "http://localhost:8888/temp-file-registry/api/v1/download?key=kioveyzrrt287opddhk9&delete=true" -o /tmp/app2
```

## Release

Release flow of this repository is integrated with github action.
Git tag pushing triggers release job.

```
# Release
git tag v0.0.2 && git push --tags



# Delete tag
echo "v0.0.1" |xargs -I{} bash -c "git tag -d {} && git push origin :{}"

# Delete tag and recreate new tag and push
echo "v0.0.2" |xargs -I{} bash -c "git tag -d {} && git push origin :{}; git tag {} -m \"Release beta version.\"; git push --tags"

```
