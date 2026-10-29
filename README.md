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
