---
spec: "REL-376-contain-ssh-bootstrap"
verdict: "FAIL"
reviewed_sha: "55cfd68ba8ef0f91d8ecfc12669895e1498e216c"
reviewer: "nan/mimo-v2.6-flash"
date: "2026-11-09"
---

## Adversarial review

**Scope**: REL-376-contain-ssh-bootstrap (spec work commit `a6bfe81`, whose parent is exactly the
stated base `e5adf4edaeb9bae7bb10d99bfc4be94aa9db6fa7`; the range also carries 60 later master
commits — judged only where they interact with the spec's area)
**Sources**: `specs/REL-376-contain-ssh-bootstrap/{proposal,tasks,verification,features}.md(json)`,
`git diff e5adf4edaeb9bae7bb10d99bfc4be94aa9db6fa7...HEAD`,
`internal/bootstrapssh/*`, `docs/runbooks/guide-control-bootstrap.md`, PR #597 CI logs

### Spec and task alignment

- **Contract freshness**: `dotf spec digest REL-376-contain-ssh-bootstrap` matches
  `review-request.json` `contract_digests` exactly (features.json `07dac2d8…`,
  proposal.md `2735801f…`, tasks.md `0b9417c9…`). No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags in
  the spec files (the only grep hits are inside the launcher's own `review-transcript.jsonl`).
  `nan/mimo-v2.6-flash` is a member of `harness/reviewer-pool.json`.
- **AC1 (kill parent → child dies ≤5s, Linux + Windows)** — Linux: **verified red-green this
  session**. Baseline in Docker `golang:1.27`:
  `go test -run 'TestContainment_KillsChildWhenParentDies|TestPrepareContainment_SetsPdeathsig'
  ./internal/bootstrapssh/` → PASS. Mutation (drop `Pdeathsig`):
  `TestContainment_KillsChildWhenParentDies` FAIL `child 4595 outlived its killed parent` (5.02s),
  `TestPrepareContainment_SetsPdeathsig` FAIL; revert → green. Windows: `gh pr checks 597` shows
  `test-windows pass`, job log contains `ok ts-bridge/internal/bootstrapssh 1.119s`; the
  windows-tagged test has no skip path (only `t.Fatalf`), so a green package means it ran and
  passed on a real Windows runner. No local Windows host → the Windows leg is CI-only (as the
  proposal declares).
- **AC2 (containment refusal never fails Start)** — `TestStartSurvivesContainmentFailure` green;
  mutation (make `start` return the containment error) → FAIL
  `contain_test.go:49: start() with failing containment = access denied, want a tunnel`; revert →
  green. Red-green verified.
- **AC3 (applied after start, released only after wait)** —
  `TestStartContainsTheStartedProcessAndReleasesAfterWait` green; mutation (close the handle before
  `Wait` in the goroutine) → FAIL `contain_test.go:79: containment was not released after the
  process has been waited for`; revert → green. Red-green verified. **But** the two lines that wire
  this into the shipped entry point `Start()` are covered by no test — see the Major below.
- **AC4 (runbook per-OS boundary)** — met: `guide-control-bootstrap.md` documents Linux SIGKILL,
  Windows kill-on-close job incl. the start→assignment window, `ProxyCommand` boundary and
  nesting refusal, macOS's no-equivalent boundary with an `lsof` recovery command, and the
  best-effort statement.
- **Tasks**: every `[x]` has diff evidence (files/tests/runbook exist; lint/gosec/`-race` claims
  reproduced: `golangci-lint run` 0 issues, `gosec -quiet ./internal/bootstrapssh/...` exit 0,
  `go test -race -count=1 ./...` exit 0 on this host, Docker package run ok, mutation reproduced).
  The one `[ ]` (`test-windows CI log…`) has evidence that exists but is unrecorded — Minor 3.
- **Range interactions**: later commits touching the package were judged — #635 (adds
  `withoutSecrets` before prepare/contain; no interaction), #669 (SOCKS dial logging + runbook
  section; no containment interaction), #676 (funlen/gocyclo baseline comments only), #662
  (`run.go` tunnel Close paths — `Tunnel.Close` is idempotent, job close stays single-caller in the
  Wait goroutine). None weaken containment. `features.json` selectors all match real tests
  (`scripts/check-features-selectors.sh` → `OK (50 go test -run selectors)`).
- `features.json` `state: pending` / empty evidence matches the archived precedent
  (HOST-005 archives in the same state); the archive's three documented pre-flights (drafts,
  passing review, promotion answers) are satisfied by the spec files themselves.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|----------|---------|------|---------|----------|---------------------------|--------------|
| Major | REAL | tests / regression-guard | The shipped entry point's containment wiring (`Start()` → `prepare: prepareContainment, contain: containProcess`) is guarded by no test: it can be deleted and nothing turns red — the exact silent recurrence of #376 | Mutation this session: set both fields to `nil` in `Start` → `go test -race -count=1 ./internal/bootstrapssh/` still `ok 3.325s`, and the containment selectors stay green (the kill test calls `prepareContainment`/`containProcess` directly; the seam tests inject fakes into `start`). Grep: the only production callers are `cmd/cli/run.go:79` and `cmd/cli/host_serve.go:171`; every test replaces `sshTunnelStarter`; `smoke.bats` has no `--bootstrap-ssh` case; no e2e touches bootstrap. features.json f3's selector runs `TestStartContains…`, which never reads `Start`'s table | **UNTESTED** — no named test reaches `Start`'s dependency table | **tests** (e.g. a source-inspection guard over `bootstrap.go`, precedent: `internal/layout_test.go`, #673 — outside the contract set, then re-review) |
| Minor | THEORETICAL | resilience / observability | A containment refusal is dropped silently: `containStarted` discards the error, so on a Windows host that refuses job assignment the operator never learns `ssh` is uncontained. The runbook documents the boundary; the runtime reports nothing (contrast #669, which logs every dial) | `bootstrap.go`: `if err != nil \|\| job == nil { return nopCloser{} }`. Refusal never observed on tested runners (PR #597 `test-windows` green) | `TestStartSurvivesContainmentFailure` proves Start proceeds (desired) but asserts no log — log-side **UNTESTED** | code (warn-log the dropped error; does not stale this review) |
| Minor | REAL | spec record | Evidence exists but is not recorded: all four `verification.md` AC boxes are `[ ]` (AC1 even says Windows "not recorded here yet") and `tasks.md`'s closing box is `[ ]`, while the archived precedent (HOST-005) carries `[x]` verification boxes. The missing Windows evidence exists: `gh pr checks 597` → `test-windows pass`, log `ok ts-bridge/internal/bootstrapssh 1.119s` | observed files + CI log cited above | n/a (record gap, not code) | verification.md + a tasks.md tick (digest normalisation ignores checkbox ticks, so ticking does not stale the review) |
| Question | — | archive gate | Whether `dotf spec archive`'s draft-tag pre-flight greps `review-transcript.jsonl`: the launcher's transcript inside the spec folder embeds the literal `[AGENT-DRAFT]` (it captured the skill text); archived HOST-005 carries no transcript to compare against | grep hits only in the transcript; gate implementation is outside the repo → **UNVERIFIED** | — | (human / gate owner) |

No Blocker: the implementation itself is correct on every path I could attack (error paths close
the job handle before returning; the Wait goroutine closes it exactly once after `Wait`; refusal
falls back to pre-change behavior; `Close` is idempotent; `gosec`, `go vet` GOOS=linux/windows/darwin,
`GOOS=windows go build ./...`, `golangci-lint`, full `-race` suite all green this session).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All four ACs verified by my own runs incl. red-green mutations; the one negative path not covered is the `Start` wiring (the Major) |
| Verification       | B | Every recorded claim reproduced (Docker, mutations, lint/gosec/`-race`, PR #597 CI); the spec's own record leaves AC boxes and the Windows run unrecorded |
| Scope              | A | Spec commit matches the proposal file-for-file; the range's other 60 commits are later master work, interactions checked, none weaken containment |
| Reliability        | B | Job handle closed on every failure path and only after `Wait`; refusal best-effort and tested; residual silent-refusal log gap + documented start→assign window |
| Maintainability    | B | Small functions, WHY comments per OS, 0 lint issues; two `//nolint … baseline #629` suppressions inside the repo's accepted shrinking baseline |
| Handoff-readiness  | B | Proposal/tasks/verification/runbook/promotion answers all present; evidence record incomplete (Minor 3) keeps it from A |

### Verdict
FAIL

Severity × reality decides: one **Major × REAL** (mutation-demonstrated absence of any test over
`Start`'s containment wiring, marked UNTESTED by the traceability gate) forces FAIL even though the
rubric is all B-or-above and every acceptance criterion's mechanism verified green. The minors are
tracked, not blocking.

### Recommended next steps

Routed by set — no contract-set (`proposal.md`/`tasks.md` content/`features.json`) change is
requested; the acceptance criteria stand as written.

1. **tests — this is the flip condition**: add a named guard that fails when `Start` drops the
   containment wiring — a source-inspection assertion over `bootstrap.go` (precedent:
   `internal/layout_test.go`) or the default dependency table extracted into a testable var and
   asserted. Red-green it (wire to `nil` → red, revert → green). Then re-review with
   `dotf spec review REL-376-contain-ssh-bootstrap`.
2. **code** — disposition in `verification.md` or a follow-up ticket: warn-log the containment
   refusal instead of discarding it in `containStarted`.
3. **verification.md (outside the contract set)** — record the Windows evidence (`gh pr checks 597`
   → `test-windows pass`, `ok ts-bridge/internal/bootstrapssh 1.119s`), tick the AC boxes and the
   `tasks.md` closing box (ticks are digest-ignored and do not stale this review once the fix
   re-review replaces it).

`dotf spec archive` is **not advisable** in the current state: verdict FAIL is not a passing review,
and the gate will refuse (correctly). Minimum to flip to PASS: item 1 with red-green evidence,
re-reviewed; items 2–3 can be dispositioned alongside.

### Evidence (this session)

- `go build ./...`, `go vet ./...` + `GOOS=windows|linux go vet ./internal/bootstrapssh/`,
  `GOOS=windows go build ./...` — exit 0
- `go test -race -count=1 ./...` — exit 0; `golangci-lint run ./internal/bootstrapssh/...` — 0 issues;
  `gosec -quiet ./internal/bootstrapssh/...` — exit 0
- Docker `golang:1.27`: kill-test baseline PASS; Pdeathsig mutation FAIL (5.02s `outlived`); revert PASS
- Mutations B (close-before-wait) and C (refusal fails Start) red-green on this host; mutation D
  (wiring dropped) stayed **green** — the finding
- `gh pr checks 597` + job log `126776174194`; `scripts/check-features-selectors.sh` — OK;
  `dotf spec digest` — matches `review-request.json`

Definition of Done, reviewer's pass: **Debt** — all defects found are in the table above (filed via
this review; commenting on the PR/issue is excluded by this run's instructions — stated skip).
**Knowledge** — this review.md is the durable artifact; no cross-project insight arose. **Board** —
#376 is CLOSED (verified `gh issue view 376`), consistent with the merged change. **Review** —
out of scope for this run (instructions: do not comment on the PR) — stated skip. **Evidence** —
every claim above carries the command output produced in this session.
