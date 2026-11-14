---
spec: "LOG-788-rotator-shared-file"
verdict: "PASS"
reviewed_sha: "1f810a8d82de1bad40d18e49388365ae01c5d676"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-14"
---

## Adversarial review

**Scope**: `LOG-788-rotator-shared-file` — the whole change, `git diff 6258d6899c3f6cc0f9ca81d37df30f39462284b0...HEAD`, and among it the three commits made on the branch (`160ec9a`, `635ec4f`, `77fd8c8`). The rest of the range arrived through merges (`internal/agent`, `internal/browser`, `cmd/cli/browser*`, `go.mod`) and is other work; it shares no code with `internal/logging`, so it is judged only where it interacts, i.e. not at all.
**Sources**: `specs/LOG-788-rotator-shared-file/{proposal,tasks,verification,features.json}`, `review-round1.md`, `review-round2.md`, `internal/logging/logging.go`, `internal/logging/rotator_test.go`, `internal/logging/logging_test.go`. `dotf spec digest LOG-788-rotator-shared-file` equals the `contract_digests` in `review-request.json` field for field, so the contract was not edited during this run, and this review is not stale.

**Commands actually run in this session**

- `go build ./...` → ok; `gofmt -l internal/logging/` → empty; `go vet ./internal/logging/` → ok; `GOOS=windows go vet ./internal/logging/` → ok.
- `go test -race -count=1 -timeout 8m ./...` → **every package `ok`**, `internal/logging` in 1.376s (full suite, this session).
- `golangci-lint run ./internal/logging/` with the pin from `mise.toml` (2.14.0) → `0 issues.`
- Red-green by mutation, run against a copy of the package in a `mktemp -d` tree (nothing written inside the repo), each mutation reverted:
  - `os.Link(tmp, …".gz")` → `os.Rename(tmp, …".gz")` (clobber) → `TestCompressDayNeverOverwritesAnArchive` red: `first archive = "run2\n", want run1 untouched`.
  - drop the `claim && !r.staleClaim(e)` guard → `TestFileRotator_LeavesAFreshClaimAlone` red: `a fresh claim was taken`.
  - drop `|| day >= today` in `pastDayEntry` → `TestFileRotator_MidnightRolloverCompressesThePastDayOnly` red: `yesterday's archive = ""`.
  - make `Close` compress the day again (the AC1 regression) → `TestFileRotator_CloseLeavesAPeersFileInPlace` red: `the shared day file is gone after one rotator closed`.
- **End-to-end multi-process probe, the round-2 Blocker's own shape** (six real OS processes, three paired rounds, 5 s of work per round, all against a copy of the unmodified package): a directory holding an 8 MB `ts-bridge-2026-11-13.log`, a 2 MB `ts-bridge-2026-11-12.log` and a 3 MB stale claim `ts-bridge-2026-11-12.log.777.5.claimed`, with two `Open()`+`Write()` processes racing each round. Afterwards: `ts-bridge-2026-11-13.log.gz` = 10,666,669 bytes (source intact), `ts-bridge-2026-11-12.log.gz` = 2,666,669 and `ts-bridge-2026-11-12.log.93066.<ns>.gz` = 4,000,001 — every archive `gzip -t` clean and decoding wholly, **no `.claimed` and no `.partial` left**, no source line lost. Round 2's corruption (`flate: corrupt input`) does not reproduce in this design, because nothing is ever appended to a published archive.
- Orphan probe: a `ts-bridge-2026-11-10.log.1.2.gz.partial` and a future-dated `ts-bridge-2030-01-01.log`, both dated 2020, survived a full rotation untouched (F1, F2).

### Spec and task alignment

- **AC1–AC5 each have a named test, and each test is load-bearing**: I re-derived four of them red by mutation in this session (the list above), and the fifth (`TestCompressDayNeverOverwritesAnArchive`, AC2) is red under the clobbering publication it guards against. The tests fail for the behaviour, not for a missing symbol.
- **AC5's concurrency clause is the weak link, not because the behaviour is wrong but because the test does not exercise it** — F1. My probe is the evidence that the property holds; the suite does not yet hold it durably.
- `tasks.md`: every `[x]` has diff evidence, including "`Close` stops compressing" and the single `rotateLocked(today)` path for `Open` and `Write`. The closing claim (`go test -race ./...`, `golangci-lint run ./...` green) reproduces green here: the full suite under `-race`, and `0 issues` from the 2.14.0 linter over the touched package.
- `features.json` maps each AC to a runnable `-run` command; all four commands pass, and I ran the whole package under `-race` rather than only those selectors.
- **Round 2's contract request is honoured**: `proposal.md` "Risks / open questions" no longer claims the accepted overlap outcome is "duplicated lines, never lost ones" for an append design; it now describes one archive per compression, `os.Link` refusing an existing name, and duplication as the only consequence of a retried live claim. The code matches that text line for line (`writeArchive`, `logging.go:290-308`).
- **No test was weakened.** `internal/logging/logging_test.go` changes by three lines in `TestFileRotatorCleanup_Retention`, and it *tightens*: `ts-bridge-just-outside-age.log` must now survive retention, which is exactly the "a `.log` holds unarchived lines, so age never deletes it" rule. The old `TestFileRotator_Close` never asserted more than `CurrentPath() == ""`, so nothing was lost there either.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in any spec file.
- The retention rule the proposal states ("removes archives (`.gz`) only") is what `cleanup` does (`logging.go:350-355`), verified by that strengthened test.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|---|---|---|---|---|---|---|
| Minor | REAL | tests / coverage | **F1 — AC5's concurrency clause has no test that would catch its regression.** `TestCompressDayConcurrentClaimsNeverCorrupt` hands both goroutines the *same* source path, so each renames it to its *own* unique claim name and the loser's `os.Rename` fails before any write happens — the test observes claim arbitration, never two simultaneous archive writes. It would have passed on round 2's corrupting append design too, which is the measurement that matters: the guard for the one Blocker this spec has is not the test it is named after. | Code read of `rotator_test.go:186-212` vs `logging.go:267-271`; the property itself does hold — proved by the six-process probe described above, not by this test. | **UNTESTED** (the named test does not exercise the path) | tests (e.g. `TestWriteArchiveTwoClaimsOfOneDayAreBothWhole`: prepare two `.claimed` files of one day, call `writeArchive` from two goroutines, assert every `*.gz` decodes to its full source) |
| Minor | THEORETICAL | resilience / disk | **F2 — A `.partial` file left by a crash mid-compression is never removed and never retried.** `writeArchive` removes its temp only on the *error* return (`logging.go:292-296`); a process killed between `gzipFile` and the publish leaves `ts-bridge-<day>.log.<pid>.<ns>.gz.partial`, which `pastDayEntry` skips (suffix is `.claimed`-only) and `cleanup` skips (suffix `.gz`-only). It is wasted space, not lost lines — the claim still holds them — but nothing will ever collect it. | Probe: a `.partial` dated 2020 survived a rotation in this session. `logging.go:237-255`, `:350-355`. | UNTESTED | code (sweep `*` + `.gz` + `partialSuffix` in `cleanup` or `compressPastDays`) + tests |
| Minor | THEORETICAL | resilience / disk | **F3 — A future-dated `.log` is never archived and never aged out.** `pastDayEntry` requires `day < today`, and retention now deletes only `.gz`; so a log written once under a forward clock skew (`ts-bridge-2030-01-01.log`) accumulates permanently. Before this change it would at least have been deleted by age. | Probe: `ts-bridge-2030-01-01.log` dated 2020, 5 KB, survived a rotation untouched. `logging.go:245-247`, `:353`. | UNTESTED | code (a separate age rule for `.log` files that are not today's, or archive anything `!= today` and let the ordering say so) + tests |
| Minor | THEORETICAL | filesystems without hard links | **F4 — Where `os.Link` cannot work (a logs directory on a share, exFAT, some FUSE mounts), every publish takes the unique-name fallback: `ts-bridge-<day>.log.gz` is never created and each run of a day mints another `.gz`, so `maxFiles = 10` evicts days of history markedly faster than on NTFS/APFS. The change behaves correctly, but neither `proposal.md` nor `verification.md` records the degradation. | Code read of `logging.go:297-307`; `os.Link`'s documented failure modes. Not reproduced (no non-hardlink volume available here). | UNTESTED | tests + spec (`verification.md`, outside the contract set) |
| Minor | SPECULATIVE | naming | **F5 — Claim and temp names are `pid + time.Now().UnixNano()`. Within one process a coarse clock could hand the same nanosecond to two compressions: the second `os.Rename` would then *replace* the first's claim (renames overwrite), and `gzipFile`'s `O_EXCL` would make the loser delete the winner's temp. Distinct processes are safe (pids differ); round 2's deterministic pid-only name was the same hazard with a wider window, so this is strictly narrower, not new. | Code read of `logging.go:282-284`, `:322`; no reproduction. | UNTESTED | code (a monotonic per-process counter appended to the name) |
| Minor | THEORETICAL | concurrency / midnight race | **F6 — A write that straddles the rotate check can land in an inode a peer has just archived and unlinked.** `Write` takes `r.mu`, calls `rotateLocked` (which reads the clock and returns when the day has not changed) and only then `r.current.Write(p)` (`logging.go:141-148`). A peer whose clock has *already* crossed midnight renames that day's `.log` to a claim, compresses it and removes the claim in that gap; the first process then writes into an unlinked inode and its error is `nil`. The gap is one function call wide when both processes read the same clock, so it is a sub-microsecond window once a day — versus the previous behaviour, where `Close` unlinked the day under a live peer on *every* exit. `proposal.md` states the peer "never appends to a claimed file"; that holds for the day check, not for an fd already held across it. | Code read of `logging.go:141-148` vs `:178-197`, `:262-279`; no reproduction (the window needs a deschedule between the two lines at a midnight boundary). | UNTESTED | code (re-check `r.currentDay` after the write and re-file the line, or keep the claim linked until no fd can be open on it) + tests |
| Minor | THEORETICAL | quality | **F7 — `compressDay` (`logging.go:257-260`) survives only as a test seam: production rotation calls `archive` from `compressPastDays`, so the wrapper exists for `rotator_test.go` alone. Two test files now call it, which makes it cheap to keep — but it is production surface with no production caller. | `grep -rn 'compressDay' --include=*.go .` → three call sites, all in `rotator_test.go`. | n/a | code (or a comment saying it is a test seam) |

Checked and dismissed (not findings):

- **The round-2 Blocker is fixed, in the shape it was found.** Nothing is appended to a published archive any more: each compression writes its own gzip to a uniquely named temp and publishes with `os.Link` (refusing an existing name) or a unique rename. Six racing processes left every archive whole and lost no line. The residual duplicate-archive case is the one `proposal.md` accepts, and it is duplication, not corruption.
- **A peer cannot append into a file that is *already* claimed or unlinked**: every `Write` rotates first, so the claim only ever holds completed lines — with the one straddling-write residual recorded as a Minor above.
- **Today's file is never compressed** (`day >= today` guard, mutated red).
- **A directory named like a log is left alone** (`e.Type().IsRegular()`, covered by `TestFileRotator_LeavesADirectoryNamedLikeALog`).
- **A failed compression keeps the lines past retention and is retried when stale** (`TestFileRotator_FailedCompressionIsRetriedNotLost`, and the retention skip mutated red in round 2's session; the retention rule itself is re-verified here through the strengthened `TestFileRotatorCleanup_Retention`).
- **A zero-value `FileRotator` works** — round 1's F5 is closed by `clock()` and covered by `TestFileRotator_ZeroValueUsesTheWallClock`.
- **`os.Link` publishes correctly on this host** — both canonical archives in my probe were created through the link path and decode wholly, so this is an observation on darwin/APFS, not only a source read.

UNVERIFIED in this session:

- **`gosec`** is not installed in this worktree, so the `security` job was not reproduced; the diff's `#nosec` annotations are the same kinds the package already carried.
- The full-repo `golangci-lint run` was run only over `./internal/logging/`, the package this change touches.
- **Windows runtime behaviour** — `GOOS=windows go vet` compiles; `os.Link`/`os.Rename` under `FILE_SHARE_DELETE` was not executed on a Windows host.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|-----------------------|
| Correctness        | B | All five ACs hold and I re-derived four of them red by mutation plus the fifth by construction; the gap is negative-path coverage — the AC5 concurrency test does not exercise what it names (F1) and two orphan paths are unguarded. |
| Verification       | B | `verification.md` names a test per AC and its red-green claims stand up; I reproduced the mechanisms independently, but the concurrency evidence rests on my probe rather than on anything committed, and no Windows run is recorded. |
| Scope              | A | The branch's three commits are `internal/logging` plus its spec folder; no dependency, no other contract, no creep. The other files in the range came in through merges. |
| Reliability        | B | Failure paths keep the lines and retry them once stale, and the corruption class is gone; `.partial` orphans and future-dated logs accumulate forever, and one straddling write at midnight can still land in an unlinked inode. |
| Maintainability    | B | Small named functions, why-comments, constants at the top, `golangci-lint` baseline untouched; one test-only wrapper in production code and two redundant `HasSuffix(".gz")` tests inside an already-filtered loop. |
| Handoff-readiness  | B | Proposal/tasks/verification are complete, honest, and reflect the round-3 design; the lesson round 2 asked for (the append-vs-publish fact, and Windows rename semantics) is still not written, and no follow-up ticket is named for the orphan paths. |

### Verdict

**PASS** — no Blocker and no REAL Major. The one Blocker from round 2 is remediated in code and I verified the remediation end to end with six racing OS processes rather than by reading the author's assertion; every remaining finding is Minor, and the two REAL ones are a missing regression guard and disk-housekeeping. The rubric is all B, so the aggregation rule and the `severity × reality` rule agree.

`dotf spec archive` is **advisable** in this state.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** by this verdict — do not edit it, or the archive's staleness check will refuse this review. Everything below belongs in code, tests, or `verification.md`, which is outside that check.

- **Tests** — replace the vacuous concurrency guard (F1): two goroutines publishing two claims of one day, asserting every `*.gz` decodes to its full source. This is the one gap that would let round 2's Blocker return unnoticed. Disposition in `verification.md`.
- **Code** — sweep `.partial` in `cleanup` (F2); give a `.log` that is not today's an age rule so a forward clock skew cannot pin it forever (F3); either add the per-process counter to `uniqueName` or say in a comment why pid+ns suffices (F5); mark `compressDay` as a test seam (F7); close the midnight straddle by re-checking the day after the write (F6).
- **`verification.md`** — record that a non-hardlink logs volume degrades to unique-name archives and therefore to a shorter retention window (F4), and that Windows remains compile-verified only.
- **Knowledge (repo `docs/lessons.md`)** — the durable fact this change turned on is that a gzip archive is published, never appended to, because two concurrent appenders corrupt every earlier member; and that Go's rename/link succeed past `FILE_SHARE_DELETE` on Windows. Round 2 asked for this and it is still absent (`docs/` is untouched in the range).
