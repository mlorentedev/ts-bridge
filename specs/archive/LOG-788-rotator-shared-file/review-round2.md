---
spec: "LOG-788-rotator-shared-file"
verdict: "FAIL"
reviewed_sha: "635ec4f500ddb78196ddf949a6f050f34e8678fc"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-14"
---

## Adversarial review

**Scope**: `LOG-788-rotator-shared-file` — the whole change, `git diff 6258d6899c3f6cc0f9ca81d37df30f39462284b0...HEAD`. Two commits on the branch (`77fd8c8`, `635ec4f`) and no merge in the range, so the range holds only this spec's work.
**Sources**: `specs/LOG-788-rotator-shared-file/{proposal,tasks,verification,features.json}`, `specs/LOG-788-rotator-shared-file/review-round1.md`, `internal/logging/logging.go`, `internal/logging/rotator_test.go`. `dotf spec digest LOG-788-rotator-shared-file` matches the `contract_digests` in `review-request.json` byte for byte, so the contract was not edited during this run.

**Commands actually run in this session**

- `go build ./...` → ok; `go vet ./internal/logging/` → ok; `GOOS=windows go vet ./internal/logging/` → ok; `gofmt -l internal/logging/` → empty.
- `golangci-lint run ./internal/logging/` → `0 issues` (version pinned in `mise.toml`).
- `go test -race -count=1 ./...` → every package `ok`, including `internal/logging` (full suite, this session).
- Red-green by mutation, each reverted immediately (`git status --porcelain internal/` empty afterwards):
  - drop `strings.HasSuffix(e.Name(), claimedSuffix)` from `cleanup` → `TestFileRotator_FailedCompressionIsRetriedNotLost` red: `after a failed compression the lines must survive in one claim, found []`.
  - make `pastDayEntry` stop matching `.claimed` entries → same test red: no archive is ever produced for the retried day.
  - `O_APPEND` → `O_TRUNC` in `appendGzipMember` → `TestCompressDayAppendsToAnExistingArchive` red: `archive = "run2\n"`.
- Two `/tmp` probes against an unmodified copy of `internal/logging/logging.go` (nothing written inside the repo, both deleted afterwards):
  - `appendGzipMember` alone, two concurrent appends of one archive at 200 B, 4 KB, 200 KB and 3 MB — **every** run unreadable (`flate: corrupt input before offset 1`), both calls returning `nil`.
  - **End to end, two separate OS processes** (`r.Open()` each, started 5 ms apart, on a real `ts-bridge-<yesterday>.log`): 8 MB → `decoded=8388600 want=8388608` (8 bytes lost, stream ends early); 40 MB → `Error -3 while decompressing: invalid block type`. In both runs the directory afterwards held **only** `ts-bridge-<yesterday>.log.gz` and `ts-bridge-<today>.log` — no `.claimed` file, so the unarchived source was removed too. Same-pid variant (two rotators in one process, 200 µs apart, 1/8/40 MB) was corrupt in all three runs.

### Spec and task alignment

- All five acceptance criteria have a named test, and the round-1 fixes are real: `cleanup` now skips claims by suffix, `compressPastDays` retries claims, `appendGzipMember` truncates back on failure, `pastDayEntry` requires a regular file, `cleanup` uses `r.now()`, and `dst` is closed once. I re-mutated three of them this session (claim-retention in `cleanup`, claim-retry in `compressPastDays`, append-mode in `appendGzipMember`) and the suite went red on each; the truncate-back branch is the exception (F2).
- `tasks.md`: every `[x]` has diff evidence; the closing claim (`go test -race ./...`, `golangci-lint run ./...`) reproduces green here.
- No test was deleted or weakened: `internal/logging/logging_test.go` is untouched in the range, `rotator_test.go` is new, and the old `TestFileRotator_Close` only ever asserted `CurrentPath() == ""`.
- `features.json` maps each AC to a runnable `-run` command; all four commands pass.
- The one behaviour beyond the proposal's "What" (`Open` at start-up compresses *every* past `.log`, not only one) is recorded in `verification.md` "Decisions made during implementation".
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in any spec file.
- Round 1's findings are honestly dispositioned in `verification.md`, and the round-1 contract edits (Windows `FILE_SHARE_DELETE`, claim lifetime) are present in `proposal.md` — but see F1, the corrected Windows text now sits beside a consequence claim — "duplicated lines, never lost ones" — that this session's reproduction contradicts.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|---|---|---|---|---|---|---|
| Blocker | REAL | resilience / archive integrity | **Two processes rotating at the same time interleave two gzip members into one archive: the day's archive is corrupt or truncated, and both claims are removed, so the lines are gone — not "duplicated", as `proposal.md` accepts them to be.** The retry path deliberately lets a process take another process's live claim: `compressPastDays` calls `archiveClaim(path, day)` for any `.claimed` entry, and `archiveClaim` renames it to its own pid's claim and then appends (`logging.go:206-212`, `:254-266`). While process A is inside `appendGzipMember` on that archive, process B opens the same `.gz` with `O_APPEND` and writes its own member concurrently; `gzip.Writer` emits its stream in several `write` calls, so the members interleave byte-wise. The archive is corrupt from the interleaving point on, **every earlier member in that `.gz` becomes unreadable too**, both calls return `nil` (so the truncate-back guard never fires), and each process removes its claim — the source is gone as well. The proposal's accepted risk states "The result is duplicated lines, never lost ones, and it needs two processes rotating within the same milliseconds"; the window is not a millisecond but the whole compress-and-append of that day's log, which scales with its size (measured: 5 ms for a 1 MB log, 80 ms for 40 MB), so any two processes whose first post-midnight write lands inside it lose the archive. | **Reproduced end to end in this session with an unmodified copy of the package**: two OS processes calling `FileRotator.Open()` 5 ms apart on a `ts-bridge-<yesterday>.log` — 8 MB → `decoded=8388600 want=8388608` (8 bytes lost); 40 MB → `Error -3 while decompressing data: invalid block type`; in both runs the directory held only the `.gz` and today's `.log`, no claim. Same-pid variant (two rotators, one process, 200 µs apart): corrupt at 1, 8 and 40 MB. Function-level probe: 200 B→3 MB payloads all `flate: corrupt input before offset 1`, both `appendGzipMember` calls returning `nil`. Reachability from code cited above; the trigger window is the one `proposal.md` already concedes. | UNTESTED — `TestCompressDayConcurrentClaimsArchiveOnce` races two `compressDay` calls on the *same* path, so the losing rename fails before any append; nothing exercises two appends to one archive. | code (make the member append a single atomic write, or serialize per-day with `flock`/`LockFileEx`) **+** tests (a two-appender test asserting `gzip.Reader` reads both members) **+** spec (`proposal.md` "Accepted" line, contract set → the re-review this FAIL already requires) |
| Minor | REAL | tests / coverage | The truncate-back-on-failure branch of `appendGzipMember` (`logging.go:284-287`) has **no** coverage, although `verification.md` and `proposal.md` both advertise it ("a write that fails midway truncates the archive back to its old size, so the archive never holds a partial member"). The only failure test blocks the archive with a *directory*, so `os.OpenFile` fails at `:270` and neither `io.Copy` nor `gw.Close` nor `Truncate` ever executes; the branch is also what F1 shows does not run when two writers interleave. | `TestFileRotator_FailedCompressionIsRetriedNotLost` (`rotator_test.go:120-137`) creates `blocked` as a directory; read of `logging.go:264-288`. | UNTESTED | code (a `writer`/`dstPath` seam) + tests (inject a short write, assert the archive keeps its previous members) |
| Minor | THEORETICAL | correctness / claim names | `archiveClaim` renames onto a deterministic per-pid claim name (`:%d`, `logging.go:255`). `os.Rename` **replaces** an existing target, so if a `.log` for a past day and a claim of the *same* pid for that day ever coexist, the rename destroys the claim's unarchived lines before they are read. Reachable when two processes share one logs volume with the same pid (pods with `shareProcessNamespace` off, so both are PID 1) — the same-pid case also collapses the claim to "no rename at all" and lands in F1. | Code read of `logging.go:255-262`; `os.Rename`'s replace semantics. No repro (needs two pid-1 processes on one directory). | UNTESTED | code (use `os.Link`+unlink, or an exclusive create/claim name without pid collision) |
| Minor | THEORETICAL | resilience / alibi | `cleanup` never ages out a claim, which is the round-1 fix — but a `.log` whose **rename** (not its append) keeps failing is still a plain `.log`, so retention deletes it after `maxDays` by mtime and the lines go with it. The spec's "nothing is lost" relies on every failure leaving a `.log` in a claim, which is only true once the rename succeeded. | `logging.go:314-320` (claim guard) vs `:337-345` (age delete); `archiveClaim` returns silently on rename error (`:257-259`). | UNTESTED | code + tests |
| Minor | SPECULATIVE | API | `FileRotator.now` is now called unconditionally (`:174`, `:337`), so a zero-value `logging.FileRotator{}` — legal to construct from another package, since the type is exported — panics on a nil func where it previously worked. No caller does this today (`grep 'FileRotator{'` finds only `NewFileRotator`). | `logging.go:118-121`, `:174`; `grep -rn 'FileRotator{' --include=*.go .`. | UNTESTED | code (fall back to `time.Now` when `now == nil`) |

Not findings, checked and dismissed:

- **AC1, AC2, AC3, AC4 hold** — AC2 re-mutated this session (`O_TRUNC` → red), AC1/AC3/AC4 re-read against the code and green in the full `-race` run; round 1 mutation-tested them and those paths are unchanged by the fix commit.
- **A peer cannot append into a claimed/unlinked inode** while this process holds it: every `Write` rotates first, so the claim only ever holds completed lines.
- **The claim is never deleted by retention** (`strings.HasSuffix(..., claimedSuffix)` skip), verified red by mutation.
- **Two same-path claimers archive once** — the losing `os.Rename` fails; the property itself holds.
- **Multi-member gzip reads back as one stream** — round 1 verified with external `gzip -dc`; the probe here also shows a sequential two-member archive is readable.
- **Windows claim behaviour** — `GOOS=windows go vet` is clean and the proposal's corrected text matches Go's `FILE_SHARE_DELETE` open flags; not executed on a Windows host (UNVERIFIED at runtime).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|-----------------------|
| Correctness        | C | ACs 1-4 are genuinely proven, but the concurrency clause of AC5 ("two concurrent compressions of one day archive it once") is only true when the two do not overlap in time: under the real overlap the archive is corrupt. |
| Verification       | B | Every AC names a test, and I reproduced three of the five mechanisms red by mutation myself plus the full suite; the advertised truncate-back guarantee and the concurrency path have no covering test. |
| Scope              | A | Diff is `internal/logging` plus its spec folder; no creep, no dependency, no contract touched outside the spec. |
| Reliability        | C | The failure path is better than round 1, but a reachable race turns the day's archive unreadable, and an unarchivable `.log` can still be aged out. |
| Maintainability    | B | Small named functions, constants, why-comments; `golangci-lint` 0 issues; one deterministic (pid-based) claim name is the load-bearing shortcut in F1/F3. |
| Handoff-readiness  | B | Proposal/tasks/verification are complete and honest about what changed; the two non-obvious facts (multi-member append, `FILE_SHARE_DELETE` rename) live only in code comments, not in `docs/lessons.md`. |

### Verdict

**FAIL** — one **Blocker** (F1: two processes rotating at the same time interleave two gzip members into one archive, leaving it corrupt or truncated with both sources removed; reproduced end to end with two OS processes against the real package code, and `proposal.md` accepts this window on the stated ground that the outcome is "duplicated lines, never lost ones"). A Blocker forces FAIL whatever its reality tag, and this one is REAL into the bargain; the rubric has no D. `dotf spec archive` is **not** advisable in this state.

Minimum set that would flip it to PASS:

1. **Code** — remove the interleaving (the one Blocker): build the gzip member in memory (or a temp file) and append it to the archive in a single `Write` on the `O_APPEND` fd, or serialise per-day appends with a lock (`flock` on Unix, `LockFileEx` on Windows). Either makes the concurrent case the accepted one — duplicates — instead of corruption.
2. **Tests** — a named regression test that is red on today's code: two goroutines call `appendGzipMember` on one archive path with content larger than one write buffer, then `gzip.Reader` must read both members.
3. **Spec (`proposal.md`, contract set)** — restate the accepted risk to match what the fix actually delivers, and correct the window ("the same milliseconds" vs. the whole compress-and-append of that day's log). Because `proposal.md` is in the contract set, this lands as a contract change and the next round's review is the mechanism, not an inconvenience.

### Recommended next steps

For the implementer to disposition in `verification.md` (applied / ticketed / declined with a reason), or to carry into a follow-up ticket. Items 1–2 are the FAIL; the rest are tracked gaps, not archive blockers:

- **Fix in code**: atomic member append or per-day append lock (F1); `os.Link`-based claim or a claim name that cannot collide (F3); `now == nil` fallback in `FileRotator` (F5); do not let retention age out a `.log` whose claim rename keeps failing (F4).
- **Add tests** (names suggested): `TestArchiveAppendIsReadableUnderConcurrentAppends` (F1), `TestArchiveKeepsPreviousMemberWhenAMemberWriteFails` (F2), `TestCompressPastDaysSkipsALogWithALiveClaim` (if you choose the hold-off variant).
- **Contract set (re-review follows)**: rewrite the "Accepted" line in `proposal.md` "Risks / open questions" for the real consequence and the real window.
- **Verify on Windows**: the runtime claim path is still unexecuted — `GOOS=windows go vet` only compiles. A Windows-runnable test or a `scripts/e2e` case is the only way to turn that from a source read into an observation.
- **Knowledge**: no lesson records the two durable facts this change turned on (append-a-gzip-member instead of truncate; Go renames past `FILE_SHARE_DELETE` holds on Windows). `docs/lessons/` is where that belongs, alongside lesson-062.
