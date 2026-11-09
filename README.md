# Tennis Connect

## Project layout

- `cmd/api` — server entrypoint (`go run ./cmd/api`).
- `internal/app` — route wiring, HTTP server, graceful stop.
- `internal/config` — environment configuration and validation.
- `internal/httpapi` — JSON, error, origin and cache helpers.
- `internal/auth` — magic links, sessions, rate limits, Resend sender.
- `internal/profile` — profile and availability.
- `internal/photos` — validation, upload, processing, moderation state.
- `internal/storage` — private S3 presigning and object access.
- `internal/moderation` — Sightengine client and verdict policy.
- `internal/location` — Mapbox geocoding, home area and travel destination.
- `internal/discovery` — eligibility, filters, whole-mile cards and signed photo URLs.
- `internal/textguard` — contact-detail detection shared by profile and moderation.
- `internal/saferead` — bounded reader shared by storage and moderation.
- `internal/testutil` — PostGIS test harness and fixtures.
- `migrations/` — operational SQL migrations; `docs/design/` — design artifacts.

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

For a database upgrading from the photos phase, apply the locations migration once:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000005_locations.sql
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
go run ./cmd/api
```

Mapbox geocoding is optional. Set `MAPBOX_ACCESS_TOKEN` to a Mapbox access token to enable area lookup. Without it the server starts normally and `PUT /api/v1/location` and `PUT /api/v1/location/travel` return 503; reads and discovery still serve previously stored areas. Area lookups send the typed area or the browser coordinates to Mapbox and request `permanent=true` results because the centroid and coarse label are stored. Selecting Mapbox as a subprocessor that receives exact coordinates at write time needs the vendor data-protection addendum and a card or enterprise contract for permanent geocoding before launch.

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

## Location API

All location endpoints require a session and return `Cache-Control: no-store` with `Referrer-Policy: no-referrer`. Coordinates are stored as PostGIS points and are never returned by any endpoint, error, cursor or log.

`PUT /api/v1/location` sets the home area and returns `200 {"area_label":"Battersea"}`. The body carries **exactly one** of `{"latitude":51.46,"longitude":-0.16}` (browser position, server-side reverse geocode) or `{"area":"SW11 4NJ" | "Battersea"}` (forward geocode). Requests are UK-only: a non-GB or unresolved area returns 422 and writes nothing; malformed bodies, ranges, or both input forms return 400. Provider timeouts, transport errors, redirects, malformed or oversized responses return 503 and never overwrite the stored area. The stored label comes from the result's own settlement name or its neighbourhood, locality or place context, is trimmed to 1–80 characters, and never contains postcode text; a neighbourhood or place name may legitimately match the typed area.

`GET /api/v1/location` returns `200 {"area_label":"Battersea"}` and, while a travel destination is live, `travel_area_label` plus `travel_expires_at` (RFC 3339). It returns 404 when the player has no area yet. Labels and dates only: no coordinates, no postcodes.

`PUT /api/v1/location/travel` accepts `{"area":"Edinburgh","days":7}` and returns `200 {"area_label","expires_at"}`. It requires an existing home area and otherwise returns `409 {"error":"set your area first"}`. `days` is 1–7 and defaults to 7; a second PUT replaces the destination. `DELETE /api/v1/location/travel` clears only the travel columns, keeps home, and returns 204 whether or not a destination was set (idempotent). A live travel destination wins for up to its expiry; after expiry the home area is effective again without any background sweep. Updating home never clears a live travel destination.

## Discovery API

`GET /api/v1/discovery?radius=10&limit=20&cursor=...` returns `200 {"cards":[{"player":{...}}],"next_cursor":null}`. Each card is `{display_name, age, gender, skill_level, bio, play_summary, photo_urls[], distance_miles}`. `age` is server-computed from the date of birth; `ranking` is deliberately absent from cards (Q3). `distance_miles` is an integer: `0` means under one mile, otherwise the value is rounded up to the next whole mile. The client maps `0` to "Under 1 mile" and `n` to "n miles".

- `radius` is 5–30 whole miles (default 10), converted server-side at 1609.344 metres per mile for `ST_DWithin`; the boundary is inclusive. `limit` is 20–50 (default 20).
- `cursor` is an opaque unsigned base64url `<whole miles>:<uuid>:<radius>` keyset token. Malformed tokens, invalid UUIDs, miles outside `0..radius`, or a radius that differs from the request return 400. A forged cursor only moves the page window; exact distances never leave the server, so a cursor cannot triangulate another player.
- Cards are ordered nearest mile bucket first, then by user id within the same bucket. This is a privacy-driven deviation from exact-distance ordering: two candidates in the same whole-mile bucket are ordered by UUID, not by their sub-mile distance.
- `play_summary` is at most two phrases built from the shared weekly availability: weekday abbreviation plus `morning` (start before 12:00), `afternoon` (before 17:00) or `evening`, de-duplicated and sorted by weekday then part, with a shared part joining all its weekdays ("Tue, Thu evenings", "Mon, Tue, Wed evenings"); at most two phrases are rendered and an empty string is returned when there are no slots. Slot detail is a Task 8 screen.
- `photo_urls` contains at most three five-minute signed URLs for the candidate's approved photos, in display order, all issued in one read-only repeatable-read snapshot. A single signing failure omits that URL and logs generically; it never fails the page. No coordinates, raw object-key fields, bucket fields or credentials beyond the signed URL itself appear in a response or log. Cards are never cached.

Discovery eligibility: the viewer needs an adult account, a completed profile, an area and at least one approved photo (recent sign-in is not required). Candidates need an adult account, a profile, an area, an approved photo, no deletion, and activity in the last seven days (strictly after `now() - interval '7 days'`). Both players' gender preferences must include each other, both age ranges must contain the other's age, skill distance must fit `GREATEST` of both tolerances (beginner < improver < intermediate < advanced), and availability must overlap on the same Europe/London weekday. Distinct 409 bodies are `"complete your profile"`, `"set your area"` and `"one photo must be approved"`. Self is excluded.

Discovery excludes, server-side and in the same repeatable-read query: the viewer's own row, a block in either direction, an existing match, and any target the viewer already reacted to (`like` or `pass`). Those predicates arrived with Task 6 (`migrations/000006_matches.sql`), so **discovery is now safe to expose to a client**; Task 8 wires the screens.

`POST /api/v1/reactions` `{target_id, action: "like"|"pass"}` returns `{"matched": bool, "match_id"?: uuid}`. A `pass` is permanent; repeating any reaction for the same target returns 409 and never overwrites the first verdict. Reciprocal likes create exactly one row in `matches` for the canonical `(user_low, user_high)` pair, so concurrent reciprocal likes cannot duplicate it. Reacting across a block in either direction returns 403.

`GET /api/v1/matches` returns `{"matches":[{match_id, display_name, matched_at, has_instagram, photo_urls[]}]}`, newest first. `GET /api/v1/matches/{id}` returns the same row for one match. Both are member-only (404 otherwise) and never carry the handle; `photo_urls[]` holds at most three five-minute signed URLs for the other player's approved photos only. A matched pair with a soft-deleted account is hidden entirely (404). Signed URLs issued before a block keep working until they expire — blocking stops *new* issuance immediately, it does not revoke URLs already handed out. `GET /api/v1/matches/{id}/instagram` is the only disclosure path and returns `{"instagram_handle": "..."}` — empty when the other player has none.

`POST /api/v1/blocks` `{target_id}` is idempotent and returns 204. Blocking hides the pair in both directions in discovery, closes the match for both members (the match list and detail 404), and revokes handle and photo access immediately.

### Performance and load gate

The committed load test is environment-gated:

```sh
DISCOVERY_LOAD=1 DISCOVERY_LOAD_SECONDS=60 TEST_DATABASE_URL='postgres://tennisconnect:local-development-only@localhost:5432/tennisconnect_test?sslmode=disable' \
  go test ./internal/discovery -run TestDiscoveryLoadGate -v
```

It seeds 100,000 representative profiles (2% in a London cluster), prints `EXPLAIN (ANALYZE, BUFFERS)`, asserts both `locations` GiST indexes are used, then soaks at 50 RPS for the configured window. Recorded evidence on the local emulated-AMD64 PostGIS container: 3,000 requests, p95 ≈ 17.5 ms, 0 errors.

This does **not** prove the one-hour 50 RPS soak from the plan, production App Runner/RDS latency, or multi-instance behaviour. A dense-city seed (about one in seven rows inside 10 miles) measured 137–450 ms per query and roughly 1.9 s p95 at 50 RPS on the same emulated container, so dense-city capacity is not proven; re-measure on production-like hardware before launch.

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
