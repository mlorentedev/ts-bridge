---
tags: [spec, verification, templates]
created: "2026-10-21"
---

# Verification - HOST-004-host-step-model

## Evidence

- [x] AC1 (PR A) -> `TestOutcomeSucceeded`; `TestSetupFor` asserts `Success == Outcome.Succeeded()` and a non-empty `Fix` on every non-success step
- [x] AC2 (PR A) -> `TestSetupFor`, `TestSetupForWindowsRDPFailureFallsBackToDefaultPort`, `TestCheckFor`: exact argv per OS, copied from the pre-HOST-004 tagged code, all run on any runner
- [ ] AC3 (PR B)
- [ ] AC4 (PR B)
- [ ] AC5 (PR B)

## Test status

- `go test -race -count=1 ./...` -> all packages ok
- `GOOS={windows,darwin,linux} go vet ./internal/host/ ./cmd/cli/` -> clean
- `golangci-lint run ./...` (v2.14.0, plus `GOOS=windows` and `GOOS=linux` on `internal/host`) -> 0 issues
- Mutation: `-Profile Any` -> `-Profile Private` in the Windows firewall script reddens both Windows `TestSetupFor` cases; dropping `route` from the ufw call reddens the Linux case.
- No regressions: the existing `internal/host` and `cmd/cli` host tests pass unchanged.

## Decisions made during implementation

- Step definitions moved to untagged `linux_steps.go`, `windows_steps.go` and `step.go`. The `_steps` suffix keeps Go from treating them as GOOS-constrained files. The tagged files keep only `isElevatedImpl`.
- Logging moved to `host.Setup` and `host.Check` (one line per step, plus one `host check` line), so Linux now logs the same shape Windows did. This is the only observable change, as declared in the proposal.
- PR A stays under the LOC cap (non-test diff: +311 / -391 including comments), so Windows was not split out.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [ ] Lesson for the repo's `docs/lessons/`? <yes: path / no: reason>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes: path / no: reason>
- [ ] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. <yes: path / no: reason>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/HOST-004-host-step-model/` -> `specs/archive/HOST-004-host-step-model/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
