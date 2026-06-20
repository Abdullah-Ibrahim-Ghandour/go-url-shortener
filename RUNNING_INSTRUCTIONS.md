# Running Instructions

This document explains how to set up, run, test, and manually verify the URL shortener.

## Prerequisites

- Go 1.26 or newer
- `curl` for the manual HTTP examples

Check your Go version:

```sh
go version
```

## Setup

Clone the repository and enter the project directory:

```sh
git clone https://github.com/Abdullah-Ibrahim-Ghandour/go-url-shortener.git
cd go-url-shortener
```

Download dependencies:

```sh
go mod download
```

## Run The Server

Start the service:

```sh
go run ./cmd/shortlink
```

Expected startup log:

```text
starting shortlink server addr=:8080 base_url=http://localhost:8080 db_path=data/shortlink.db
```

The default configuration is:

```text
SHORTLINK_ADDR=:8080
SHORTLINK_BASE_URL=http://localhost:8080
SHORTLINK_DB_PATH=data/shortlink.db
```

To override the defaults:

```sh
SHORTLINK_ADDR=:9090 \
SHORTLINK_BASE_URL=http://localhost:9090 \
SHORTLINK_DB_PATH=data/local.db \
go run ./cmd/shortlink
```

On Windows PowerShell:

```powershell
$env:SHORTLINK_ADDR = ":9090"
$env:SHORTLINK_BASE_URL = "http://localhost:9090"
$env:SHORTLINK_DB_PATH = "data/local.db"
go run ./cmd/shortlink
```

## Encode A URL

In another terminal, run:

```sh
curl -s http://localhost:8080/encode \
  -H "Content-Type: application/json" \
  -d '{"url":"https://codesubmit.io/library/react"}'
```

Example response:

```json
{
  "short_url": "http://localhost:8080/Ab3dE9xY"
}
```

The code is random, so your `short_url` value will be different.

## Decode A URL

Use the `short_url` returned by `/encode`:

```sh
curl -s http://localhost:8080/decode \
  -H "Content-Type: application/json" \
  -d '{"short_url":"http://localhost:8080/Ab3dE9xY"}'
```

Example response:

```json
{
  "url": "https://codesubmit.io/library/react"
}
```

## Follow A Short URL

Open the returned `short_url` in a browser, or inspect the redirect with:

```sh
curl -I http://localhost:8080/Ab3dE9xY
```

Example response headers:

```http
HTTP/1.1 302 Found
Location: https://codesubmit.io/library/react
```

## Verify Persistence

1. Start the server:

   ```sh
   go run ./cmd/shortlink
   ```

2. Encode a URL with `/encode` and save the returned `short_url`.

3. Stop the server with `Ctrl+C`.

4. Start the server again with the same `SHORTLINK_DB_PATH`.

5. Decode the saved `short_url` with `/decode`, or open it in a browser.

The decode response should still return the original URL, and the short URL should still redirect, because mappings are stored in SQLite.

## Run Tests

Run the full test suite:

```sh
go test ./...
```

Build all packages:

```sh
go build ./...
```

The tests include:

- service unit tests
- HTTP handler tests for `/encode`, `/decode`, and `GET /{code}`
- SQLite storage integration tests
- endpoint integration coverage through `httptest.Server`

## Clean Local Runtime Data

The service creates local SQLite files under `data/` by default. To reset local data:

```sh
rm -rf data
```

On Windows PowerShell:

```powershell
Remove-Item -Recurse -Force data
```
