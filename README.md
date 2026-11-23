# Tennis Connect

Adult-only UK tennis matchmaking. Safety-first Go API, PostGIS discovery, private photos, static SvelteKit client.

## Stack

```mermaid
flowchart LR
  B[Player browser] --> W[SvelteKit static]
  W -->|same-origin cookies| A[Go 1.26 API]
  A --> PG[(PostgreSQL 18 + PostGIS)]
  A --> S3[Private S3 photos]
  A --> RS[Resend magic links]
  A --> SE[Sightengine moderation]
  A --> MB[Mapbox area lookup]
  CI[GitHub Actions: 8 required checks] --> IMG[Distroless image]
  IMG --> AR[App Runner x2 planned]
  PG --> RDS[(Private RDS planned)]
```

## Core flows

### Photo gate

```mermaid
flowchart LR
  U[Upload URL] --> P[Browser PUT]
  P --> V[Validate · strip EXIF · resize]
  V --> M{Sightengine}
  M -->|clean| A[Approved]
  M -->|unsafe| R[Rejected]
  M -->|uncertain| Q[Pending]
  A --> D[Discovery eligible]
  R --> X[Object deleted]
  Q --> X2[Not visible · manual retry]
```

### Discovery to match

```mermaid
flowchart LR
  E[Profile + area + approved photo] --> G[PostGIS radius]
  G --> F[Gender · age · skill · availability]
  F --> C[Whole-mile cards]
  C --> L[Like / pass]
  L --> M{Mutual like}
  M -->|yes| X[Match]
  X --> I[Instagram only on explicit request]
  M -->|no| C
```

### Safety and account

```mermaid
flowchart LR
  R[Report] --> Q[Reviewer queue]
  Q --> D[Dismiss]
  Q --> B[Permanent ban]
  B --> S[Revoke sessions · close matches · hide]
  B --> A[One 14-day appeal]
  DEL[Delete account] --> H[Hide + revoke sessions]
  H --> W[30-day recovery window]
  W --> P[Purge rows · S3 lifecycle]
```

## Technical decisions

| Decision | Reason |
| --- | --- |
| Go stdlib `net/http`, pgx v5 | Small surface, explicit routing and SQL |
| PostgreSQL/PostGIS source of truth | Radius, travel and filters run server-side |
| Whole-mile distance buckets | Prevents coordinate triangulation |
| Presigned S3 uploads and reads | API never receives or serves raw image bytes |
| Synchronous, fail-closed moderation | Provider failure never auto-approves |
| Static SvelteKit served by Go | Same-origin cookies, no CORS layer |
| Separate reviewer identity + TOTP | No shared admin account; MFA on every review |
| Rebase-only PRs, 8 required checks | Linear history, CI as the merge gate |

## Security invariants

- E-mail, date of birth, exact coordinates and photo object keys never leave the server.
- Instagram is disclosed only inside a live match, on explicit request.
- Contact details in names, bios and photo OCR fail closed.
- Banned accounts lose sessions, matches and discovery visibility; same e-mail cannot re-register.
- Export is owner-scoped and excludes other players' private data.
- Private responses are `no-store`; cookies are `HttpOnly`, `SameSite`, `Secure` outside local HTTP.

## Status

| Area | State |
| --- | --- |
| Backend Tasks 1–7 | Shipped: auth, profiles, photos, location, discovery, matches, safety/account |
| Frontend | Shipped: onboarding, discovery, matches, profile, safety, account, reviewer console |
| Deployable image | Shipped: multi-stage distroless, non-root, container smoke test |
| Production load and dense-city capacity | Unproven on production-like hardware |
| iOS Safari / Android Chrome | Manual pass still open |
| PITR, RPO/RTO drills | Still open |
| CD | Deferred; `main` deploys nothing |

## Docs

- [API reference](docs/api.md)
- [Local development](docs/local-development.md)
- [Security model](docs/security.md)
- [Deployment](docs/deploy.md)
- [Pentest scope](docs/pentest-scope.md)
- [UX specification](docs/design/v1-ux-spec.md)
- [CI and branch protection](docs/ci.md)
