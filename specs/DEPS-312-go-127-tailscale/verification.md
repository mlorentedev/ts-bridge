---
tags: [spec, verification, templates]
created: "2026-10-09"
---

# Verification - DEPS-312-go-127-tailscale

## Evidence

- [x] Criterion 1 -> `go version` returned `go1.27.1 windows/amd64`; `go mod tidy` followed by `go mod verify` succeeded, and `git diff --exit-code FETCH_HEAD -- go.mod go.sum` confirmed the graph matches Dependabot #431. Module files had line-ending normalization only.
- [x] Criterion 2 -> `bash scripts/tests/test-go-toolchain.sh` first failed on the original 1.26 CI pins, then passed on all four workflows; conflicting-version, missing-setup and linter-mismatch fixtures are rejected. `actionlint` succeeded on all four edited workflows.
- [x] Criterion 3 -> Windows `go build ./...`, `go vet ./...`, `go test -count=1 ./...` and pinned `golangci-lint v2.14.0 run` succeeded; all six cross-builds succeeded. The cited PR #436 head `b8ff388` was orphaned by the squash merge (`eab4bee`); the current evidence is CI at `aa3e8c6`: run `40544475350` (CI: Linux race, Windows, macOS, smoke, lint, security, six cross-builds), `40544475357` (Repo hygiene) and `40544475503` (Release), all success.
- [x] Criterion 4 -> `AGENTS.md` declares Go 1.27.1+ and installs the linter with `mise install`; since #464 the version lives only in `mise.toml`, which the guard holds equal to `.github/workflows/ci.yml`.

## Test status

- `go test -count=1 ./...` -> PASS, all Go packages on Windows after the upgrade. The original `go test ./...` baseline had one load-dependent SSH readiness timeout; its isolated test passed 3/3 before any changes.
- `go vet ./...`, `go build ./...`, `golangci-lint v2.14.0 run` -> PASS (`0 issues`). The pinned binary's published SHA256 matched its release checksums; it was built with Go 1.27.0.
- `actionlint -shellcheck=shellcheck` on four workflows, `shellcheck scripts/tests/test-go-toolchain.sh`, `bash scripts/tests/test-go-toolchain.sh`, `bash scripts/check-lessons.sh` -> PASS.
- The toolchain guard now checks each Go-running job for `setup-go` and rejects module Go versions newer than the approved linter's Go compiler or pins that drift from `AGENTS.md`.
- Cross-compile `./cmd/ts-bridge/` for linux/windows/darwin on amd64/arm64 -> PASS (6/6).
- Manual CLI smoke: `go run ./cmd/ts-bridge version` -> `ts-bridge dev (commit unknown)`.
- `go test -race ./...` not run locally: this Windows workstation does not have the required C toolchain; Linux CI exercises the race build.

## Decisions made during implementation

- `tailscale.com@v1.104.0` itself declares `go 1.27.1`; lowering our `go` directive is not valid. `actions/setup-go` now reads the module file rather than maintaining eight separate numeric pins. Lint stays pinned to v2.14.0, whose release binary is Go-1.27-built.
- The workstation's direct-first module proxy returned a zip checksum different from `go.sum` and the public sumdb for `wireguard/windows@v1.0.1`. A fresh fetch from the official Go proxy matched the sumdb; further commands used that proxy without disabling checksum checks. Lesson 037 records the operational rule.

## Independent review round 1 - 2026-10-27

Reviewer `nan/deepseek-v4-flash` returned **FAIL** at `aa3e8c6`. The signed report is
`review.md` with its matching `review-request.json`. It found no functional defect:
build, vet, tests and CI lint are green. The two REAL Majors and the minors are
dispositioned below. The contract-set edits (F1, F3, F4) change the digests, so a
fresh review is required.

| Finding | Disposition | Evidence / boundary |
|---|---|---|
| F1 Major: f4 verification unsatisfiable after #464 | Applied (contract). | f4 now checks `Go 1.27.1+` and a `mise install` line in `AGENTS.md`, the policy the guard enforces. |
| F2 Major: guard fails open on a commented `jobs:` key | Applied (code + tests). | Keys accept a trailing comment, and every Go command counts even when no recognised job holds it. Four new fixtures: commented `jobs:`, commented job key, unrecognised `"jobs":` (all rejected without `setup-go`), and a configured job under commented keys (accepted). Before the fix the suite failed on `accepted a Go job without setup-go under a commented jobs key`. Mutants: restoring the exact `^jobs:` match, restoring the exact job-key match, and restricting Go detection to recognised jobs (`in_job &&`) each make the suite fail. The real tree reports `OK (4 Go workflows)`. PR-Agent on #501 then found a second spelling: a quoted job key after a configured job inherited the previous job's `setup-go`. Job keys are now detected structurally (any non-comment line at job depth closes the previous job), with a fixture that failed before that change (`let a quoted job key inherit the previous job's setup-go`); restoring the old key pattern kills it. |
| F3 Minor: AC1 names Tailscale v1.104.0 | Applied (contract). | AC1, `What` and f1 now say v1.104.x; `go.mod` requires v1.104.1 since #438. AC3 is ticked on the `aa3e8c6` CI evidence. |
| F4 Minor: f3 hardcodes `go run …@v2.14.0` | Applied (contract). | f3 runs `mise install && golangci-lint run`, reading the pin. Local lint is still not run on this workstation (owner prohibition); CI `lint` is the evidence. |
| F5 Minor: orphaned `b8ff388` evidence, stale PR box | Applied. | Criterion 3 cites the `aa3e8c6` runs; the `tasks.md` PR box is ticked (#436). |
| F6 Minor: CRLF `go.mod`/`go.sum` on Windows | Ticketed: #498. | Separate `.gitattributes` change. Until it lands, f1 (`go mod tidy && git diff --exit-code`) is red on a Windows `core.autocrlf=true` checkout from line endings alone; run f1 on Linux or a fresh LF checkout, and do not read a Windows red as a module regression. |
| F7 Minor: guard blind to `go get`, wrappers, `GOTOOLCHAIN` | Ticketed: #499. | Scope decision needed; not in this fix. |
| F8 Minor: guard not run under zsh | Ticketed: #500. | zsh is absent on this workstation; CI owns two-shell runs. |

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-037-2026-10-09.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: dependency-driven compiler floor, not an architectural change
- [x] New pattern candidate for `00_meta/patterns/`? no: single-project module-download incident

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/DEPS-312-go-127-tailscale/` -> `specs/archive/DEPS-312-go-127-tailscale/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
