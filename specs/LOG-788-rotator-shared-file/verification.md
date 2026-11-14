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
- [x] AC2 -> `TestCompressDayAppendsToAnExistingArchive`. It failed on the old code with `archive = "run2\n"`: the first run was erased.
- [x] AC3, AC4 -> `TestFileRotator_MidnightRolloverCompressesThePastDayOnly`. It failed on the old code: no archive for the previous day after crossing midnight. It also asserts that today is never compressed and no claimed file is left behind.

The red runs used the old `Close`/`compress` with only the clock seam added, so each test failed for the behaviour, not for a missing symbol.

## Test status

- `go test -race ./...`: all packages ok.
- `golangci-lint run ./...`: 0 issues. `GOOS=windows go vet ./internal/logging`: clean.
- No regressions in the existing suite: yes.

## Decisions made during implementation

- The claim is an atomic rename to `<day>.log.<pid>.claimed`, not a lock file, so there is no stale lock to recover. A crash between the claim and the archive leaves the claimed file, and the 7-day retention removes it.
- `Open` at start-up now compresses every past day left as a `.log`, not only the one a running process held. Before, a day whose process exited was never compressed, only deleted by age.
