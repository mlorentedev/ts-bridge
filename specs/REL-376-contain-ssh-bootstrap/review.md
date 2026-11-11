---
spec: "REL-376-contain-ssh-bootstrap"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "f5e27cd691a7d3e47733f3f1f6d4e3ba862bf3fe"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-09"
---

## Adversarial review

**Scope**: REL-376-contain-ssh-bootstrap (round 2). The spec's own commits in the range are
`a6bfe81` (`fix(bootstrapssh): tie the ssh child to the parent's lifetime (#597)`) and `f5e27cd`
(`test(bootstrapssh): guard the containment hooks Start passes to start`, the round-1 fix). Other
commits inside `e5adf4e...HEAD` belong to other specs (`2d868df` #635, `3090051` #669, `8ad8da1`
#676); they are judged only where they interact with containment.

**Sources**: `specs/REL-376-contain-ssh-bootstrap/{proposal,tasks,verification}.md`,
`review-request.json` (contract digests re-checked with `dotf spec digest`: match), diff
`e5adf4edaeb9bae7bb10d99bfc4be94aa9db6fa7...HEAD`, files `internal/bootstrapssh/{bootstrap,contain,
contain_linux,contain_windows,contain_other}.go`, `contain*_test.go`, `wiring_test.go`,
`docs/runbooks/guide-control-bootstrap.md`.

**Independent runs in this session** (host darwin/arm64, Go 1.27.3):
`go build ./...` ok; `go test -race ./internal/bootstrapssh/` ok; `gofmt -l` clean; `go vet` ok for
darwin, `GOOS=linux go vet`, `GOOS=windows go vet` and `GOOS=windows go test -c` ok;
`golangci-lint run ./internal/bootstrapssh/...` 0 issues (darwin and `GOOS=windows`);
`gosec ./internal/bootstrapssh/...` 0 issues; Linux package under `docker run golang:1.27`
`go test -race` PASS; `scripts/check-features-selectors.sh` OK (50 selectors).

### Spec and task alignment

- `proposal.md` AC1–AC4 each map to a task box and to diff evidence. All `tasks.md` boxes are `[x]`
  and none is unticked-without-evidence.
- AC1 (abrupt kill ends the child): `TestContainment_KillsChildWhenParentDies`
  (`//go:build linux || windows`, no skip, `Fatalf` on a containment refusal). Reproduced on Linux:
  PASS under `golang:1.27` with `-race`. Windows: corroborated from the CI run the disposition cites
  (`gh run view 41887607029`: `test-windows` success, `ok ts-bridge/internal/bootstrapssh 1.119s`,
  and the test file builds on windows, so the pass covers it).
- AC2 (a failing containment does not fail `Start`): `TestStartSurvivesContainmentFailure` PASS.
- AC3 (containment applied to the started process, released only after `Wait`):
  `TestStartContainsTheStartedProcessAndReleasesAfterWait` PASS, and it is sensitive — closing the
  job before `Wait` makes the package FAIL (mutation run this session).
- AC4 (runbook per-OS boundary): the "An abrupt exit is handled per OS" section covers Linux,
  Windows (join window, `ProxyCommand`, job nesting) and macOS. One nuance, finding 6.
- The round-1 FAIL was dispositioned in `verification.md`. Its fix (`TestStartWiresContainment`,
  `defaultDependencies`) is present; the red-green evidence it claims reproduces (nil hooks fail
  both assertions). That fix closes *identity* of the hooks but not *invocation or ordering* —
  finding 1.
- `features.json` states are `pending`/empty like every active spec in the tree (repo convention,
  not a gap); its selectors still name live tests (selector guard OK).

**Verified strength that mitigates a documented risk**: the Linux real-kill test is not a
tautology. Removing `Pdeathsig` from `contain_linux.go` makes it FAIL with
`child 4543 outlived its killed parent` after 5s — reproduced this session in `golang:1.27`. That
is the red-green evidence the tasks claim, independently confirmed.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | THEORETICAL | test coverage / wiring | No named test proves `start` **calls** `prepare`, nor that it calls it **before** `cmd.Start()`. Two mutations leave the whole package suite green: (a) `deps.prepare(cmd)` moved after `cmd.Start()`, (b) the `prepare` call removed entirely. Verified on darwin and, for (b), in `golang:1.27`. On Linux both silently drop AC1's mechanism (a post-`Start` `SysProcAttr` write has no effect) while every test, the lint gate and the real-kill test stay green — the real-kill test calls `prepareContainment` directly and never goes through `start`. Round-1's fix pinned the hooks' identity, not their use. | mutation runs this session: `go test ./internal/bootstrapssh/` ok for both mutants (darwin); mutant (b) also ok on linux in docker | UNTESTED — `TestStartWiresContainment` compares function pointers only; `TestStartContainsTheStartedProcessAndReleasesAfterWait` covers `contain`, not `prepare` | tests (add a seam test that records `prepare` and asserts `cmd.Process == nil` when it runs, mirroring the existing `contain` assertion; e.g. `TestStartAppliesPrepareBeforeStart`) |
| Minor | REAL | quality / length | `start` is 44 lines, over the documented 40-line limit (AGENTS.md table, `.golangci.yml funlen.lines: 40`), and passes only because `//nolint:funlen // baseline #629` was added by the unrelated baseline commit (`8ad8da1`, #676) over a function this change pushed from 40 lines at the base to 44. Removing the marker: `golangci-lint run ./internal/bootstrapssh/...` → `Function 'start' is too long (44 > 40)`; `go test ./cmd/cli -run TestComplexityBaselineOnlyShrinks` → `funlen exemptions = 21, ceiling 22: lower baselineCeiling to match`. The baseline marker is documented as tagging a function that predates the limits and as "split the function instead of exempting it". | measured: `start` = 40 lines at `e5adf4e`, 44 (`funlen` count) at HEAD; both commands run this session | `TestComplexityBaselineOnlyShrinks` (guards the ceiling mechanism, not the 40-line limit itself) | code (extract the spawn-and-contain step from `start`) + tests (shrink `baselineCeiling["funlen"]` to 21) |
| Minor | REAL | verification artifact | `verification.md` contradicts itself about the Windows evidence: the AC1 bullet still says "Windows: `test-windows` CI only; the run is not recorded here yet", while the later round-1 disposition records run `41887607029`. I confirmed the run: `test-windows` succeeded and its log carries `ok ts-bridge/internal/bootstrapssh 1.119s`; the kill test builds on windows and has no skip, so the package pass is the named evidence. The stale bullet makes the artifact read as unverified on the platform that has no local coverage. | `gh run view 41887607029 --log` (job `test-windows`, run conclusion success) | the CI run itself covers `TestContainment_KillsChildWhenParentDies`; the artifact, not the test, is wrong | verification.md (outside the contract set) |
| Minor | THEORETICAL | test integrity | Windows `processGone` returns `true` ("gone") for **any** `OpenProcess` error, including access denied. A permission error would satisfy the kill assertion without observing the child at all, and could also hide a genuine regression in the opposite direction (a real child that the test cannot open). The Linux sibling treats `ReadFile` failure the same way, but there the error class (`ENOENT`) is the signal being tested. | code read: `contain_kill_windows_test.go:8-11` | UNTESTED (no test of `processGone` itself) | tests (fail on an unexpected `OpenProcess` error instead of returning "gone") |
| Minor | REAL | security-scan coverage | `contain_windows.go` — the only file in this change using `unsafe.Pointer` and an `uint32` pid narrowing — is never scanned: `gosec ./internal/bootstrapssh/...` on the hosted OS reports `Files: 5` and skips it (build tag `windows`), CI's `security` job is `ubuntu-latest`, and `test-windows` runs only build/vet/test. The `#nosec G103` / `#nosec G115` comments are therefore never exercised by a scanner; their correctness rests on the author's judgment. Repo-wide gap (every `*_windows.go`), not created by this change. | `gosec` output (`Files: 5`, `Issues: 0`), `.github/workflows/ci.yml` `security` and `test-windows` jobs | none — no scanner currently covers the file | CI (ticket; a windows gosec pass over `*_windows.go`) — not this spec |
| Minor | THEORETICAL | docs (AC4) | The runbook states "Linux: the kernel sends `SIGKILL` to `ssh` when ts-bridge dies", while the code comment correctly caveats that Pdeathsig fires on the **forking thread's** death (Go's own `SysProcAttr` doc: the signal "may happen before process termination", go.dev/issue/27505). AC4 asks the runbook to document what each OS covers and its boundary; this boundary is documented in code only. Practically accurate today (no `LockOSThread` goroutine in the bootstrap), but the operator-facing artifact overstates it. | runbook section vs `contain_linux.go` comment vs `$GOROOT/src/syscall/exec_linux.go:92-96` | UNTESTED (documentation) | docs (runbook; outside the contract set) |

**Carried from round 1, accepted — not re-opened**: a containment refusal is dropped silently
(Minor, THEORETICAL at the time). AC2's text asks exactly that `Start` proceed; the runbook states
containment is best effort, and `TestStartSurvivesContainmentFailure` pins the behaviour. Recorded
as declined with a reason, which is a legitimate disposition. The "evidence not recorded" Minor is
applied and I independently reproduced it.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Every AC verified: Linux kill reproduced, Windows corroborated from CI, seam tests sensitive to mutation; the one missing negative path is the unguarded `prepare` wiring (finding 1). |
| Verification       | B | Reproducible commands and outputs exist and I reproduced the two central ones; one stale, self-contradicting Windows bullet (finding 3). |
| Scope              | A | Diff matches the proposal (three `contain_*` files, seam + real-kill tests, runbook section, wiring fix); unrelated commits in the range belong to other specs. |
| Reliability        | B | Error paths handled: refusal dropped by design, job closed after `Wait`, startup-failure path cancels and waits, job created before the `Wait` goroutine; residual Linux thread caveat and the untested prepare ordering. |
| Maintainability    | B | Clear per-OS files, comments explain why; `start` exceeds the 40-line limit and is exempted rather than split (finding 2); `contain.go`'s block comment attaches to `nopCloser` rather than being package-level (cosmetic). |
| Handoff-readiness  | B | Promotion candidates answered (no lesson/ADR/pattern), runbook and verification updated, next steps clear; the Windows evidence bullet is stale. |

Aggregation: no D; no C; all B or above → does not force a downgrade. The single open Major is
THEORETICAL → **PASS WITH GAPS** per the severity × reality rule.

### Verdict
PASS WITH GAPS

No Blocker: the shipped Linux and Windows containment is correct as written, the Linux mechanism was
reproduced, and the supply of a wrong `Pdeathsig`/job setting is guarded by tests that fail when the
setting is removed. The failure mode found is a **guard** gap: nothing keeps `start` from applying —
or ordering — `prepare` wrongly, and the suite cannot see it.

### Recommended next steps

Routing by set: the verdict is not FAIL, so `proposal.md`, `tasks.md` and `features.json` are
**closed** — an edit to any of them invalidates this review. Disposition each item in
`verification.md` (applied / ticketed / declined with reason), which the staleness check excludes.

- **Tests** (outside contract set) — finding 1: add `TestStartAppliesPrepareBeforeStart`, mirroring
  `TestStartContainsTheStartedProcessAndReleasesAfterWait`'s `cmd.Process != nil` assertion; with it,
  both surviving mutants die. Finding 4: make Windows `processGone` fail on an unexpected
  `OpenProcess` error.
- **Code** (outside contract set) — finding 2: extract `start`'s spawn-and-contain block into a
  helper, drop the `//nolint:funlen // baseline #629`, and lower `baselineCeiling["funlen"]` to 21 in
  `cmd/cli/lint_baseline_test.go` (the shrinking direction the baseline test demands).
- **Docs** — finding 6: one sentence in the runbook's Linux bullet, carrying the same Pdeathsig
  thread caveat the code comment has. Finding 3: delete the stale "not recorded here yet" line and
  cite run `41887607029`.
- **CI/ticket, not this spec** — finding 5 is repo-wide (`*_windows.go` files fall outside the
  `ubuntu-latest` gosec job). File it against the `security` job rather than widening this change.
- **Archive advisability**: `dotf spec archive` will accept this PASS-WITH-GAPS (fresh contract
  digests verified, `review-request.json` matching). Advisable once the four items above are
  dispositioned in `verification.md`; no re-review is required for them, since none is a REAL Major
  or a contract-set change.

**Minimum to flip this to a clean PASS**: the tests item of finding 1 (a named test proving
`prepare` runs before `Start`), then re-review.
