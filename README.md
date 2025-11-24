# temp-file-registry

A lightweight temporary in-memory file registry exposed via a simple HTTP API.  
Upload files tied to a key and download them for a limited time. Files are stored in memory and automatically removed when expired.

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

This project stores uploaded files in memory and exposes no authentication or rate limiting. Do NOT expose it publicly or use it for sensitive or production workloads without adding appropriate security and resource controls.

## Quick facts (from main.go)

- API base path: `/temp-file-registry/api/v1`
- Default port: `8888`
- Default file expiration: `10` minutes
- Default max file size: `1024` MB
- Log level values: `-4:Debug`, `0:Info`, `4:Warn`, `8:Error`
- Files stored in memory (map[string]FileRegistry), protected by a mutex and cleaned every minute

## Installation

Requirements:
- Go (recommended recent stable release)

Build:

```bash
git clone https://github.com/xshoji/temp-file-registry.git
cd temp-file-registry
go build -ldflags="-s -w" -trimpath -o temp-file-registry main.go
```

## Usage / Quick start

Start server with defaults (listens on port 8888):

```bash
./temp-file-registry
```

Upload a file:

```bash
curl -X POST \
  -F "file=@/path/to/localfile.txt" \
  -F "key=example-key" \
  -F "expiryTimeMinutes=15" \
  http://localhost:8888/temp-file-registry/api/v1/upload
```

Download a file:

```bash
curl -O "http://localhost:8888/temp-file-registry/api/v1/download?key=example-key"
# Delete after download:
curl -O "http://localhost:8888/temp-file-registry/api/v1/download?key=example-key&delete=true"
```

## Command-line options

```bash
$ temp-file-registry -h
Usage: temp-file-registry [OPTIONS] [-h, --help]

Description:
  temp-file-registry is temporary file registry provided through an HTTP web API.

Options:
  -e, --expiration-minutes int    Default file expiration (minutes) (default 10)
  -l, --log-level int             Log level (-4:Debug, 0:Info, 4:Warn, 8:Error)
  -m, --max-file-size-mb int64    Max file size (MB) (default 1024)
  -p, --port int                  Port (default 8888)

```

(Help output is produced via Go's flag package; `-h` / `--help` supported.)

## API

Base URL: http://<host>:<port>/temp-file-registry/api/v1

### POST /upload

Register an uploaded file in memory.

- URL: `/temp-file-registry/api/v1/upload`
- Method: `POST`
- Form fields:
  - `file` (file) — file to upload (multipart/form-data)
  - `key` (string) — key to register the file under (required)
  - `expiryTimeMinutes` (int, optional) — override server default for this file
- Success: HTTP 200 with a JSON message including file registry info
- Errors:
  - 400 — multipart parse error or file too large
  - 405 — method not allowed

Notes:
- Uploads are limited by server `max-file-size-mb` (converted to bytes).
- The uploaded multipart file is kept in memory (multipart.File and header stored in a map).

### GET /download

Retrieve an uploaded file by key.

- URL: `/temp-file-registry/api/v1/download`
- Method: `GET`
- Query parameters:
  - `key` (string) — required
  - `delete` (string) — optional; if `true`, the file is removed after a successful download
- Success: HTTP 200 with file bytes; appropriate `Content-Type` and `Content-Disposition` headers set
- Errors:
  - 404 — file not found (key missing or expired)
  - 405 — method not allowed

Behavior detail:
- After responding, the server rewinds the in-memory file so it can be downloaded again (unless deleted).
- Expired files are removed by an internal goroutine that runs once per minute and deletes any registry entries with expiredAt < now.

## How it works (simplified)

1. Client POSTs a multipart form to /upload with `file` and `key`.
2. Server stores the multipart.File and header in an in-memory map keyed by `key`, along with an expiration timestamp.
3. A background goroutine checks the map every minute and removes expired entries.
4. Client GETs /download?key=... to retrieve the file. Optionally supply `delete=true` to remove after download.

## Limitations & security considerations

- In-memory storage: large/many uploads will increase memory usage and may exhaust system memory.
- No authentication/authorization.
- No rate limiting, virus scanning, or upload validation beyond size limit.
- Headers and filenames are used as provided by the client.
- Not suitable for public production use without additional safeguards.

## Contributing

Contributions welcome. Possible improvements:
- Add persistent storage backend (filesystem, S3, DB)
- Authentication and access controls
- Rate limiting and request validation
- Tests and CI

## Requirements

- Go 1.16+ recommended

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
