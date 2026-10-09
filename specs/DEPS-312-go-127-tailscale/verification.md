---
tags: [spec, verification, templates]
created: "2026-10-09"
---

# Verification - DEPS-312-go-127-tailscale

## Evidence

- [x] Criterion 1 -> `go version` returned `go1.27.1 windows/amd64`; `go mod tidy` followed by `go mod verify` succeeded, and `git diff FETCH_HEAD -- go.mod go.sum --exit-code` confirmed the graph matches Dependabot #431. Module files had line-ending normalization only.
- [x] Criterion 2 -> `bash scripts/tests/test-go-toolchain.sh` first failed on the original 1.26 CI pins, then passed on all four workflows; a conflicting `go-version` fixture first failed the test, then was rejected after the guard fix. `actionlint` succeeded on all four edited workflows.
- [ ] Criterion 3 -> Windows `go build ./...`, `go vet ./...`, `go test -count=1 ./...` and pinned `golangci-lint v2.14.0 run` succeeded; all six cross-builds succeeded. Linux race and Windows CI checks remain pending until the PR runs.
- [x] Criterion 4 -> `AGENTS.md` declares Go 1.27.1+ and installs golangci-lint v2.14.0; `go.mod` and `.github/workflows/ci.yml` match.

## Test status

- `go test -count=1 ./...` -> PASS, all Go packages on Windows after the upgrade. The original `go test ./...` baseline had one load-dependent SSH readiness timeout; its isolated test passed 3/3 before any changes.
- `go vet ./...`, `go build ./...`, `golangci-lint v2.14.0 run` -> PASS (`0 issues`). The pinned binary's published SHA256 matched its release checksums; it was built with Go 1.27.0.
- `actionlint -shellcheck=shellcheck` on four workflows, `shellcheck scripts/tests/test-go-toolchain.sh`, `bash scripts/tests/test-go-toolchain.sh`, `bash scripts/check-lessons.sh` -> PASS.
- Cross-compile `./cmd/ts-bridge/` for linux/windows/darwin on amd64/arm64 -> PASS (6/6).
- Manual CLI smoke: `go run ./cmd/ts-bridge version` -> `ts-bridge dev (commit unknown)`.
- `go test -race ./...` not run locally: this Windows workstation does not have the required C toolchain; Linux CI exercises the race build.

## Decisions made during implementation

- `tailscale.com@v1.104.0` itself declares `go 1.27.1`; lowering our `go` directive is not valid. `actions/setup-go` now reads the module file rather than maintaining eight separate numeric pins. Lint stays pinned to v2.14.0, whose release binary is Go-1.27-built.
- The workstation's direct-first module proxy returned a zip checksum different from `go.sum` and the public sumdb for `wireguard/windows@v1.0.1`. A fresh fetch from the official Go proxy matched the sumdb; further commands used that proxy without disabling checksum checks. Lesson 037 records the operational rule.

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
