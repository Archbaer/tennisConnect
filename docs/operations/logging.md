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

### Auth, safety and match events

| Event | Fields | Notes |
|---|---|---|
| `auth.link_requested` | `event` | One per issued magic link. No address. |
| `auth.login_succeeded` | `event`, `actor_ref` | Magic-link verification or registration completed. |
| `auth.login_failed` | `event`, `reason` | `reason` is an enum: `invalid_request`, `invalid_or_expired_token`, `rate_limited`, `banned`, `recovery_required`, `underage`. Never the address or token. |
| `auth.logout` | `event` | One per sign-out. No session value. |
| `safety.report_submitted` | `event`, `actor_ref`, `target_ref`, `category` | Category only; report details are never logged. |
| `safety.block_created` | `event`, `actor_ref`, `target_ref` | Emitted after the block commits. |
| `safety.report_dismissed` | `event`, `actor_ref`, `target_ref`, `category` | Reviewer decision. |
| `safety.reviewer_banned` | `event`, `actor_ref`, `target_ref`, `category` | Reviewer decision; the target is banned. |

`actor_ref` and `target_ref` are `observability.ActorRef` pseudonyms (first 16
hex characters of SHA-256 of the user UUID). Events carry no e-mail, token, IP,
cookie or report body, and each action emits one event without extra queries.

### Redaction

`observability.New` installs a `ReplaceAttr` redactor on both JSON and text
handlers. An attribute is replaced with `[redacted]` when its key, after
lower-casing and removing `-`, `_` and spaces, either:

- equals one of `password`, `passwd`, `secret`, `token`, `authorization`,
  `cookie`, `set-cookie`, `session`, `email`, `e_mail`, `address`, `phone`,
  `dob`, `birth`, `bio`, `details`, `instagram`, `url`, `upload_url`,
  `presigned`, `api_key`, `apikey`, `key`, `signature`, `latitude`,
  `longitude`, `lat`, `lng`, `coordinates`, `ip`; or
- contains `password`, `passwd`, `secret`, `token`, `authorization`, `cookie`,
  `session`, `email`, `address`, `phone`, `birth`, `details`, `instagram`,
  `presigned`, `apikey`, `signature`, `latitude`, `longitude`, `coordinates`
  or `uploadurl` (so `session_id`, `object_key` and `e_mail` are caught).

Values are also redacted under an innocent key when they look like an e-mail
address, a presigned/signed URL (for example `X-Amz-Signature=`), a bearer
credential, or a long base64url token (40+ characters containing digits and a
non-hex rune). Lowercase hex request IDs and digests pass through. Groups are
redacted attribute by attribute at every depth.

To add a key, append it to `sensitiveKeyExact` or `sensitiveKeyParts` in
`internal/observability/redact.go` and add a row to the table-driven
`TestBannedKeysAreRedacted` test in the same package. Never log the value you
are trying to protect first: redaction is a backstop, not permission.

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
