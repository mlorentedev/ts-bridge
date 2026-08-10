---
tags: [spec, verification, templates]
created: "2026-08-10"
---

# Verification - REFACTOR-278-cli-command-tree

## Evidence

- [x] Criterion 1 -> `TestNewRootCmdContainsProductionCommands`, `TestRootHelp`, and existing command/flag tests exercise `NewRootCmd()`.
- [x] Criterion 2 -> `TestNewRootCmdCreatesIndependentTrees`.
- [x] Criterion 3 -> `cmd/cli/root.go` constructs every subcommand; source scans find no command singleton declarations or `func init()` under `cmd/cli`.
- [ ] Criterion 4 -> local Windows gates pass; Linux race and CI checks pending on PR #290.

## Test status

- Test suite: `go test ./...` -> all packages pass; `go test -cover ./cmd/cli` -> 37.8% statements.
- Static checks: `go vet ./...` and `golangci-lint v2.12.2 run` -> clean.
- Security: `GOOS=linux GOARCH=amd64 gosec ./...` -> 0 issues.
- Manual smoke test: built `./cmd/ts-bridge/`; root help contains every production command and `version --short` prints `dev`.
- Race detector: CI-only on Linux; the repository intentionally omits `-race` on Windows because no C toolchain is installed.
- No regressions in existing test suite: yes.

## Decisions made during implementation

- `NewRootCmd()` isolates Cobra command and flag state. Existing process hooks (`Runner`, `LoggerInit`) and build variables remain package globals and are not promised safe for concurrent execution.
- ADR-010 was amended instead of adding an ADR: this change implements ADR-013 inside the existing CLI package boundary.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons.md`? No; the replica-test failure mode is already recorded by ADR-013.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? No new ADR; ADR-010 was amended.
- [x] New pattern candidate for `00_meta/patterns/`? No; no cross-project recurrence was established.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/REFACTOR-278-cli-command-tree/` -> `specs/archive/REFACTOR-278-cli-command-tree/`
- [ ] Bitácora board ticket moved to Done / closed with PR link
- [ ] Promotions above executed (if any)
