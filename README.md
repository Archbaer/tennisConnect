# Tennis Connect

## Local backend

Requirements: Go 1.25, Docker, Docker Compose, and OpenSSL.

Start PostgreSQL/PostGIS:

```sh
docker compose up -d database
```

For a database created before the auth migration, apply it once after recreating the container with the updated migration mount:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000002_auth.sql
```

For a database upgrading from the passwordless-auth phase, apply the profile migration once:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000003_profiles.sql
```

For a database upgrading from the profile phase, apply the photos migration once:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000004_photos.sql
```

Create local configuration once. Keep the generated keys stable: replacing them makes existing account lookups and encrypted data unreadable.

```sh
umask 077
{
  printf "%s\n" "DATABASE_URL='postgres://tennisconnect:local-development-only@localhost:5432/tennisconnect?sslmode=disable'"
  printf "%s\n" "APP_BASE_URL='http://localhost:8080'"
  printf "%s\n" "EMAIL_FROM='Tennis Connect <auth@example.com>'"
  printf "%s\n" "RESEND_API_KEY='replace-with-real-resend-key'"
  printf "EMAIL_ENCRYPTION_KEY='%s'\n" "$(openssl rand -base64 32)"
  printf "EMAIL_HMAC_KEY='%s'\n" "$(openssl rand -base64 32)"
  printf "%s\n" "COOKIE_SECURE='false'" "TRUST_PROXY_HEADERS='false'"
} > .env
```

Set a real Resend key in `.env`, then load configuration and run:

```sh
set -a
source .env
set +a
go run .
```

Private photo storage is optional locally. Set `S3_BUCKET` and `AWS_REGION` (for example `eu-west-2`) plus credentials from the standard AWS chain to enable uploads. When no credentials exist, set `AWS_EC2_METADATA_DISABLED=true` so the AWS SDK does not wait on instance metadata. Without `S3_BUCKET` the server starts normally and the photo endpoints return 503; `AWS_REGION` is required whenever `S3_BUCKET` is set. The bucket must block all public access, enforce TLS-only access, and allow browser `PUT` from the app origin with the `content-type` header. Never apply a public ACL.

Local credentials and insecure cookies are for the local Docker database and HTTP development only. Production requires TLS, secure cookies, secret-managed keys, and a production Resend sender. Trust proxy headers only behind a trusted proxy that appends the real client address.

Email normalization lowercases the domain only. The local part keeps its case because SMTP providers may treat local parts as case-sensitive.

Check database-backed health:

```sh
curl --fail http://localhost:8080/healthz
```

## Authentication API

1. `POST /api/v1/auth/link` with `{"email":"player@example.com"}` sends a sign-in link. Response is `{"message":"check your email"}` for both known and new addresses. Invalid input returns 400, rate limits return 429 with `Retry-After`, and provider/database failures return 503.
2. `GET /auth/verify?token=...` displays a confirmation form without consuming the token.
3. `POST /api/v1/auth/verify` with form field `token` signs in an existing account. New addresses receive a date-of-birth form without consuming the token.
4. `POST /api/v1/auth/register` with form fields `token` and `birth_date=YYYY-MM-DD` creates an adult account. Underage or malformed dates return 400; invalid, expired, or replayed tokens return 401.
5. `GET /api/v1/auth/session` returns 204 for a valid session and 401 otherwise.
6. `POST /api/v1/auth/sign-out` revokes the server session and clears the cookie. A database failure returns 503 and retains the cookie for retry.

Successful sign-in responses contain `{"message":"signed in"}`. Errors use `{"error":"..."}`. The `tennisconnect_session` cookie lasts 14 days and uses `HttpOnly` and `SameSite=Lax`; `Secure` is enabled by default outside local HTTP development. Only SHA-256 session-token hashes are stored server-side.

## Profile API

`GET /api/v1/profile` returns the authenticated player's profile. `PUT /api/v1/profile` creates or replaces it with JSON:

```json
{
  "display_name": "Alex",
  "gender": "woman",
  "skill_level": "intermediate",
  "ranking": "10.5",
  "bio": "Tuesday evening rallies.",
  "gender_preference": "all",
  "min_age": 25,
  "max_age": 45,
  "skill_tolerance": 1,
  "availability": [
    {"weekday": 2, "start": "18:00", "end": "21:00"}
  ]
}
```

Weekdays use ISO numbering: Monday 1 through Sunday 7. Times are `Europe/London` local times in 24-hour `HH:MM` format and cannot cross midnight. One to 20 unique slots are required. Skill tolerance 0 means exact skill only; 1 allows adjacent levels. Gender is `woman` or `man`; gender preference is `women`, `men`, or `all`. Ranking is optional. Display names and bios cannot contain contact details. Numeric phone/date-shaped candidates with eight or more digits are rejected to avoid ambiguous contact information; write dates with month names instead, such as `29 October 2026`.

## Photos API

`POST /api/v1/photos/upload-url` creates one pending photo slot and returns `201 {"photo_id":"...","upload_url":"...","expires_in_seconds":300}`. The presigned URL is a bearer credential valid for five minutes for exactly that object key: never log, cache, or share it. A server-side session and same-origin `Origin` are required, and an owner can hold at most three photos; a fourth upload request while three slots are held returns 409. The API never receives the image bytes.

`POST /api/v1/photos/{id}/process` re-validates the uploaded bytes server-side (JPEG/PNG/WebP, 5 MB, 20 MP), strips EXIF, resizes the longest side to at most 1600 px, stores the sanitized JPEG under a new key, deletes the original, and moderates the result (see below). It returns `200 {"photo_id":"...","status":"approved"|"rejected"|"pending"}`. Missing, oversized, malformed, foreign, or already-moderated photos return 400, 404, or 409 as appropriate. Replaying the upload URL cannot overwrite the sanitized object. Pending and rejected photos are never public: this phase has no public photo route, and an abandoned upload slot older than one hour is reclaimed on the next upload attempt once all three slots are full.

SigV4 presigned PUTs cannot bound the body size, so oversized or non-image uploads are rejected and deleted during processing. Browser uploads need a bucket CORS rule allowing `PUT` with the `content-type` header from the app origin; an S3 lifecycle rule on the `photos/` prefix can remove objects orphaned by abandoned uploads.

### Moderation

`/process` checks the sanitized image synchronously before answering. A clean photo with one visible, unobstructed face returns `200 {"photo_id":"...","status":"approved"}`. A rejected photo returns `200 {"photo_id":"...","status":"rejected"}`, keeps its row, deletes its object, and exposes no image URL or stated reason; the owner's next upload reuses the freed slot. An inconclusive score returns `200 {"status":"pending"}`: not approved, not visible to anyone, and not retried automatically. A provider timeout, transport error, malformed response, or database failure returns 503 and leaves the photo pending for a manual retry.

Moderation needs `SIGHTENGINE_API_USER` and `SIGHTENGINE_API_SECRET`; set both or neither. With neither, uploads still work but `/process` returns 503 and pending rows are reclaimed after an hour. Each process call sends the EXIF-free, at-most-1600-px JPEG to Sightengine, which the provider bills as up to four operations; only a fully decoded success containing every required field can change a photo's state.

Reject at 0.5 and approve below 0.4 on the safety keys, require a face box of at least 1 % with `obstruction.none` at least 0.5, and reject OCR text that trips the fail-closed contact-detail rule (including 8+ digit numbers such as scoreboards). These thresholds are product policy, not vendor guidance.

`GET /api/v1/photos` lists the owner's photos as `{"photos":[{"photo_id","display_order","status"}]}` in display order, never with URLs. `GET /api/v1/photos/{id}/url` returns a five-minute, owner-only URL for an approved or pending photo; rejected, foreign, and unknown ids all return the same 404. Neither response may be cached. Other players' photos are not served in this phase.

Create the isolated test database once:

```sh
docker compose exec -T database psql -U tennisconnect -d tennisconnect -c 'CREATE DATABASE tennisconnect_test;'
```

Run all checks, including real database authentication tests:

```sh
test -z "$(gofmt -l .)"
go mod verify
go vet ./...
TEST_DATABASE_URL='postgres://tennisconnect:local-development-only@localhost:5432/tennisconnect_test?sslmode=disable' go test -race ./...
govulncheck ./...
```

Tests reset only the database named `tennisconnect_test`; setup refuses other database names. `PORT` defaults to `8080`. The server refuses invalid configuration or an unavailable database at startup.
