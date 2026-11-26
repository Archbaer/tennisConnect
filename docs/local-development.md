# Local development

Contributor setup, migrations and local provider configuration.

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

Requirements: Go 1.26, Docker, Docker Compose, and OpenSSL.

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

For a database upgrading from the locations phase, apply the matches migration once:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000006_matches.sql
```

For a database upgrading from the matches phase, apply the safety migration once:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000007_safety.sql
```
For a database upgrading from the safety phase, apply the match reveals migration once:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000008_match_reveals.sql
```

For a database upgrading from the match reveals phase, apply the recovery migration once:

```sh
docker compose up -d --force-recreate database
docker compose exec -T database psql --single-transaction --set=ON_ERROR_STOP=1 -U tennisconnect -d tennisconnect -f /docker-entrypoint-initdb.d/000009_recovery_sessions.sql
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

## Frontend

The SvelteKit app lives in `web/`. It is a static build served by the Go API so cookies stay same-origin.

```sh
cd web
npm ci
npm run dev        # http://localhost:5173, proxies /api, /auth and /reviewer/verify to :8080
npm run check
npm run test
npm run build      # writes web/build
cd ..
WEB_DIR=web/build go run ./cmd/api
```

With `WEB_DIR` set, Go serves the built site at `/` and keeps `/api/`, `/auth/`, `/reviewer/verify` and `/healthz` untouched. Unknown API paths return 404, not the SPA. A missing `index.html` or an invalid directory fails startup. The app never stores tokens or other players' private data in browser storage.
