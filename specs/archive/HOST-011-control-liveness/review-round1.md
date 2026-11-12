---
spec: "HOST-011-control-liveness"
verdict: "PASS"
reviewed_sha: "7ce4b456060d0f3fca152ac0f8639f1d0a7f1b25"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-11"
---

## Adversarial review

**Scope**: `HOST-011-control-liveness` — the whole branch, `git diff 639f6b4447f65055c64edf5153f433722475f2a2...HEAD`
(3 first-parent commits: `e22ced2` spec docs, `d308e1a` the fix, `7ce4b45` evidence + lesson).
**Sources**: `specs/HOST-011-control-liveness/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`; diff of `internal/serve/control.go`, `internal/serve/control_test.go`,
`cmd/cli/host_serve.go`, `scripts/e2e/{run.sh,mesh_linux.sh,mesh_health.sh}`,
`docs/lessons/lesson-058-2026-11-11.md`; PR #719 (OPEN, body carries `Closes #565`).

**Contract freshness**: `dotf spec digest HOST-011-control-liveness` returns exactly the four
digests recorded in `review-request.json` (`proposal.md b0e8899e…`, `tasks.md 8c036958…`,
`features.json f048a3ac…`, `spec.md ""`) — the review describes the contract the launcher froze.

### What I ran (fresh, in this session)

| Command | Result |
|---|---|
| `go build ./...` | exit 0 |
| `go test -race -count=1 ./...` | exit 0, no FAIL lines (only `cmd/ts-bridge [no test files]`) |
| `go test ./internal/serve/ -run 'TestWatchControlOnline\|TestHealth_FoldsControlOnline' -count=1 -v` | PASS, 9 subtests + folds test |
| `go test ./internal/serve/ -run 'TestServeHealth_ControlSession\|TestHealth_FoldsSources' -count=1` | PASS (AC3 unchanged) |
| `golangci-lint run ./internal/serve/... ./cmd/cli/...` (2.14.0 from `mise`) | `0 issues.` |
| `GOOS=windows` / `GOOS=darwin go vet ./internal/serve/ ./cmd/cli/` | clean |
| `bash scripts/tests/test-e2e-mesh-preflight.sh` | all 18 cases passed |
| `bash scripts/tests/test-shellcheck.sh` | OK |
| mutation: `offlineSamplesDown = 1` | `TestWatchControlOnline/two_offline_samples_do_not_flap` and `/an_online_sample_resets_the_count` FAIL (red-green verified, as `verification.md` claims) |
| mutation: delete the poller goroutine from `watchServeControl` | build OK, **full `./cmd/cli/ ./internal/serve/` suite still green** (see F1) |
| `gosec` | not installed on PATH here → **UNVERIFIED** (claimed clean in `verification.md`) |
| `make e2e-mesh-linux` | **UNVERIFIED** — needs `dotf vm` + colima + Docker; its log file is not in the repo |

Working tree left clean apart from this review (mutations reverted from backups; `git status`
shows only `review.md` + the launcher's `review-request.json`).

### Spec and task alignment

- **AC1 (503 within 40 s, 200 within 30 s of recovery; same reason with `bootstrap_ssh`)** — code
  matches: `WatchControlOnline` feeds the second source `control-online` and uses the existing
  `ReasonControlPlaneUnreachable` token; `TestHealth_FoldsControlOnline` folds it beside
  `bootstrapSource`. The end-to-end half is the e2e case, which I could not run (UNVERIFIED, F4).
- **AC2 (two offline samples keep readiness, the third drops it; a failed read counts as offline)** —
  matches `onlineDebounce.observe`; the eight table rows in `TestWatchControlOnline` cover exactly
  this, including `err != nil` and `Self == nil`. Mutant `offlineSamplesDown = 1` is red, as claimed.
- **AC3 (bus watcher unchanged)** — the diff only adds to `control.go`; `WatchControlSession` body is
  byte-identical to the base, and its two tests pass unchanged.
- **Out of scope respected**: no reconnect logic, no `host check` change, no `bootstrap_ssh` live run.
- **Tasks**: every implementation box is `[x]` with matching diff evidence. The two Closing boxes
  (`PR opened`, `review`) are still `[ ]` while PR #719 is open — see F6 (contract set, closed).
- **`features.json`**: f1–f3 re-run green at HEAD (their commands are the ones in the table above);
  f4 (`make e2e-mesh-linux`) is the only unverifiable entry.
- No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags in the spec folder.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | tests / wiring | No test covers the production wiring. `watchServeControl` is referenced by **no** test in `cmd/cli`; deleting the `ticker`+`WatchControlOnline` goroutine leaves the entire Go suite green, so a future refactor can silently un-fix #565 while CI stays green. `tasks.md` declares this trade-off ("covered by the mesh e2e, not a unit test"), and the same seam (the bus watcher wiring) was already untested before this change, so it is inherited pattern rather than a new defect. | mutation of `cmd/cli/host_serve.go` → `go build` OK, `go test ./cmd/cli/ ./internal/serve/` → `ok ok` | named but out-of-CI: e2e case `mesh outage: /health/ready reads control_plane_unreachable` (`scripts/e2e/run.sh`); **no Go test** | tests (a fake-node seam, or a `watchServeControl` variant taking the poller) — follow-up |
| Minor | REAL | tests / e2e harness | The new harness logic is not covered by any shim suite, against this repo's stated rule that e2e logic is "tested in CI against shims": `mesh_outage_case` (stop → assert → start → assert, rc folding) and `scripts/e2e/mesh_health.sh` (stdin contract, budget loop, the two `case` patterns, the exit-1 failure path) never execute under `scripts/tests/test-e2e-*.sh`. I ran the preflight shim suite: 18/18 pass without ever running `mesh_health.sh`, because the `dotf … --script` shim answers `PASS mesh enrol: shim`. | `bash scripts/tests/test-e2e-mesh-preflight.sh` → all passed, no `mesh_health.sh` execution | UNTESTED (the shim suite is the natural home) | tests (add `scripts/tests/test-e2e-mesh-health.sh` driving `mesh_health.sh` against a local HTTP fake, plus a case asserting `mesh_outage_case` restarts Headscale after a failed first assertion) |
| Minor | THEORETICAL | resilience | A **hung** status read does not fail closed. `nodeOnline` calls `status(ctx)` with the serve context, which carries no deadline; a wedged LocalAPI makes the watcher block instead of counting an offline sample, so the source stays at its last value ("up") and `/health/ready` keeps answering 200. `proposal.md` claims "a broken local API fails closed" — true for a *failed* read, not for a *hanging* one. | code read: `internal/serve/control.go` `WatchControlOnline`/`nodeOnline`; no per-read timeout, `ctx` is the serve ctx | UNTESTED | code (per-read `context.WithTimeout`, e.g. 5 s, counted as offline) — follow-up |
| Minor | THEORETICAL | timing | The e2e budgets are tight against the code's worst case. Down: ticks at 10/20/30 s + the measured ~3 s lag ≈ 33 s against a 40 s budget, and ticks coalesce (ticker buffer 1), so a slow `Status` call stretches the window past 40 s. Up: ~14 s reconnect + up to 10 s to the next tick ≈ 24 s against a 30 s budget. Measured 20 s / 19 s leaves 7 s and 6 s of margin. | code + measurement table in `verification.md` | named e2e case (out of CI) | tests/code (raise the e2e budgets or drop the interval to 5 s) — follow-up |
| Minor | REAL | verification | The AC1 end-to-end evidence is not reproducible from the repo: the named artifact `mesh-linux-20261111T080536Z.log` is not committed (and `.gitignore`d `scripts/e2e/out/`), and the VM run cannot happen in CI, so a later reader (or archiver) cannot re-derive the two PASS lines. The "red without the poller" claim also rests on the measurement table, not on the new case failing pre-change. | `git diff --stat` has no log file; `make e2e-mesh-linux` cannot run here | UNTESTED (manual run, not re-derivable) | verification.md (paste the two PASS lines and the log's commit) |
| Minor | SPECULATIVE | maintainability | `scripts/e2e/mesh_health.sh` hardcodes `127.0.0.1:9090`, duplicating the `health_addr` this change added to `mesh_linux.sh`. Change the port in one place and the outage case times out with a misleading FAIL. | `mesh_health.sh:9` vs `mesh_linux.sh:33` | UNTESTED | code (pass the addr on the script's stdin with the budget) — follow-up |
| Minor | REAL | spec drift | `tasks.md` Closing keeps `[ ] PR opened with Closes #565` unticked although PR #719 (OPEN) carries `Closes #565`, and `verification.md` names no PR. Contract set is closed under this verdict — record it in `verification.md`, do not edit `tasks.md`. | `gh pr list --head fix/issue-565-control-liveness` → #719 OPEN | n/a | verification.md |
| Question | — | scope | The `Why` leans on HOST-008 AC4/AC6 (`host check` still reporting `published`). I confirmed by code read that `cmd/cli/host_check_published.go:36` GETs `/health/ready` and treats non-200 as not published, so the claim holds and the out-of-scope declaration is sound — but no test pins that this 503 now reaches `host check`'s `published` field. | `host_check_published.go:36`; `host_check_published_test.go` absent from the diff | UNTESTED (end-to-end consequence of the fix, deliberately out of scope) | optional follow-up ticket |
| Question | — | assumptions | `Self.Online` is the control server's view relayed in the map response (`proposal.md` "Risks"). Only Headscale is exercised; a control server that stopped sending the field would read offline forever. Already stated by the author with its mitigation, so no action — recorded here so the archive carries it. | `proposal.md` Risks | — | — |

**No Blocker or REAL Major found.** The two new behaviours are unit-tested with negative paths, the
reused reason token keeps `/health/ready`'s contract, and the fold's "first down source, sorted"
behaviour makes the token identical whichever watcher fires.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC2/AC3 verified with named tests and a red-green mutant; AC1's end-to-end half is only covered by an out-of-CI e2e I could not run (UNVERIFIED, not failed). |
| Verification       | B | f1–f3 re-run green at HEAD with the exact `features.json` commands and a demonstrated mutant; f4's evidence is an uncommitted log, and `gosec` is unverifiable here. |
| Scope              | A | Diff maps 1:1 to the proposal (one watcher, its wiring, the e2e case + `health_addr`, the lesson); no creep, out-of-scope items untouched. |
| Reliability        | B | Failure and shutdown paths handled and tested (error read, `Self == nil`, closed ticks, ctx cancel, one report per change); a hung status read is not bounded (F3). |
| Maintainability    | A | Functions ≤40 lines, trivial CC, `onlineDebounce` isolated and table-tested, comments explain **why** the bus cannot see the outage; only the duplicated port constant (F5) is a smell. |
| Handoff-readiness  | A | Spec triad complete, `verification.md` carries the measurement and decisions, lesson 058 written, PR body has the `## Knowledge` section naming lesson/ADR/runbook dispositions. |

### Verdict
PASS

### Archive advisability

`dotf spec archive HOST-011-control-liveness` is **advisable in the current state**: this review is
fresh against the frozen contract digests, there is no Blocker and no REAL Major, the rubric has no
C and no D, and nothing in the spec folder blocks the archive. The findings are tracked gaps, not
conditions.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** under this verdict — do
not edit those three before archiving, or the review goes stale. Disposition each item below in
`verification.md` (applied / ticketed / declined with a reason), or carry it into a follow-up ticket:

- **F1 (tests)** — decide: accept the declared e2e-only cover for `watchServeControl`, or add a seam
  test that fails when the poller goroutine is removed (mutation shown in this review). Ticket if deferred.
- **F2 (tests)** — add `scripts/tests/test-e2e-mesh-health.sh` covering `mesh_health.sh`'s stdin
  contract, both match patterns, the budget-exhaustion exit 1, and `mesh_outage_case`'s restart on a
  failed first assertion; that is the repo's own rule for e2e logic.
- **F3 (code)** — bound the per-read status call so a wedged LocalAPI fails closed instead of
  freezing the source at "up"; either implement in a follow-up ticket or decline with the rationale.
- **F4 (verification.md)** — paste the two e2e PASS lines with the log's date/commit so AC1's
  end-to-end evidence is readable without a VM, and name PR #719 (F6).
- **F5 (code, follow-up)** — pass the health address to `mesh_health.sh` rather than hardcoding it.
- **F6 (verification.md)** — note that the `tasks.md` "PR opened" box is stale and why it is not
  being edited.
- **F7 (ticket, optional)** — pin that a 503 `control_plane_unreachable` reaches `host check`'s
  `published` field, the consequence the `Why` describes.
