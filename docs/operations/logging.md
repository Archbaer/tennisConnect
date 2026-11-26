# Logging

Application logs are structured JSON on stdout. The deployment platform owns
collection, retention and access; the application never writes log files.

## Schema

Every HTTP request emits one `http_request` event:

```json
{
  "time": "2026-11-26T12:00:00.000Z",
  "level": "INFO",
  "msg": "http_request",
  "event": "http_request",
  "request_id": "16-byte-hex",
  "method": "GET",
  "route": "GET /api/v1/discovery",
  "status": 200,
  "duration_ms": 12,
  "bytes_written": 1234,
  "actor_ref": "16-hex-pseudonym"
}
```

- `route` is the Go route pattern. Unmatched paths log `unmatched`; raw paths and
  query strings are never logged.
- `request_id` is generated server-side and returned as `X-Request-ID`.
- `actor_ref` is the first 16 hex characters of SHA-256 of the user UUID. It is
  absent for anonymous requests.
- Panics log `event=panic` with the request ID and stack, and return a generic 500.

Other events use named values: `auth.*`, `reviewer.*`, `safety.*`, `account.*`,
`provider.error` and `cleanup.failed`.

## Never logged

E-mail, magic-link tokens, session or recovery cookies, Authorization headers,
client IP addresses, exact coordinates, postcodes, Instagram handles, presigned
URLs, object keys, bucket names, provider payloads, report details, dates of
birth or image bytes.

Provider failures log only provider, operation and a fixed error class.

## Configuration

- `LOG_LEVEL=debug|info|warn|error`, default `info`.
- `LOG_FORMAT=json|text`, default `json`.
- Local development can use `LOG_FORMAT=text`; production uses JSON.

## Sink and retention

Target deployment is AWS App Runner, which ships stdout to CloudWatch Logs.

- `/tennisconnect/app`: 30 days, request/provider/cleanup events, restricted read.
- `/tennisconnect/audit`: 365 days for auth/safety/account events, tighter IAM.

Suggested alarms:

- 5xx rate above 1% over 5 minutes
- any panic
- provider errors above 5 in 5 minutes
- auth 429 spike
- cleanup failure
- no logs from an instance for 10 minutes

Frontend telemetry and third-party analytics are out of scope for V1.
