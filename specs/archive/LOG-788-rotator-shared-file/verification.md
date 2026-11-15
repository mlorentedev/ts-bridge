---
tags: [spec, verification, templates]
created: "2026-11-14"
id: "LOG-788-rotator-shared-file"
type: spec-verification
status: active
owner: manu
---

# Verification - LOG-788-rotator-shared-file

## Evidence

- [x] AC1 -> `TestFileRotator_CloseLeavesAPeersFileInPlace`. It failed on the old code with "the shared day file is gone after one rotator closed".
- [x] AC2 -> `TestCompressDayNeverOverwritesAnArchive`. The original code failed the equivalent test with `archive = "run2\n"`: the first run was erased.
- [x] AC3, AC4 -> `TestFileRotator_MidnightRolloverCompressesThePastDayOnly`. It failed on the old code: no archive for the previous day after crossing midnight. It also asserts that today is never compressed and no claimed file is left behind.

- [x] AC5 (round 1 review findings) -> `TestFileRotator_FailedCompressionIsRetriedNotLost`, which failed before the fix with "found []" because retention deleted the claimed lines in the same rotation. `TestFileRotator_LeavesADirectoryNamedLikeALog` failed before the fix ("the directory was moved"). `TestCompressDayConcurrentClaimsNeverCorrupt` asserts that two concurrent compressions leave every archive whole and no claim behind (20 iterations, -race; `go test -race -count=20 ./internal/logging/` green). `TestFileRotator_LeavesAFreshClaimAlone` covers the stale-age guard.

The red runs used the old `Close`/`compress` with only the clock seam added, so each test failed for the behaviour, not for a missing symbol.

- Round 2 review (FAIL, deepseek) Blocker: retrying another process's live claim let two writers append to one `.gz`, which interleaved the gzip members and corrupted the archive. `test-windows` reproduced it (`flate: corrupt input before offset 1`). Round 3 removes the shared writer: one archive per compression, published without clobbering; unique claim names; only stale claims are retried; retention deletes `.gz` only (the retention test now expects an old `.log` to be kept); a zero-value rotator uses `time.Now`.

## Test status

- `go test -race ./...`: all packages ok.
- `golangci-lint run ./...`: 0 issues. `GOOS=windows go vet ./internal/logging`: clean.
- No regressions in the existing suite: yes.

## Decisions made during implementation

- Round 1 review (FAIL, deepseek) fixes:
  - cleanup never ages out a `.claimed` file, and reads the rotator's clock;
  - leftover claims for past days are retried on each rotation;
  - a failed append truncates the archive back to its old size;
  - only regular files are treated as logs;
  - `dst` is closed once;
  - the Windows risk text was corrected.
- The claim is an atomic rename to `<day>.log.<pid>.claimed`, not a lock file, so there is no stale lock to recover. A crash between the claim and the archive leaves the claimed file; the next rotation in any process retries it.
- `Open` at start-up now compresses every past day left as a `.log`, not only the one a running process held. Before, a day whose process exited was never compressed, only deleted by age.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-063-2026-11-14.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-018 (Logs) already decides that a process never removes a file another process writes; this spec implements it.
- [x] New pattern candidate for `00_meta/patterns/`? no: write-once publication of a shared artifact is established practice, and one occurrence here is not a cross-project recurrence.

