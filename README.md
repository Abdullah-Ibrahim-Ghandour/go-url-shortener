# Go URL Shortener

A small URL shortening service written in Go. It exposes JSON endpoints to encode an original URL into a short URL and decode a short URL back to the original URL.

The service uses SQLite for local persistence, so links remain available after the process restarts.

## Live Demo

https://abdullah-shortlink.up.railway.app

Try encoding a URL:

```sh
curl -s https://abdullah-shortlink.up.railway.app/encode \
  -H "Content-Type: application/json" \
  -d '{"url":"https://codesubmit.io/library/react"}'
```

Then decode the returned `short_url`:

```sh
curl -s https://abdullah-shortlink.up.railway.app/decode \
  -H "Content-Type: application/json" \
  -d '{"short_url":"https://abdullah-shortlink.up.railway.app/<code>"}'
```

## Running Instructions

See [RUNNING_INSTRUCTIONS.md](RUNNING_INSTRUCTIONS.md) for setup, run, test, manual verification, and cleanup steps.

## API

All responses use JSON with `Content-Type: application/json; charset=utf-8`.

### Encode

```http
POST /encode
Content-Type: application/json
```

Request:

```json
{
  "url": "https://codesubmit.io/library/react"
}
```

Success response:

```json
{
  "short_url": "http://localhost:8080/Ab3dE9xY"
}
```

Example:

```sh
curl -s http://localhost:8080/encode \
  -H "Content-Type: application/json" \
  -d '{"url":"https://codesubmit.io/library/react"}'
```

### Decode

```http
POST /decode
Content-Type: application/json
```

Request:

```json
{
  "short_url": "http://localhost:8080/Ab3dE9xY"
}
```

Success response:

```json
{
  "url": "https://codesubmit.io/library/react"
}
```

Example:

```sh
curl -s http://localhost:8080/decode \
  -H "Content-Type: application/json" \
  -d '{"short_url":"http://localhost:8080/Ab3dE9xY"}'
```

### Errors

```json
{
  "error": "invalid_url"
}
```

| Status | Error | Meaning |
| --- | --- | --- |
| 400 | `invalid_request` | Malformed JSON, missing field, unknown field, or trailing JSON token. |
| 400 | `invalid_url` | Original URL is empty, too long, malformed, or not `http`/`https`. |
| 400 | `invalid_short_url` | Short URL does not match this service or has an invalid code. |
| 404 | `not_found` | Short URL is valid but the code is unknown. |
| 405 | `method_not_allowed` | Endpoint was called with a non-POST method. |
| 500 | `internal_error` | Unexpected server-side failure. |

## Persistence

SQLite stores mappings in a `links` table:

```sql
CREATE TABLE IF NOT EXISTS links (
  code TEXT PRIMARY KEY,
  original_url TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL
);
```

Encoding is idempotent: encoding the same original URL again returns the existing short URL. Short codes are random 8-character Base62 strings generated with `crypto/rand`. Insertions use database uniqueness constraints so code collisions do not corrupt existing mappings.

## Security Considerations

- Original URLs are validated and must be absolute `http` or `https` URLs with a host.
- Request bodies are capped at `16 KiB`.
- Short codes are random Base62 values, but public short URLs can still be guessed or brute-forced over time.
- Rate limiting is not implemented. A public deployment should add rate limits for encode and decode endpoints.
- URL shorteners can be abused for spam, malware, or phishing. Production systems should add abuse reporting, scanning, and administrative controls.
- Submitted URLs may contain sensitive information. The server intentionally does not log submitted or decoded URLs.
- The service does not fetch submitted URLs. If future preview or metadata-fetching behavior is added, SSRF protections will be required.

## Scalability Notes

- SQLite is suitable for small single-instance deployments, but it is not the right shared database for many horizontally scaled writers.
- Multiple service instances would need shared storage such as PostgreSQL, MySQL, or another centralized datastore.
- The database unique constraints are the source of truth for collision safety.
- The 8-character Base62 code space contains `62^8` possible codes. Larger deployments can increase code length as usage grows.
- Hot decode paths can be cached because mappings are immutable after creation.
- Public deployments should add rate limiting, abuse controls, request metrics, and operational alerts.
- Very large distributed systems may use centralized code allocation, partitioned ID generation, or pre-generated code pools.

## Future Improvements

- Add a `GET /{code}` redirect endpoint.
- Add authentication and administrative controls.
- Add rate limiting and abuse reporting.
- Add Docker or deployment configuration.
- Add production metrics, tracing, and alerting.
