---
tags: [spec, verification, templates]
created: "2026-10-22"
---

# Verification - CLI-476-setup-exit-status

## Evidence

- [x] AC1 (failed step exits 2, text and JSON, result printed) -> commit `8ca1783` / `TestHostSetupCmd_ExitStatus` "a failed step, text" and "a failed step, json" (red before the change: `exit 0, want 2`)
- [x] AC2 (ok, changed and manual exit 0) -> commit `8ca1783` / `TestHostSetupCmd_ExitStatus` "ok, changed and manual, text" and "..., json"
- [x] AC3 (not elevated exits 1) -> commit `8ca1783` / `TestHostSetupCmd_ExitStatus` "not elevated"; `smoke.bats` "host setup: without root, refuses" unchanged
- [x] AC4 (ADR states it) -> commit `8ca1783` / `docs/adr/adr-017-host-desired-state.md` §2 amendment; CLI reference updated

## Test status

- Test suite: `go test -race -count=1 ./...` -> all ok; `GOOS={linux,windows,darwin} go vet ./...` clean; `golangci-lint run ./...` 0 issues
- Manual smoke test: none; setup mutates the machine, so the command is driven through the injected `hostSetup` instead
- No regressions in existing test suite: yes

## Decisions made during implementation

- `host setup` returned early on macOS ("not applicable") using `runtime.GOOS`, so the first test run on a Mac never reached the exit path. The OS is now the package variable `hostGOOS` (lesson 040: make the OS an input), set to `linux` by the test helper.
- The JSON path returns the print error first and the exit status second, so a marshal failure still exits 1, not 2.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the one non-obvious point (OS as an input) is lesson 040 already
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: docs/adr/adr-017-host-desired-state.md
- [x] New pattern candidate for `00_meta/patterns/`? no: a single-repo CLI contract

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CLI-476-setup-exit-status/` -> `specs/archive/CLI-476-setup-exit-status/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
