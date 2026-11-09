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

`gitleaks` is **diff-scoped on pull requests** — it scans the commits the PR adds
(CI logs a small number such as "7 commits scanned"), not the repository's full
history. Full-history coverage comes from the `push`-to-`main` runs of the same
workflow. Do not assume a green PR means the whole history is clean; run
`gitleaks detect --source .` locally for that.

`dependency-review` (a separate workflow) is also a required check. It needs the
repository **dependency graph** to be enabled under Settings → Code security;
with the graph disabled the action errors out with "Dependency review is not
supported on this repository" and blocks every PR.

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

#### Why `govulncheck` no longer uses an action

The `security` job runs `govulncheck` directly:

```sh
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

It previously used `golang/govulncheck-action`, which provisions its own Go
toolchain by resolving `stable`. That meant the scan ran on whatever Go happened
to be newest (1.27.x) rather than the version this repository declares, so the
required check certified a toolchain the project does not ship. The consequence
was not academic: under its `stable` toolchain the scan reported "No
vulnerabilities found" while the project's own toolchain reported reachable
standard-library vulnerabilities. Two further problems followed from that: the
check's result could change — or turn red — with no change to the repository, and
the action's nested `setup-go` restored the module cache a second time on top of
the one the job had already restored, logging `tar: Cannot open: File exists`
for every module.

Running the scanner directly fixes all three: `setup-go` provisions the go.mod
toolchain, the scanner version is pinned, and there is only one cache restore.
Both the toolchain and the scanner version are explicit, so the job is
reproducible.

The scan is only honest if the declared toolchain is patched. `go.mod` carries a
`toolchain` directive pinning a Go release with no reachable standard-library
vulnerabilities; `setup-go` honours that directive (it uses `toolchain` when
present, otherwise the `go` line). When the scanner reports findings:

- Bump the `toolchain` directive to a patched release and re-run.
- Do **not** add suppressions or `-exclude` flags to make the job pass. A green
  security check that ignores reachable vulnerabilities is worse than a red one.

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
gitleaks detect --source .                   # full history locally
govulncheck ./...
```

`golangci-lint` must be v2 (the config uses the `version: "2"` schema). Always
run `golangci-lint config verify --config .github/golangci.yml` after editing the
config: the CI action validates the schema before linting, so an invalid field
(e.g. `misspell.locale`, which only accepts `US` or `UK`, not an empty string)
fails the job without reporting a single lint finding.

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
| Bypass actors | none |
| Require pull request | yes |
| Required approvals | **0** (single maintainer; self-approval is impossible — see below) |
| Dismiss stale approvals on push | yes |
| Require conversation resolution | yes |
| Required status checks | `verify`, `lint`, `build`, `test-nodb`, `migrations`, `security`, `dependency-review` — all must be up to date with `main` before merging |
| Require linear history | yes (no merge commits) |
| Allowed merge methods | **rebase only** — squash and merge-commit are disabled at the repository level too |
| Block force pushes | yes |
| Block deletion | yes |
| Require signed commits | no (single developer, no hardware key requirement) |

### Why the review requirement is 0

The required-approvals count is deliberately **0**, and there is **no bypass
actor**.

While the project has a single maintainer, a requirement of 1 approving review
can never be satisfied: GitHub does not allow an author to approve their own pull
request, so `main` would simply stop accepting merges. The previous configuration
paired 1 approval with an admin bypass to work around that, but a bypass with
nothing left to bypass is a footgun — it silently authorises the very thing the
gate exists to stop, and it would keep doing so after a second maintainer joined
and the requirement started to matter.

Setting the count to 0 keeps everything that protects `main` and drops only the
human gate. Still enforced today:

- changes must arrive through a pull request — no direct pushes to `main`
- all seven required status checks must pass (`verify`, `lint`, `build`,
  `test-nodb`, `migrations`, `security`, `dependency-review`)
- required checks must be up to date with `main` (`strict` policy)
- linear history; no merge commits
- no force-pushes and no branch deletion

So a pull request rule with 0 approvals still means "no direct pushes, changes
arrive as PRs, CI must pass" — only the approval step is gone.

**When a second maintainer joins**, turn the human gate on and remove any
reliance on bypassing:

1. Set `required_approving_review_count` to `1` (full-replacement `PUT` of the
   rules array, see below).
2. Optionally set `require_code_owner_review` or add required reviewers.
3. Do **not** re-add a bypass actor to work around self-approval; the point of a
   second maintainer is that cross-approval is possible.

### Why rebase is the only merge method

The repository rule is that every distinct change gets its own commit. A
squash-merge collapses a pull request into a single commit and destroys that
history, so squash is disabled; a plain merge-commit would break linear history.
Rebase is therefore the only allowed method:

```sh
gh api -X PATCH repos/Archbaer/tennisConnect \
  -F allow_squash_merge=false -F allow_merge_commit=false -F allow_rebase_merge=true
```

Rebase is not the default in the GitHub UI so much as the only remaining option —
with the other two methods disabled, the merge box offers rebase exclusively.

### Approval for unattributed changes

`require_extra_approval_for_unattributed_changes` is **disabled**. It only adds a
review requirement when an *unattributed Copilot* pull request is opened under
the Copilot app identity rather than on behalf of a person. Every pull request in
this repository is opened by the owner, so the setting never applied; it is off
to keep the effective approval count exactly 1 and avoid a surprise second
approval if a Copilot-driven pull request ever appears.

### Changing the settings

Read the current configuration:

```sh
gh api repos/Archbaer/tennisConnect/rulesets
gh api repos/Archbaer/tennisConnect/rulesets/<ruleset_id>
```

Update it. `PUT` replaces only the fields you send, so a partial body is safe —
but changing a rule's parameters requires sending that whole `rules` array,
because the array is replaced wholesale:

```sh
# change approvals to 1 (send the full rules array; see ruleset.json)
gh api -X PUT repos/Archbaer/tennisConnect/rulesets/<ruleset_id> --input ruleset.json
```

Disable without deleting (during an incident), then re-enable:

```sh
gh api -X PUT repos/Archbaer/tennisConnect/rulesets/<ruleset_id> \
  -F enforcement=disabled
gh api -X PUT repos/Archbaer/tennisConnect/rulesets/<ruleset_id> \
  -F enforcement=active
```

Always read back after changing anything, and check `bypass_actors` explicitly —
it is easy to restore a ruleset and silently reintroduce a bypass:

```sh
gh api repos/Archbaer/tennisConnect/rulesets/<ruleset_id> \
  --jq '{enforcement, bypass: .bypass_actors, approvals: [.rules[] | select(.type=="pull_request") | .parameters.required_approving_review_count]}'
```

Delete it entirely (leaves `main` unprotected):

```sh
gh api -X DELETE repos/Archbaer/tennisConnect/rulesets/<ruleset_id>
```

If you rename a job in `ci.yml`, update the `required_status_checks` rule in the
same pull request, otherwise the ruleset waits forever on a check that never
appears and `main` becomes unmergeable.
