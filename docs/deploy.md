# Deployment notes

This repository has CI only. Nothing deploys automatically and no production
credentials are committed. This file records the intended production shape and
the checks that must pass before launch.

## Image

`Dockerfile` builds the static SvelteKit site, compiles `cmd/api` with the
pinned Go toolchain, and copies both into a non-root distroless image. Build it
locally with:

```sh
docker build -t tennis-connect .
```

The container needs a reachable PostgreSQL/PostGIS database at startup and the
environment variables documented in `docs/local-development.md`. `WEB_DIR` is already set to
`/app/web/build`; do not mount over it.

## Target shape

- Two App Runner instances in `eu-west-2`, behind TLS and a stable domain.
- Private RDS PostgreSQL with PostGIS, no public endpoint, 30-day point-in-time
  recovery, RPO 5 minutes and RTO 1 hour.
- Private S3 bucket: block all public access, TLS-only policy, no public ACL,
  CORS limited to the app origin and `PUT` plus `content-type`, and a lifecycle
  rule on `photos/` for orphaned objects.
- Secrets Manager for `DATABASE_URL`, Resend, Mapbox, Sightengine,
  `EMAIL_ENCRYPTION_KEY` and `EMAIL_HMAC_KEY`. The last two must stay stable:
  rotating them makes account lookup and reviewer TOTP secrets unreadable.
- Least-privilege IAM: the service role reads only its S3 prefix and secret
  names. No wildcard `*` resource.
- Logs are structured and redacted. Never log e-mail, tokens, exact coordinates,
  report details, TOTP codes or provider credentials.

## Headers

The API sets `Cache-Control: no-store` and `Referrer-Policy: no-referrer` on
private responses. The static handler sets `X-Content-Type-Options: nosniff`,
`Referrer-Policy: no-referrer`, a restrictive `Permissions-Policy`, and
`Content-Security-Policy: frame-ancestors 'none'; base-uri 'none'`.
SvelteKit adds a build-time hash-based CSP meta tag for scripts, styles, fonts,
images and connections; `style-src` includes `'unsafe-inline'` for Svelte style
attributes. Pin the S3 source to the exact production bucket origin. Add HSTS
only after the production domain is stable; a preload mistake is hard to undo.

## Migrations

Run every file in `migrations/` in filename order with
`psql -v ON_ERROR_STOP=1` and no `--single-transaction` (the files own their
transactions). Take a snapshot or verify PITR before a production migration.

## Local verification, 2026-11-26

- CSP enabled in hash mode and verified against both SvelteKit dev and the
  built static site served by Go: no browser CSP violations, `frame-ancestors`
  header present, `X-Request-ID` present.
- Structured JSON request logging verified with request IDs and no query string.
- Playwright now includes a mobile-Chromium project; desktop and mobile both
  pass.
- Short discovery load gate: 500 requests over 10 seconds, p95 25.7 ms, zero
  errors, 100,000-user seed; the one-hour soak and dense-city capacity remain
  open.
- Recovery flow is covered by Go tests; the recovery-only cookie and screen are
  in place.

## Launch gates still open

- The one-hour 50 RPS soak and dense-city discovery capacity were not measured
  on production-like hardware. The committed local gate is environment-gated.
- Restore and failover drills for 30-day PITR, RPO 5 minutes and RTO 1 hour have
  not been run.
- Accessibility and browser coverage is Chromium plus axe in CI; iOS Safari and
  Android Chrome still need a manual pass.
- CD is deliberately deferred. This repository does not deploy `main`.

## Local container smoke test

```sh
docker compose up -d database
docker build -t tennis-connect .
docker run --rm -p 8080:8080 --add-host=host.docker.internal:host-gateway \
  --env-file .env \
  -e DATABASE_URL='postgres://tennisconnect:local-development-only@host.docker.internal:5432/tennisconnect?sslmode=disable' \
  -e WEB_DIR=/app/web/build tennis-connect
curl --fail http://localhost:8080/healthz
```

The server refuses invalid configuration, an unavailable database, or a
`WEB_DIR` without `index.html`.
