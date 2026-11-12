---
spec: "HOST-011-control-liveness"
verdict: "PASS"
reviewed_sha: "32b10fb6d175c3b14bbe8bd690530deb2a889be7"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-11"
---

## Adversarial review

**Scope**: `HOST-011-control-liveness` — the whole branch, `git diff 639f6b4447f65055c64edf5153f433722475f2a2...HEAD`
(7 first-parent commits: `e22ced2` spec docs, `d308e1a` the fix, `7ce4b45`/`765630f` evidence + lesson,
`23221b8` bounded read + poll-without-bus, `67da1c1` e2e shim cases, `32b10fb` round-1 review kept), plus
`80f1282`, a merge of `origin/master` that brought `5f76145` (Dependabot → Renovate) and `3ca9518`
(archive SEC-631/QA-606/REL-376). Those two are **not this spec's work**; they touch no file this
spec changes except the spec folders' neighbours, so they are judged only as scope noise (F7).
**Sources**: `specs/HOST-011-control-liveness/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`, `review-round1.md`; diff of `internal/serve/control.go`,
`internal/serve/control_test.go`, `cmd/cli/host_serve.go`, `cmd/cli/host_serve_control_test.go`,
`scripts/e2e/{run.sh,mesh_linux.sh,mesh_health.sh}`, `scripts/tests/test-e2e-probe.sh`,
`docs/lessons/lesson-058-2026-11-11.md`.

**Contract freshness (checked, not assumed)**: `dotf spec digest HOST-011-control-liveness` returns
exactly the four digests in `review-request.json` — `proposal.md b0e8899e…`, `tasks.md 8c036958…`,
`features.json f048a3ac…`, `spec.md ""` (empty). The contract this review describes is the one the
launcher froze. I did not edit any contract file.

**Round-1 interaction**: round 1 (`7ce4b45`) returned PASS with minors F1 (`watchServeControl`
wiring untested), F2 (`mesh_health.sh`/`mesh_outage_case` untested), F3 (unbounded status read),
F5 (hardcoded endpoint), F6 (PR unnamed). `23221b8` and `67da1c1` add code and tests that answer
F2/F3/F5 and one hop of F1. I re-tested those same mutations against HEAD instead of trusting the
commit messages, and each new test is genuinely red when its property is removed (table below).

### What I ran (fresh, in this session, at `32b10fb`)

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -race -count=1 ./...` | exit 0; every package `ok`, no `FAIL` line (`cmd/ts-bridge [no test files]`) |
| `go test ./internal/serve/ -run 'TestWatchControlOnline' -count=1 -v` (f1/f2 command) | exit 0; 8 subtests + `TestWatchControlOnline_BoundsEachRead` PASS |
| `go test ./internal/serve/ -run 'TestWatchControlOnline\|TestHealth_FoldsControlOnline' -count=1` (f2) | exit 0 |
| `go test ./internal/serve/ -run 'TestServeHealth_ControlSession\|TestHealth_FoldsSources' -count=1` (f3, AC3) | exit 0 |
| `golangci-lint run ./internal/serve/... ./cmd/cli/...` (2.14.0, `mise`) | `0 issues.` |
| `gofmt -l .` | empty |
| `GOOS=linux` / `GOOS=windows go vet ./internal/serve/ ./cmd/cli/` | clean |
| `bash scripts/tests/test-e2e-probe.sh` | 14 cases `ok`, incl. the 5 new `health-*` cases |
| mutation `offlineSamplesDown = 3 → 1` | FAIL `TestWatchControlOnline/two_offline_samples_do_not_flap` + `/an_online_sample_resets_the_count` (AC2 is pinned) |
| mutation: delete `nodeOnline`'s `context.WithTimeout` | FAIL `TestWatchControlOnline_BoundsEachRead` — **round-1 F3 is fixed and pinned** |
| mutation: delete the ticker+`WatchControlOnline` block from `watchControl` | FAIL `TestWatchControl_PollsSelfOnline` — **round-1 F1's inner hop is fixed and pinned** |
| mutation: `watchServeControl` calls nothing (`_ = lc; _ = err`) | build OK, `go test ./cmd/cli/ ./internal/serve/` **still green** — F1's outer hop is still unpinned |
| mutation: `mesh_health.sh` case `503:503*control_plane_unreachable*` → `503:503*` | FAIL `health-503-other-reason` — **round-1 F2's probe half is fixed and pinned** |
| `gosec` | not installed on PATH → **UNVERIFIED** |
| `make e2e-mesh-linux` | **UNVERIFIED** — needs `dotf vm` + colima + Docker; `scripts/e2e/out/` is gitignored (`.gitignore:76`) so the named log is not in the repo |

Working tree left clean apart from `specs/HOST-011-control-liveness/review.md` and the launcher's
`review-request.json` + round-1 transcript files (`git status --porcelain`); every mutation was
reverted from a `mktemp -d` backup and re-verified clean.

### Spec and task alignment

- **AC1 (503 with `control_plane_unreachable` within 40 s, 200 within 30 s of recovery, same reason
  with `bootstrap_ssh`)** — code matches: `WatchControlOnline` drives the second source
  `control-online` with the existing `ReasonControlPlaneUnreachable`, and
  `TestHealth_FoldsControlOnline` folds it beside `bootstrapSource` (`internal/serve/control_test.go`).
  Sorting in `Health.store` makes `control` and `control-online` yield the **same** token whichever
  fires. The live half is the e2e case, which I could not run (UNVERIFIED, F4).
- **AC2 (two offline samples hold readiness, the third drops it; a failed read counts as offline)**
  — matches `onlineDebounce.observe`; the eight table rows cover `err != nil`, `Self == nil`, the
  reset, and "down once, not per sample". Mutant `offlineSamplesDown = 1` is red.
- **AC3 (bus watcher unchanged)** — `WatchControlSession`'s body is untouched; its two tests pass
  unchanged. One signature did change: `NodeStates(ctx, lc *local.Client)` → `NodeStates(ctx, lc
  BusWatcher)` (F6).
- **Out of scope respected** — no reconnect logic, no `host check` change, no live `bootstrap_ssh` run.
- **Tasks** — every `[x]` maps to diff evidence. `tasks.md`'s `[ ] PR opened with Closes #565` is still
  unticked while `verification.md` now carries a `## Pull request` section answering it (round-1 F6);
  the contract set is closed under this verdict, so record it in `verification.md`, never by editing
  `tasks.md`.
- **`features.json`** — f1–f3 re-run green at HEAD with their exact commands; f4 (`make e2e-mesh-linux`)
  is the only entry I cannot execute. All four carry non-empty `evidence`; none was hand-set to
  `passing` by me.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` markers in the spec files (the only grep hit is
  `review-round1.md` *quoting* the phrase).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | tests / wiring | F1 (continues round-1 F1): the **outer** hop is still unpinned. `cmd/cli/host_serve.go:228` is the only call to `watchServeControl`, and no test references it; replacing its body with `_ = lc; _ = err` leaves the entire Go suite green. The new `TestWatchControl_PollsSelfOnline` pins one level down (deleting the poller inside `watchControl` is now red), so the change is a real improvement, but `host serve` can still be silently un-fixed one line above that test's reach. | mutation of `watchServeControl` → `go build ./...` OK, `go test ./cmd/cli/ ./internal/serve/ -count=1` → `ok ok` | UNTESTED for the outer hop (`TestWatchControl_PollsSelfOnline` covers `watchControl` only) | tests (a `watchServeControl` seam, e.g. a variant taking `controlClient`, `ready` and a logger) — ticket if deferred |
| Minor | REAL | tests / e2e harness | F2 (continues round-1 F2, half fixed): `mesh_health.sh` is now covered in CI and red-green verified, but `mesh_outage_case` (`scripts/e2e/run.sh:133-149`) still has no shim case: the stop → assert → start → assert folding, `rc` accumulation across both assertions, and the `docker start … \|\| return 1` short-circuit never execute under `scripts/tests/test-e2e-*.sh`. A case that fails its first assertion and still restarts Headscale (so the run leaves the control plane usable) is untested. | `grep -rn 'mesh_outage' scripts/tests/*.sh` → no hit; `repo-hygiene.yml:87-96` runs the five shim suites, none of them this function | UNTESTED (`test-e2e-mesh-preflight.sh` is the natural home) | tests |
| Minor | REAL | verification artifact | F3: `verification.md` describes the implementation at `d308e1a`, not HEAD. The AC evidence rows do not mention `TestWatchControlOnline_BoundsEachRead` (the read bound), `TestWatchControl_PollsSelfOnline` (poll despite a broken bus), or `scripts/tests/test-e2e-probe.sh` (the probe's fake-endpoint coverage) — the three things that actually answer round-1 F1/F2/F3. `features.json` f1–f3 evidence also cites `d308e1a` (their commands still pass at HEAD, so no false claim; the sha is merely stale). | `verification.md` "### 2026-11-11: implementation at `d308e1a`" section; no later test-status entry | n/a (artifact) | spec (verification.md — outside the contract set, editable without invalidating this review) |
| Minor | THEORETICAL | timing / e2e budget | F4: AC1's 40 s budget is close to the implementation's worst case. Ticks are 10 s apart and coalesce (buffer 1); a bounded read can take up to 5 s, so the third consecutive offline sample can land near 3×10 s + 5 s ≈ 35 s, plus the measured ~3 s lag and the probe's ~2 s granularity ≈ 40 s — the budget itself. Measured 20 s leaves margin, so this is a flake risk on a slow guest LocalAPI, not an observed failure. | `controlPollInterval = 10s`, `controlReadTimeout = 5s` (`internal/serve/control.go`), budget `40` (`run.sh:136`), measurement table in `verification.md` | named e2e case (out of CI) | code/tests (`40` → `50` s, or poll every 5 s) — follow-up |
| Minor | SPECULATIVE | maintainability | F5 (rounds down round-1 F5): `mesh_health.sh` still defaults to `http://127.0.0.1:9090/health/ready`, a second copy of `health_addr: 127.0.0.1:9090` in `mesh_linux.sh:33`. `HEALTH_URL` now overrides it, so the test path is safe; only the e2e default can drift and then time out with a misleading message. | `mesh_health.sh:10` vs `mesh_linux.sh:33` | named: `health-200` etc. cover the override, not the default | code — follow-up |
| Minor | THEORETICAL | API surface | F6: `BusWatcher` (`internal/serve/control.go:66-69`) is exported only so `cmd/cli`'s `controlClient` can satisfy `NodeStates`; nothing outside the two packages uses the name, and the widening is source-compatible but a permanent public surface for a testability seam. `controlPollInterval` (a package-level var mutated by `TestWatchControl_PollsSelfOnline`) forecloses `t.Parallel` in `cmd/cli` — today that package has none, so it is latent, not active. | `grep -rn 'BusWatcher'` → declaration + `NodeStates` signature only; `grep -rn 't.Parallel()' cmd/cli/*.go` → none | n/a | code (unexport, or document why exported) — follow-up |
| Question | — | assumptions | F8 (carried from round 1): `Self.Online` is the control server's view relayed in the map response. Only Headscale is exercised; a control plane that stopped relaying the field would read offline forever and flap readiness. The author states this in `proposal.md` "Risks" with its mitigation, so it is recorded, not a defect. | `proposal.md` Risks | — | — |
| Question | — | scope | F7: the range carries `80f1282` (merge of `origin/master`), which brings `5f76145` Dependabot → Renovate (`.github/dependabot.yml` deleted, `renovate.json` added) and `3ca9518` (archiving SEC-631/QA-606/REL-376). Neither interacts with this spec's code, tests or docs; the spec's own files are untouched by the merge. Judged as unrelated work that arrived through a merge, not as creep. | `git log --merges 639f6b4..HEAD` → `80f1282`; its two side commits | n/a | — |

**No Blocker and no REAL Major.** The two behaviours under review are unit-tested with negative paths,
the reused `control_plane_unreachable` token keeps `/health/ready`'s contract, the fold makes the token
identical whichever watcher fires, and three separate properties (debounce count, read bound, poller
start) each go red under a targeted mutation. The gaps above are coverage and artifact hygiene, not
observed misbehaviour — and the two that round 1 named as code risks (unbounded read, poll skipped on a
broken bus) are genuinely closed.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC2/AC3 verified with named tests plus red-green mutants; AC1's end-to-end half rests on an e2e I could not run (UNVERIFIED, not failed), and the wiring's outer hop is unpinned (F1). |
| Verification       | B | The `features.json` commands re-run green at HEAD and three mutations are red; the residual gap is that AC1's live evidence is a gitignored log and `verification.md` still describes `d308e1a` (F3) — reproducible-with-context, not anecdotal. |
| Scope              | A | The spec's diff maps 1:1 to the proposal (one watcher, its wiring, the e2e case + `health_addr`, the lesson, the probe cases); out-of-scope items untouched; the Renovate/archive work is a merge, not creep (F7). |
| Reliability        | B | Failure and shutdown paths are handled and tested (error read, `Self == nil`, closed ticks, ctx cancel, one report per change) and the read is now bounded; the e2e budget margin is thin (F4). |
| Maintainability    | B | Functions ≤40 lines, trivial CC, `onlineDebounce` isolated and table-tested, comments explain why the bus cannot see the outage; the exported `BusWatcher` and the package-level interval var (F6) and the duplicated default port (F5) are the smells. |
| Handoff-readiness  | B | Lesson 058 + index entry landed, PR section added to `verification.md`, next steps clear; the verification artifact lags the last two commits (F3). |

### Verdict
PASS

`severity × reality`: no Blocker, no REAL Major; every open finding is a Minor of REAL/THEORETICAL/
SPECULATIVE reality, and the rubric has no C and no D. A SPECULATIVE finding (F5) cannot move this
below PASS, and the REAL Minors are tracked rather than blocking.

### Archive advisability

`dotf spec archive HOST-011-control-liveness` is **advisable in the current state**: this review is
fresh against the frozen contract digests, the code builds and tests green at HEAD with three
mutations red, and nothing in the contract set stands between the change and the archive. Note what I
could not check: whether the out-of-repo gate treats this verdict as passing (it is `PASS`, the same
verdict round 1 gave) and whether the freshness window it applies reaches this run — both **UNVERIFIED**.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** under this verdict: do not
edit those three before archiving, or this review goes stale and a third round is needed.
`verification.md` is outside the staleness check — disposition each item there (applied / ticketed /
declined with a reason), or carry it into a follow-up ticket:

- **F3 (verification.md, do this one — it is cheap and it is the artifact the archiver reads)** — add a
  test-status entry for `23221b8`/`67da1c1` naming `TestWatchControlOnline_BoundsEachRead`,
  `TestWatchControl_PollsSelfOnline`, and `scripts/tests/test-e2e-probe.sh`'s five `health-*` cases,
  and record the round-1 dispositions (F1 inner hop applied / outer hop deferred, F2 probe half
  applied, F3 applied, F5 partly applied).
- **F1 (tests)** — add a `watchServeControl` seam test that fails when line 228's call is removed;
  ticket it if deferred.
- **F2 (tests)** — add a shim case for `mesh_outage_case` in `scripts/tests/test-e2e-mesh-preflight.sh`
  (stub `docker` and `dotf`), asserting the second assertion still runs and Headscale is restarted
  when the first one fails.
- **F4 (code/tests)** — widen the outage budget from 40 s to ~50 s (or poll every 5 s) so a slow guest
  LocalAPI cannot turn a correct fix into a red e2e.
- **F5/F6 (code, follow-up)** — pass the health address to `mesh_health.sh` on stdin instead of
  defaulting to a literal, and either unexport `BusWatcher` or state in a comment why it is public.
- **F8/F7** — no action; both are recorded so the archive carries them.
