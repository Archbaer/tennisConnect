# CI and branch protection

Everything that gates a change to `main` is described here. If you change a
required check, a workflow name, or a protection setting, update this file in
the same pull request.

## Workflows

| Workflow | File | Trigger | Purpose |
| --- | --- | --- | --- |
| `CI` | `.github/workflows/ci.yml` | `push`, `pull_request` | Build, lint, test, migrations, security scan |
| `Dependency review` | `.github/workflows/dependency-review.yml` | `pull_request` | Rejects PRs that add a vulnerable dependency |

`Dependency review` is a separate workflow because it needs a base/head diff and
therefore only makes sense on `pull_request`.

### Jobs in `CI`

All jobs are required status checks on `main` (see ruleset below). Every job has
`timeout-minutes: 10`; the whole workflow is designed to stay well under that.

| Job | What it catches |
| --- | --- |
| `verify` | Compile errors via `go vet`; formatting drift; module checksum tampering (`go mod verify`); data races and test isolation/order bugs (`go test -race -count=2 -shuffle=on` against PostGIS). |
| `lint` | Issues `go vet` does not report: unchecked `rows.Err` (`rowserrcheck`), leaked HTTP bodies (`bodyclose`), unused code (`unused`), dead assignments (`ineffassign`), staticcheck's semantic checks, `gosec` security rules, spellings. Also gates `go mod tidy -diff` drift and whitespace errors. |
| `build` | Platform-specific breakage. Cross-compiles `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`. A matrix is deliberately avoided so there is one stable check name and one module cache. |
| `test-nodb` | Accidental hard dependency on a live database. Runs the suite with `TEST_DATABASE_URL` unset — DB tests must skip, not fail. |
| `migrations` | Broken or non-repeatable SQL. Applies every `migrations/*.sql` in filename order to a freshly dropped schema, twice, and asserts the expected tables plus PostGIS exist. |
| `security` | Reachable vulnerabilities in the module graph and standard library (`govulncheck`) and committed secrets (`gitleaks`). |

`lint` intentionally does **not** enable the `govet` linter: `verify` already
runs `go vet ./...` and duplicating it only doubles the failure surface.

#### Migration job notes

Migration files each open and commit their own transaction (`BEGIN; ... COMMIT;`).
They are applied with `psql -v ON_ERROR_STOP=1` and **without**
`--single-transaction`, which would nest inside the file's own transaction and
report spurious warnings. Any error fails the job. The job drops
`public` between passes, so it verifies "applies from empty" rather than
"is idempotent when re-run over existing objects" — the migrations are not
written to be re-runnable on a populated schema.

#### Cache warning in `security`

`golang/govulncheck-action` runs its own `actions/setup-go`, which resolves
`stable` (currently a newer toolchain than `go.mod`) and restores the Go module
cache a second time. That collides with the cache `setup-go` already restored in
the job, producing `tar: Cannot open: File exists` errors in the log and a
non-blocking annotation. The job passes `cache: false` to govulncheck only; the
scan itself always runs.

### Action pinning

Every action is pinned to a full commit SHA with a `# vX.Y.Z` comment. Bump
through `.github/dependabot.yml`, which groups all `github-actions` updates into
a single PR, or manually after confirming the SHA:

```sh
gh api repos/<owner>/<repo>/git/ref/tags/<tag> --jq '.object.sha'  # commit
gh api repos/<owner>/<repo>/git/tags/<sha> --jq '.object.sha'      # if annotated
```

### Local equivalents

```sh
gofmt -l .                                   # formatting
go vet ./...                                 # verify/lint
go build ./...                               # build
go mod verify && go mod tidy -diff           # module hygiene
go test ./...                                # test-nodb
TEST_DATABASE_URL='postgres://tennisconnect:local-development-only@localhost:5432/tennisconnect_test?sslmode=disable' \
  go test -race -count=2 -shuffle=on ./...   # verify
golangci-lint run --config .github/golangci.yml ./...
gitleaks detect --source .
govulncheck ./...
```

`golangci-lint` must be v2 (the config uses the `version: "2"` schema).

## Branch protection (ruleset)

`main` is protected by a repository ruleset named **Protect main**, not by
legacy branch protection. Rulesets are the modern API and support bypass actors,
which legacy protection does not.

### Applied settings

| Setting | Value |
| --- | --- |
| Ruleset name | `Protect main` |
| Enforcement | `active` |
| Target | branch, ref name `refs/heads/main` |
| Bypass actor | Repository **admin** role (`actor_id: 5`, `repository_role`, `bypass_mode: always`) |
| Require pull request | yes |
| Required approvals | 1 |
| Dismiss stale approvals on push | yes |
| Require conversation resolution | yes |
| Required status checks | `verify`, `lint`, `build`, `test-nodb`, `migrations`, `security`, `dependency-review` — all must be up to date with `main` before merging |
| Require linear history | yes (squash or rebase; no merge commits) |
| Block force pushes | yes |
| Block deletion | yes |
| Require signed commits | no (single developer, no hardware key requirement) |

### Why the admin bypass exists

This is a solo repository. Without a bypass, a requirement of 1 approving review
would make `main` impossible to merge into, because GitHub does not let an author
approve their own pull request. The repository admin role is therefore allowed to
bypass, so the owner can merge their own work. If a second maintainer joins,
remove the bypass and keep the review requirement.

Note that a bypass permits the merge; it does not silently skip CI. Status checks
still run and the ruleset still blocks a non-bypassing actor on failure.

### Changing the settings

Read the current configuration:

```sh
gh api repos/Archbaer/tennisConnect/rulesets
gh api repos/Archbaer/tennisConnect/rulesets/<ruleset_id>
```

Update it (full replacement — send the whole `rules` array):

```sh
gh api -X PUT repos/Archbaer/tennisConnect/rulesets/<ruleset_id> --input ruleset.json
```

Disable without deleting (during an incident), then re-enable:

```sh
gh api -X PUT repos/Archbaer/tennisConnect/rulesets/<ruleset_id> \
  -F enforcement=disabled
gh api -X PUT repos/Archbaer/tennisConnect/rulesets/<ruleset_id> \
  -F enforcement=active
```

Delete it entirely (leaves `main` unprotected):

```sh
gh api -X DELETE repos/Archbaer/tennisConnect/rulesets/<ruleset_id>
```

If you rename a job in `ci.yml`, update the `required_status_checks` rule in the
same pull request, otherwise the ruleset waits forever on a check that never
appears and `main` becomes unmergeable.
