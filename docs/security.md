# Security model

This file records the trust boundaries, authorization rules and accepted risks
that the safety and account work added. Keep it current when a new private field,
route or background action appears.

## Trust boundaries

- The Go API is the only authority for identity, eligibility, matching,
  moderation, bans, deletion and private-field disclosure.
- PostgreSQL/PostGIS is the source of truth. Exact coordinates, e-mail addresses,
  date of birth and photo object keys never leave the server.
- Resend receives a magic-link recipient address at send time. S3 receives the
  sanitized photo and issues short-lived presigned URLs. Sightengine receives the
  sanitized photo. Mapbox receives the typed or browser area at write time.
- Reviewers are a separate identity in `reviewers`. They do not hold player
  profiles and cannot call player routes with a reviewer cookie.
- The future frontend is untrusted for authorization. It may show or hide a
  control, but the API must enforce the same rule.

## Authentication and session rules

- Player sessions are SHA-256 token hashes; cookies are `HttpOnly`,
  `SameSite=Lax`, and `Secure` outside local HTTP.
- Reviewer sessions are hashed, `HttpOnly`, `SameSite=Strict`, eight hours, and
  are kept in a separate cookie. Review routes also require a TOTP-elevated
  session.
- TOTP secrets are AES-GCM encrypted with a key derived from
  `EMAIL_ENCRYPTION_KEY` under the `reviewer-totp-key-v1` label. Codes are
  30 seconds, six digits, ±1 step, constant-time compared, single-use, and
  limited to five attempts per minute.
- Banned players cannot use player routes. They keep a restricted session only
  for the 14-day appeal endpoints. Deleted players lose all sessions.
- Sign-out and bans revoke the server-side session hash; an HttpOnly cookie
  alone is never authentication.

## Authorization matrix

| Route family | Anonymous | Player | Banned player | Reviewer without MFA | Reviewer with MFA |
| --- | --- | --- | --- | --- | --- |
| Auth link/verify | allowed | allowed | allowed | n/a | n/a |
| Profile, photos, location, discovery, reactions, matches, blocks | 401 | own data | 403 | n/a | n/a |
| `POST /api/v1/reports` | 401 | allowed, rate-limited | 403 | n/a | n/a |
| `DELETE /api/v1/account`, `GET /api/v1/account/export` | 401 | own account | 403 | n/a | n/a |
| `GET/POST /api/v1/account/appeal` | 401 | 404, no suspension | allowed, one 14-day appeal | n/a | n/a |
| Reviewer auth, enroll, verify | link/verify allowed | n/a | n/a | session only | session only |
| Reviewer report queue/actions | 401 | 401 | 401 | 403 | allowed |

## Private fields

Never return or log: e-mail, date of birth, exact coordinates, postcodes,
Instagram handles outside a live match, raw object keys, provider credentials,
TOTP secrets or codes, report details to a target, reporter identity to a
reviewer, report counts, ban status of another player, or signed URLs after
their five-minute lifetime.

## Report and ban invariants

- One open report per reporter-target pair.
- A report is reviewed once; dismissal and ban write an append-only
  `report_events` row.
- A ban is permanent in V1, sets `users.banned_at`, revokes sessions, deletes
  live matches, and removes the account from discovery, match and Instagram
  reads.
- Same-e-mail registration is rejected for a banned row even after deletion
  cleanup.
- A ban and a simultaneous report/decision are serialised by the report row
  lock and the session delete.

## Deletion and export

- Deletion is immediate hiding plus a 30-day recovery boundary. Sign-in within
  the window clears `deleted_at`; after it the hourly cleanup removes the row.
- Cascades remove database rows. The documented S3 lifecycle rule removes
  orphaned objects; the API does not enumerate S3 objects.
- Export is owner-only, repeatable-read, and excludes other players' private
  data. It never contains e-mail, DOB, coordinates, object keys or signed URLs.
- Backups may retain deleted data for the provider's retention window. The
  documented launch requirement is a 30-day PITR policy.

## Frontend

- The SvelteKit app is static, same-origin with the API, and uses `credentials:
  include`. Bootstrap CSS and Bootstrap Icons are installed locally; no CDN,
  Bootstrap JS or jQuery is loaded. It never reads or writes tokens, other players' private fields, or
  session data in `localStorage`/`sessionStorage`.
- User content is rendered through Svelte text interpolation. There is no
  `{@html}` path for a profile, bio, report or message.
- The Go static handler refuses non-GET/HEAD requests, dotfiles, `..` segments
  and any path under `/api/`, `/auth/`, `/reviewer/` or `/healthz`. Unknown API
  paths stay 404 instead of returning `index.html`.
- Authorization remains server-side. A hidden button is not a permission check;
  every mutation is re-authorized by the API.
- Playwright covers mocked empty/loading states, offline handling, denied
  location permission and real-API authorization flows plus serious/critical
  axe checks. Mobile Safari and Android Chrome remain manual launch gates.

## Deployment

- The image is distroless, non-root, with no shell or Node runtime.
- Secrets come from Secrets Manager. `EMAIL_ENCRYPTION_KEY` and `EMAIL_HMAC_KEY`
  are stable; rotating them breaks account lookup and reviewer TOTP.
- S3 blocks public access and serves only presigned URLs. RDS is private with
  30-day PITR.
- No CD exists. `main` does not deploy.

## Accepted risks

- Signed URLs issued before a ban, block or deletion remain valid until their
  five-minute expiry. Revocation would require a proxy or much shorter TTL.
- TOTP is the V1 reviewer second factor. Passkeys resist phishing better and are
  deferred until a reviewer UI exists.
- The 72-hour review target is a due timestamp and queue sort, not an automatic
  enforcement worker. Overdue reports stay visible.
- Reviewer bans have no unban or appeal-decision endpoint in V1; appeals are
  stored for a later manual process.
- Production dense-city load and the one-hour 50 RPS soak remain unproven. The
  committed load gate is local and environment-gated.
- CD is deferred; `main` deploys nothing automatically.
- The static app uses SvelteKit build-time script hashes plus a Go
  `frame-ancestors 'none'` header. `style-src` keeps `'unsafe-inline'` because
  Svelte emits inline style attributes. The S3 image/connect source is a
  region-scoped wildcard host; pin it to the exact bucket origin when the
  production bucket name is fixed.
- Playwright covers Chromium only. iOS Safari and Android Chrome need a manual
  accessibility and upload pass.

## Final verification, 2026-11-24

- `go test -race ./...` against PostGIS: all packages pass.
- `go vet ./...`, `gofmt -l .`, `go mod verify`, `go mod tidy -diff`: clean.
- `govulncheck ./...`: no vulnerabilities (toolchain go1.26.11).
- `gitleaks detect --source .`: 99 commits, no leaks.
- `npm run check`: 0 errors, 0 warnings.
- `npm run test`: 4 tests pass.
- `npm run test:e2e`: 10 Chromium tests pass, including real-API authorization flows and serious/critical axe.
- `npm audit --omit=dev`: 0 vulnerabilities.
- `docker build` plus container `/` and `/healthz` smoke test: 200.
- Production load, iOS Safari, Android Chrome, PITR drills and public deployment
  are not covered by this run. See `docs/deploy.md`.

## Verification before merge

- `go test -race -count=2 -shuffle=on ./...` against the PostGIS test database.
- `go vet ./...`, `gofmt`, `go mod verify`, `go mod tidy -diff`.
- `govulncheck ./...` and `gitleaks detect --source .`.
- Route matrix above exercised by tests, including IDOR and cross-user reads.
- No new secret, raw e-mail, coordinate or report detail in logs.
- Structured logs use a fixed schema, server-generated request IDs, stdout only
  and pseudonymous actor references; see `docs/operations/logging.md`.
