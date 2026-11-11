---
tags: [spec, verification]
created: "2026-11-04"
id: "REL-376-contain-ssh-bootstrap"
type: spec-verification
status: active
owner: manu
---

# Verification - REL-376-contain-ssh-bootstrap

## Evidence

- [x] AC1 → `TestContainment_KillsChildWhenParentDies`. A helper parent contains a sleeping child
  and prints the child's pid. The test then kills the parent (SIGKILL / TerminateProcess) and polls
  until the child is gone or a zombie.
  - Linux (Docker `golang:1.27`): PASS.
  - Mutant: without Pdeathsig, the test fails with `child 4610 outlived its killed parent`.
  - Windows: `test-windows` CI only. Run 41887607029 on #597 passed with `ok ts-bridge/internal/bootstrapssh 1.119s`; `TestContainment_KillsChildWhenParentDies` builds on windows and has no skip.
- [x] AC2 → `TestStartSurvivesContainmentFailure`: with containment returning `access denied`,
  `start` still returns a tunnel, and `Close` works.
- [x] AC3 → `TestStartContainsTheStartedProcessAndReleasesAfterWait`: `cmd.Process` is set when
  containment runs, and `cmd.ProcessState` is set when it is released.
- [x] AC4 → `docs/runbooks/guide-control-bootstrap.md`, Routing and ownership.

## Test status

- macOS: `go test -race ./...` exit 0. This proves only that the no-op path compiles and that the
  seam tests pass; it is not containment evidence.
- Linux (Docker): `go test -race ./internal/bootstrapssh/` ok. The `Contain|Pdeathsig` tests pass
  with `-v`.
- `golangci-lint run` reports 0 issues for darwin, linux and windows. `gosec ./...` exit 0.

## Decisions made during implementation

- Containment is best effort. A failure is dropped, not surfaced. The alternative, refusing to
  start, would turn a theoretical leak into a certain outage on hosts where Job Objects nest badly.
- The Windows job handle is closed in the `Wait` goroutine, not in `Close`. Every exit path
  (graceful, early exit, startup failure) then releases it after the process has ended.
- macOS stays uncontained: there is no kernel facility for this, and a watcher process is a second
  thing to leak.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the per-OS facts are in the code comments and the runbook, nothing surprising was hit
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: an implementation detail of ADR-005's CLI-363 amendment
- [x] New pattern candidate for `00_meta/patterns/`? no: one occurrence

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/REL-376-contain-ssh-bootstrap/` -> `specs/archive/REL-376-contain-ssh-bootstrap/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)

## Review disposition (round 1, FAIL)

`review.md` (nan/mimo-v2.6-flash) returned FAIL on one Major:

- **No test guards `Start`'s containment wiring** (Major, REAL): applied. `Start` now takes its dependencies from `defaultDependencies()`, and `TestStartWiresContainment` asserts that `prepare` and `contain` there are `prepareContainment` and `containProcess`. Red-green: with both fields set to `nil` the test fails on both assertions; restored, it passes. `go test -race ./internal/bootstrapssh/` is green, and so are golangci-lint (darwin, linux, windows), `GOOS=windows go test -c` and gosec.
- **A containment refusal is dropped silently** (Minor, THEORETICAL): declined for this spec. AC2 asks that `Start` proceed, and the runbook documents the boundary. A warn-log would be a new observable contract, and no runner has ever refused job assignment.
- **Evidence not recorded** (Minor, REAL): applied. The AC boxes above are ticked. Windows evidence: `test-windows` passed on #597 (run 41887607029), with `ok ts-bridge/internal/bootstrapssh 1.119s`. `contain_kill_test.go` builds on `linux || windows` and holds no skip, so `TestContainment_KillsChildWhenParentDies` ran in that package.
- **The draft-tag pre-flight may read the transcript** (Question): answered by the archive itself. `review-transcript.jsonl` is git-ignored, and `dotf spec archive` passed its draft-tag pre-flight on three specs whose folders held a transcript (#701).

## Review disposition (round 2, PASS-WITH-GAPS)

`review.md` (nan/deepseek-v4-flash, reviewed `f5e27cd`) returned PASS-WITH-GAPS. Round 1 is kept as `review-round1.md`. No contract file changed after the review: `dotf spec digest` matches `review-request.json`.

- **No test proves `start` calls `prepare`, or calls it before `cmd.Start()`** (Major, THEORETICAL): applied. `TestStartAppliesPrepareBeforeStart` records `prepare` and asserts `cmd.Process == nil` when it runs. Both of the review's mutants now fail: dropping the call gives "start never called prepare", and moving it after `Start` gives "prepare ran after the process started". Restored, the test passes on darwin and in `golang:1.27` on linux.
- **`start` is over the 40-line limit, exempted by a baseline marker** (Minor, REAL): applied. The prepare/start/contain/wait block moved into `spawn`, the `//nolint:funlen // baseline #629` marker is gone, and `baselineCeiling["funlen"]` drops from 22 to 21. `golangci-lint run` reports 0 issues and `TestComplexityBaselineOnlyShrinks` passes.
- **`verification.md` contradicts itself on the Windows evidence** (Minor, REAL): applied. The AC1 bullet now cites run 41887607029.
- **Windows `processGone` reads any `OpenProcess` error as gone** (Minor, THEORETICAL): applied. Only `ERROR_INVALID_PARAMETER` (no such pid) reads as gone, and any other error fails the test. The Linux helper gets the same treatment: only `ErrNotExist` reads as gone. Both helpers now take `testing.TB`.
- **No scanner reads `contain_windows.go`** (Minor, REAL, repo-wide): ticketed as #709. It is a CI change outside this spec, and the owner merges it.
- **The runbook overstates the Linux Pdeathsig boundary** (Minor, THEORETICAL): applied. The Linux bullet in `docs/runbooks/guide-control-bootstrap.md` now gives the thread caveat.

## PR-Agent disposition (#710)

- **Vacuous wiring guard** (REAL): applied. `Start` now builds its dependencies through `startDependencies`. `TestStartWiresContainment` asserts that `startDependencies` is `defaultDependencies`, and `TestStartUsesItsDependencies` proves `Start` goes through it. The mutant PR-Agent named (`Start` passing an inline table without the hooks) now fails `TestStartUsesItsDependencies`. The contract files are unchanged.
