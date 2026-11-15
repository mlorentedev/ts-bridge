---
spec: "LOG-788-rotator-shared-file"
verdict: "FAIL"
reviewed_sha: "77fd8c821cfad2fe29b18d56d73a912b5fc6b732"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-14"
---

## Adversarial review

**Scope**: `LOG-788-rotator-shared-file` — whole change, `git diff 6258d6899c3f6cc0f9ca81d37df30f39462284b0...HEAD` (one commit, `77fd8c8 fix(logging): never compress a day another process is still writing`; no merge brought anything in).
**Sources**: `specs/LOG-788-rotator-shared-file/{proposal,tasks,verification}.md`, `features.json`, `internal/logging/logging.go`, `internal/logging/rotator_test.go`. `contract_digests` in `review-request.json` match `dotf spec digest LOG-788-rotator-shared-file` exactly, so the contract was not edited during this run.

**Commands actually run in this session**

- `go build ./...` → ok; `go vet ./internal/logging/` → ok; `GOOS=windows go vet ./internal/logging/` → ok; `gofmt -l internal/logging/` → empty; `golangci-lint run ./internal/logging/` → `0 issues` (version 2.14.0, pinned in `mise.toml`).
- `go test -race -count=1 ./internal/logging/` → `ok`, 3 new tests `PASS`.
- `go test -count=1 ./...` → every package `ok` (full suite, this session).
- Red-green, by mutation with the mutation reverted immediately (`git diff HEAD -- internal/logging/` empty afterwards):
  - `Close` re-compressing its day → `TestFileRotator_CloseLeavesAPeersFileInPlace` red: `the shared day file is gone after one rotator closed`.
  - `appendGzipMember` opening `dst` with `O_TRUNC` → `TestCompressDayAppendsToAnExistingArchive` red: `archive = "run2\n"`.
  - Removing the `day >= today` guard and `compressPastDays` from the rotation → `TestFileRotator_MidnightRolloverCompressesThePastDayOnly` red: no archive for the previous day.
- Two probe suites against a copy of `internal/logging` in `/tmp` (nothing written inside the repo): the claim-serialisation probe (50 iterations, two goroutines claiming one day) and external `gzip -dc` on a three-member archive. Both confirm the design as far as they go; the failing probe is F1.

### Spec and task alignment

- All four acceptance criteria have a named test that is red under the old behaviour and green now (verified by mutation, not by `verification.md` alone). AC1 → `TestFileRotator_CloseLeavesAPeersFileInPlace`; AC2 → `TestCompressDayAppendsToAnExistingArchive`; AC3/AC4 → `TestFileRotator_MidnightRolloverCompressesThePastDayOnly`.
- `tasks.md`: every box `[x]` has diff evidence. The two closing claims (`go test -race ./...`, `golangci-lint run ./...`) reproduce green here.
- No test was deleted or weakened. `internal/logging/logging_test.go` is untouched; `rotator_test.go` is new. The old `TestFileRotator_Close` only asserted `CurrentPath() == ""`, so it never enforced "Close compresses" and is not a silently weakened guard.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in any spec file.
- The diff's shape matches `proposal.md` "What": `Close` no longer compresses, one `rotateLocked` path serves `Open` and `Write`, past-only compression, claim-by-rename, append-a-member. No creep. The one behaviour the proposal does not state and `verification.md` does record as a decision: `Open` now compresses *every* past day left as a `.log`, not only one.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|---|---|---|---|---|---|---|
| Major | REAL | resilience / data loss | A compression that fails loses the day's lines permanently, and `cleanup` can delete the claim made moments earlier in the *same* rotation. `compressDay` renames `day.log` → `day.log.<pid>.claimed` (logging.go:222-232), and `appendGzipMember`'s failure path returns with the claim left in place (the comment at :230 says "the claimed file keeps the lines; retention removes it later"). Two things make that false: (a) `rotateLocked` runs `r.cleanup()` at :190 right after `compressPastDays` at :189, and `cleanup` deletes **any** `ts-bridge-`-prefixed entry older than `maxDays` by **modTime** (:309-311) — a claim inherits the *log's* last-write time, not the claim time, so a log older than 7 days is deleted in the very same rotation that claimed it; (b) nothing ever revisits a `.claimed` file — `compressPastDays` matches names ending `.log` only (:206-211), so a leftover claim is dead weight whether cleanup deletes it now or in 3 days. `proposal.md` "Risks / open questions" accepted this as "the claimed file keeps the `ts-bridge-` prefix, so retention removes it after 7 days" — the retention window is measured from the wrong clock, and the accepted loss ("removes it after 7 days") is really "silently drops the lines, sometimes immediately". Reproduced in `/tmp` probe: 8-day-old `ts-bridge-<day>.log`, archive path blocked → after one `Open()` the directory holds only `ts-bridge-<today>.log`; the log, its content and any archive are all gone. Same mechanism with an unwritable/full archive on a *fresh* log: lines survive in an orphaned `.claimed` that never gets archived — second probe, archive path blocked for a yesterday `.log`, then fixed and rotated on a later day: `ts-bridge-2026-11-13.log.<pid>.claimed` is still there and no archive was ever produced for that day, so nothing retries a claim. | Logging data only (no credential, no user data), and the trigger is an error path or a stale log — but the outcome is precisely the class of loss #788 exists to remove, and the accepted-risk text says it does not happen. | UNTESTED — no test exercises a failed `appendGzipMember`, a leftover claim, or cleanup running over a claim. | code + tests (skip `claimedSuffix` in `cleanup` and/or set the claim's mtime to now; give a leftover claim a retry or delete-after-archive path) + spec: proposal `Risks` wording (contract set → next round) |
| Minor | REAL | spec accuracy / Windows | `proposal.md` "Risks" states "Windows refuses to rename a file another process holds open. The claim fails and the day is retried on the next rotation. That is the desired behaviour", and the `compressDay` doc comment repeats it (logging.go:216-221). That premise is wrong for the only writers that matter: Go opens files on Windows with `FILE_SHARE_READ\|FILE_SHARE_WRITE\|FILE_SHARE_DELETE` (`$GOROOT/src/internal/syscall/windows/at_windows.go:156`), so `os.Rename` of a file a peer ts-bridge process still holds open **succeeds**. On Windows, therefore, the day *is* claimed, compressed and removed while the peer holds it — the same unlinked-inode behaviour the spec attributes to Unix — and the proposed "retried on the next rotation" never happens. Functionally this is still safe (every `Write` rotates before writing, so a peer cannot append to the stale inode), but the stated risk model and the code comment are inaccurate, and the spec's own escape hatch ("the claim fails") does not exist. | GOROOT source cited above; Windows-specific runtime behaviour not executed here (no Windows host). | UNTESTED — no test asserts claim behaviour against a held-open file on either OS; the `test-windows` job only compiles. | spec (proposal `Risks` — contract set) + code comment |
| Minor | THEORETICAL | tests / concurrency | The claim is the load-bearing mechanism for "two processes rotating at once never compress the same file twice", yet no committed test runs two claims concurrently — `TestCompressDayAppendsToAnExistingArchive` calls `compressDay` twice *sequentially*. My `/tmp` probe (two goroutines, 50 iterations) found exactly one member each time, so the design holds by construction; the gap is regression coverage for the property the spec advertises. | Probe passed; absent from `rotator_test.go`. | UNTESTED | tests |
| Minor | REAL | correctness (edge) | `compressPastDays` never checks `e.IsDir()` (logging.go:203-212). A directory named `ts-bridge-<past-day>.log` is renamed to `.claimed` and an empty `.gz` is created for it. Reproduced in `/tmp` probe: after `Open()` the directory listed `ts-bridge-<day>.log.<pid>.claimed` and `ts-bridge-<day>.log.gz`. Low likelihood, cheap guard. | Probe output in this run. | UNTESTED | code + tests |
| Minor | THEORETICAL | resilience / archive integrity | A write error *during* `io.Copy` leaves a truncated gzip member in the archive and returns (`appendGzipMember`, logging.go:250-257). Because members are appended, every later successful append lands after the truncation, and both `zcat` and Go's `gzip.Reader` stop at the bad member — the archive becomes unreadable past that point, silently. A write-to-temp-then-append, or a length check before dropping the claim, would keep the archive valid. Not reproduced (needs a mid-copy write failure). | Code read; the failure mode needs an injected short write. | UNTESTED | code |
| Minor | REAL | test seam consistency | A `now func() time.Time` seam was added to `FileRotator` and used by `rotateLocked` (:174), but `cleanup` still reads `time.Now()` (:299). Retention is therefore not controllable from the seam — which is why nothing in the suite can express "a log older than 7 days is claimed then deleted" (F1), and a rotator driven at a fake date applies real-time retention. | `grep`: :174 `r.now()`, :299 `time.Now()`. | UNTESTED | code + tests (use `r.now()` in `cleanup`) |
| Minor | REAL | quality | `appendGzipMember` closes `dst` twice: `defer dst.Close()` (:249) plus `return dst.Close()` (:257). Harmless today (the second error is ignored by `defer`), but it makes the returned error ambiguous and would trip a `errcheck`/`revive` tightening later. | code read; linter currently silent. | UNTESTED | code |

Not findings, checked and dismissed:

- **Multi-member gzip is readable as one stream** — spec claim verified with external `gzip -dc` over a three-member archive in `/tmp` (`one\ntwo\nthree\n`).
- **Today's file is never compressed** (AC4): `day >= today` skips today *and* future days, so a backwards clock is not a way to compress a live file; verified by mutation and by a probe with a future-dated `.log` left untouched.
- **A peer cannot append into a claimed/deleted inode**: every `Write` calls `rotateLocked` first, so a writer holding yesterday's handle rotates to today before writing. This makes the proposal's "one line lost at midnight" risk narrower than stated, not wider.
- **Claim/compress/cleanup blocking every log write under `r.mu`** happens once per day change and is bounded by the directory size; acceptable, not filed.
- **`cleanup` runs before/after ordering vs. a peer's successful compression**: a fresh claim's modTime is minutes old, so the ordinary midnight case is safe; only the stale-log case (F1) bites.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|-----------------------|
| Correctness        | C | All four ACs are genuinely proven (red under mutation, green now), but the negative path loses the data the subsystem exists to keep (F1). |
| Verification       | B | `verification.md` names each test and its red failure; I reproduced the red-green on all three and the full suite is green — but the failure path no test covers is exactly where F1 lives. |
| Scope              | A | Diff is the rotator, its tests and the spec folder; the one behaviour change beyond the proposal (`Open` compresses every past day) is recorded in `verification.md`. |
| Reliability        | C | Failed compression loses lines permanently with no retry, cleanup can eat an in-flight claim, and a short write can leave the archive unreadable past the truncation. |
| Maintainability    | B | Small focused functions, named constants, why-comments; `golangci-lint` 0 issues; `cleanup` keeps its pre-existing `nolint`, the new code introduces none. |
| Handoff-readiness  | B | Proposal/tasks/verification are complete and accurate about what they cover; no lesson or runbook touch, and the two inaccurate risk statements (F1, F2) are carried by the contract. |

### Verdict

**FAIL** — one **REAL Major** (F1: a failed or stale compression loses the lines permanently, and `cleanup` can delete an in-flight claim in the same rotation), with no named test for that path. The rubric has no D, but `severity × reality` escalates independently: a REAL Major forces FAIL. `dotf spec archive` is **not** advisable in this state.

Minimum to flip to PASS:

1. **Code** — make a claim un-deletable by a concurrent/concurrent-in-time cleanup and give it a way home: skip `claimedSuffix` entries in `cleanup`, set the claim's mtime to now at claim time, and have a later rotation either archive or remove a leftover claim.
2. **Tests** — a named regression test that fails on today's code: claim a day, make `appendGzipMember` fail (blocked/unwritable archive path), then assert the lines end up in the archive on a later rotation instead of vanishing (needs the `r.now()` seam inside `cleanup`, see the seam finding).
3. **Spec** — correct the two risk statements that the evidence contradicts: the claim's lines do not survive "7 days of retention" (measured from the log's mtime, sometimes deleted at once), and Windows does not refuse the rename of a file a Go peer holds open. Both are in `proposal.md`, so this lands as a contract change and the next round's review is the mechanism, not an inconvenience.
4. **Tests** — one concurrency test for the claim (two goroutines racing one day produce exactly one member).

### Recommended next steps

For the implementer to disposition in `verification.md` (applied / ticketed / declined with a reason), or to carry into the next ticket. Everything below `proposal.md`/`tasks.md`/`features.json` is outside the contract set and can be changed freely; the contract edits in item 3 need the re-review that this FAIL already requires:

- **Fix in code**: claim retention (skip `.claimed` in `cleanup`, mtime the claim at claim time, retry or archive-then-drop a leftover claim); `r.now()` inside `cleanup`; `e.IsDir()` guard in `compressPastDays`; drop the duplicate `dst.Close()` in `appendGzipMember`; keep the archive readable when a member write fails.
- **Add tests** (names suggested): `TestRotationKeepsLinesWhenArchiveWriteFails`, `TestCleanupDoesNotRemoveAClaimInFlight`, `TestCompressDayTwoClaimersOneMember`, `TestCompressPastDaysSkipsADirectory`.
- **Contract set (re-review follows)**: rewrite the `proposal.md` "Risks / open questions" entries for the claim's lifetime and for Windows, and the matching `compressDay` doc comment.
- **Verify on Windows**: the `test-windows` job compiles the package but never executes a claim against a held-open file; a Windows-runnable test (or a `scripts/e2e` case) is the only thing that would turn F2 from a source-read into an observation.
- **Not blocking**: multi-member gzip compatibility with external tools is confirmed with `gzip -dc`; if a downstream consumer other than `zcat`/Go reads these archives (`grep` found none in-repo), name it in a follow-up ticket.
