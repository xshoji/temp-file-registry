# temp-file-registry

A lightweight temporary in-memory file registry exposed via a simple HTTP API. Upload files tied to a key and download them for a limited time. Files are stored in memory and automatically removed when expired.

```
+--------+     +------------------------+
| Client | --> |   temp-file-registry   |
|        | <-- |   (in-memory storage)  |
+--------+     +------------------------+
                 | Store files by key |
                 | Auto-clean expired |
                 +--------------------+
```

## Features

- 🗂️ Upload files tied to a user-specified key
- ⏱️ Per-file expiration (minutes), with a default expiration
- 📏 Server-wide max file-size limit (configurable)
- 🧠 In-memory storage and automatic cleanup of expired files (cleaner runs every minute)
- 🗑️ Optionally delete a file after download via query parameter
- ⚡ Minimal implementation using Go standard library

## Important warning

This is for development/testing only. Do not use in production without security measures.

## Installation

```bash
git clone https://github.com/xshoji/temp-file-registry.git
cd temp-file-registry
go build -o temp-file-registry main.go
```

## Usage

Start server:

```bash
./temp-file-registry -p 8888
```

Upload:

```bash
curl -X POST -F "file=@file.txt" -F "key=mykey" http://localhost:8888/temp-file-registry/api/v1/upload
```

Download:

```bash
curl -O "http://localhost:8888/temp-file-registry/api/v1/download?key=mykey"
```

## Options

```plaintext
Usage: temp-file-registry [OPTIONS]

Description:
  temp-file-registry is temporary file registry provided through an HTTP web API.

Options:
  -e, --expiration-minutes <int>     Default file expiration (minutes) (default 10)
  -l, --log-level <int>              Log level { -4:Debug | 0:Info | 4:Warn | 8:Error } (default 0)
  -m, --max-file-size-mb <int64>     Max file size (MB) (default 1024)
  -p, --port <int>                   Port (default 8888)
 ```

## API

- POST /upload: Upload file with key
- GET /download: Download file by key (optional ?delete=true)

## Contributing

PRs welcome.

## License

Add a LICENSE file or specify a license in the repository.

## Development

```
# execute
go run main.go

# build
go build -ldflags="-s -w" -trimpath -o /tmp/$(basename "$PWD") main.go

# start
/tmp/temp-file-registry
```

## Release

Pushing a version tag runs the `goreleaser` workflow to build and publish the
GitHub release.

```bash
# Release
v=v0.0.7; git tag "${v}" && git push origin "refs/tags/${v}"

# Move the tag to HEAD and re-run the release
v=v0.0.7; git tag -f "${v}" && git push -f origin "refs/tags/${v}"
```

Deleting a tag turns its release into a draft. Move the tag instead to keep the
release and its existing notes while replacing its assets. Checksums change;
fix an already published version with a new version instead.
