---
spec: "BUG-030-profile-target-source"
verdict: "PASS"
reviewed_sha: "ff6f67696aa70c69da28ed8b4cc38a4973cc75cf"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-28"
---

## Adversarial review

**Scope**: `BUG-030-profile-target-source` — round 3, at `ff6f676` (branch
`docs/bug030-trailer-disposition`, one commit ahead of `origin/master`).
Reviewed change set is the BUG-030 subset of the launcher's `3d1bb456...HEAD`
diff: `ca636e4` (#489: `cmd/cli/connect.go`, `cmd/cli/connect_target_source_test.go`,
runbook, lesson, spec), `fd6e4e1` (#496: tests + spec), and the spec-only commits
`aa3e8c6` (#497) and `ff6f676`.

**Sources**: `specs/BUG-030-profile-target-source/{proposal,tasks,verification}.md`,
`features.json`, `review-request.json`, `docs/lessons/lesson-045-profile-target-source.md`,
`docs/runbooks/guide-profile-connections.md`, `cmd/cli/connect.go`,
`cmd/cli/connect_target_source_test.go`, `internal/config/merge.go`,
`internal/config/config.go`, `cmd/cli/run.go`, `scripts/check-ai-trailers.sh`,
`.github/workflows/repo-hygiene.yml`.

### Spec and task alignment

- **AC1** (precedence preserved, one warning naming the winning source): implemented
  as a read-only diagnostic. `applyConnectProfile` now returns `selectedProfile.Target`
  from the existing read (no second store read); `runConnect` calls
  `warnConnectTargetOverride` after `config.Merge`, `lockCommandProfile` and `LoggerInit`,
  before `Runner`. Source attribution matches the resolver I read in
  `internal/config/merge.go`: `applyFlagProxyMode` then `applyEnvProxyMode` mean the
  flag wins over `TS_TARGET`, which wins over the YAML/profile layer, so the call-site
  ladder (`flagTarget` → `TS_TARGET` → `YAML`) agrees for every reachable combination.
  Met.
- **AC2** (no warning for profile-only, equal targets, no profile, SOCKS5 mode, invalid
  config): all five are implemented by the two guards plus reliance on `cfg.Target`
  being cleared in SOCKS5 mode. Four are genuinely proven; the invalid-config clause is
  asserted against a buffer that can never receive output (Finding 1). Behaviour itself
  is correct — I proved it independently with a reviewer probe. Met in code, weakly
  proven in tests.
- **AC3** (structured text/JSON, before the runner, visible under `--quiet`, no auth
  material, profile bytes unchanged): proven at command level for text and JSON, with a
  real logger (`TestConnectTargetOverrideProductionLogger`: one console record before a
  runner marker, one JSON log-file record, `--quiet` preserved) and with the
  `LoggerInit == nil` fallback (`TestConnectTargetOverrideDefaultLogger`). Arguments are
  `profile`, `source`, `target` — no credential, and `--target`/env/YAML targets that
  carry key material fail validation before the warning. The unchanged-profile-bytes
  assertion runs with `TS_AUTHKEY` set, so it bypasses `loadManagedProfileCredential`;
  `profile.Store.Get` is read-only (`store.go:139`), so the property holds, but the
  assertion does not exercise that path. Met.
- **Non-goals respected**: no precedence change, no `.env`-vs-process distinction added
  (pre-existing `.env`-overrides-process discrepancy stays with #399), no host/browser
  or credential changes. The site `cli-reference.md` delta in the launcher diff belongs
  to #490, not to this spec.
- **Task/file state**: `tasks.md` still leaves two objectively complete items unticked
  and `features.json` keeps every feature `pending` with empty evidence (Finding 2).
  No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain anywhere in the spec.
- **Contract freshness**: `dotf spec digest BUG-030-profile-target-source` reproduces the
  three digests recorded in `review-request.json` (`8f6e5772…`, `3b08b176…`, `78d893d5…`),
  so this verdict is fresh against the reviewed contract.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | verification / AC2 | The `invalid target` row proves nothing: on the error path `LoggerInit` is never called, so the captured `logs` buffer cannot receive the warning, and a warning emitted before validation would pass unnoticed. Product code today is silent; the assertion, not the product, is the problem. | Throwaway `git archive` copy, mutant M5 = extra `warnConnectTargetOverride` call before `config.Merge`: `go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning/(text|json)/invalid_target'` still `ok`, while a reviewer probe capturing `slog.Default()` failed with `{"level":"WARN",…"target":"invalid-target"}`. On unmutated HEAD both probes pass: invalid target silent on the default-logger path, `--socks5` + `TS_TARGET` clears the target silently. | `TestConnectProfileTargetOverrideWarning/{text,json}/invalid_target` (vacuous); working probe is throwaway-only | tests |
| Minor | REAL | handoff / contract status | `tasks.md` leaves "Verify lint and Linux race tests through CI" and "Open an atomic PR with Closes #417" unchecked although both are done, and `features.json` keeps `state: pending` / `evidence: ""` for all three features while the work is merged and CI-verified. The record understates completion. | `gh pr view 489 --json statusCheckRollup`: `lint`, `test` (Linux `-race`), `test-windows`, `test-macos`, `security`, `hygiene`, `smoke`, six `build-matrix` legs and GitGuardian all `SUCCESS`; #417 closed by #489. `features.json` read at HEAD. | n/a — bookkeeping | spec (contract set; editing it requires a new review round) |
| Minor | REAL | audit-trail preservation | The working tree deletes the tracked round-2 `review.md` (FAIL) and leaves `review-round2.md` and `review-request-round2.json` untracked. Committed as-is, round 2's signed report leaves the repo even though round 1's was preserved (`review-round1.md`), while `verification.md` still points at `review.md` for it. | `git status --short`: ` D specs/BUG-030-profile-target-source/review.md`, `?? review-round2.md`, `?? review-request-round2.json` | n/a | spec / git (commit the round-2 pair with this review) |
| Minor | REAL (ticketed) | correctness edge / AC2 | Target equality is a raw string compare: profile `acemagic-office:45000` vs `TS_TARGET=ACEMAGIC-OFFICE:45000` warns although it is the same DNS destination. Diagnostic-only false positive; AC2 specifies raw configured-value equality, so this is outside the contract. | Reproduced in round 2; tracked as #508 ("BUG-036: profile target override warning compares host case-sensitively"), still OPEN. | `TestConnectProfileTargetOverrideWarning/{text,json}/equal_environment_target` covers exact equality only | code (follow-up, not this spec) |
| Minor | SPECULATIVE | maintainability / drift | The winning source is re-derived at the call site (`os.Getenv("TS_TARGET")` + `flags.Target`) instead of being returned by the resolver, so a future precedence change could stale the label without a failing test. Every combination I constructed today agrees, and `TS_TARGET`/flag whitespace-only values cannot reach the warning because validation rejects them. | Code read of `internal/config/merge.go` (`applyYAML` → `applyEnv` → `applyFlags`) and `warnConnectTargetOverride`; declined with reasons in rounds 1–2, reproduced here as still true. | UNTESTED (no drift test) | code (declined; surface only) |
| Minor | REAL | governance residual | `master` still carries the AI coauthorship trailer on `ca636e4`, `fd6e4e1` and `aa3e8c6`; the owner retained history and shipped prevention instead. The new gate prevents recurrence only for `Co-authored-by` trailers in pull-request commits — it deliberately skips `push` runs and does not inspect PR/issue bodies. | `bash scripts/check-ai-trailers.sh 813d898~5 813d898` → 5 trailers incl. `fd6e4e1` (exit 1); `933f904 1553b0d` → `OK (1 commits)`. Synthetic repo: canonical and lower-case-key `co-authored-by` trailers flagged; a human `Co-authored-by: Bob Human`, `Claudette R`, and a prose mention are all ignored. Workflow step is gated `if: github.event_name == 'pull_request'`. | `scripts/tests/test-ai-trailers.sh` | vault/process (no code) |

**Severity calibration.** Findings 1, 4 and 6 are cosmetic or already-tracked: none of
them changes a bridge target, a credential or an operator-visible decision, and none is a
spec violation of this proposal. Finding 6 is **not** re-escalated to Blocker/Major: the
amendment the earlier rounds asked for (`d1bc868` reworded before merge) is no longer
available without rewriting merged history, which the owner's rule forbids, so the
remaining actionable remedy — prevention — is shipped and verified above. That is a
resolved record, not a passing verdict on the trailered history itself: `ca636e4`,
`fd6e4e1` and `aa3e8c6` still violate the standing order and are retained by decision.

### Test and mutation evidence re-run in this session

Prod and contract digests, CI state and the regression suite were re-run here, not taken
from `verification.md`:

- `go test ./cmd/cli -run 'TestConnectProfileTargetOverrideWarning|TestConnectTargetOverrideDefaultLogger|TestConnectTargetOverrideProductionLogger' -count=1` → `ok` (20.5 s).
- `go test ./cmd/cli ./internal/config/... ./internal/logging -count=1` → `ok` ×4 packages.
- `go vet ./...` → exit 0.
- Seven mutants applied one at a time to a throwaway `git archive HEAD` copy, restoring the
  pristine file between runs:
  - drop the `profileTarget == ""` guard → 2 subtest failures (`profile_without_target`, both formats);
  - drop the equality guard → 8 failures (`profile_only`, equal-target rows);
  - force `source := "YAML"` → 10 failures (`environment_overrides_YAML`, `flag_…`, `dotenv_…`);
  - drop the `target == ""` guard → 2 failures (`SOCKS5_clears_profile_target`);
  - emit the warning after `Runner` → 12 failures;
  - drop the redundant `profileName == ""` guard → green, and correctly so: `loadCommandProfile("")`
    returns an empty profile, so `profileTarget == ""` already short-circuits (redundant
    condition, not a coverage gap);
  - hoist an extra warning before `config.Merge` → the `invalid_target` row stays green
    (Finding 1); the default-logger test catches the duplicate, which is why the AC2 gap is
    scoped to the error path.
- Reviewer probes (throwaway copy, not committed): invalid target emits no warning even
  when `slog.Default()` is captured; `--socks5 127.0.0.1:1080` with `TS_TARGET` in the
  environment clears `cfg.Target` and emits no warning.
- Test-change review: `fd6e4e1` replaced a direct `warnConnectTargetOverride(...)` unit call
  with the full-command path plus a real production logger; no assertion was removed or
  weakened, so the retention bar for deletions does not apply.
- **Local `golangci-lint` was not run**, per the owner's workstation prohibition (endpoint
  protection); the `lint` result used above is the CI run on #489/#496
  (`gh pr view … --json statusCheckRollup` → `SUCCESS`). No `.exe`, wrapper, `go run`,
  container or alternative launch path was used, and no endpoint setting was touched.
- Reviewer identity: the session transcript records provider `nan`, model
  `deepseek-v4-flash`, which matches the pool id in `harness/reviewer-pool.json`, so the
  signature above is the model that reviewed.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All three ACs hold; attribution agrees with the resolver for every reachable combination; no functional defect found in three review rounds or in my mutation battery. |
| Verification       | B | Commands reproduce and CI was independently confirmed, but AC2's invalid-config row cannot fail (Finding 1). |
| Scope              | A | BUG-030's own diff is 32 productive lines in `connect.go` plus tests/docs/spec; the wider launcher diff is other specs' merges, not creep from this one. |
| Reliability        | A | Read-only diagnostic, no second store read, placed after validation/lock/logger, before the runner; SOCKS5 and error paths stay silent. |
| Maintainability    | B | 18-line function, CC 5, WHY documented; the accepted duplication of the precedence derivation stays a latent drift risk (Finding 5). |
| Handoff-readiness  | B | Proposal, tasks, verification, lesson and runbook all present; contract status is stale in `tasks.md`/`features.json` (Finding 2). |

### Verdict

**PASS**

No Blocker and no REAL Major. The two previous FAILs were the coauthorship trailer; its
recurrence is now prevented by a verified gate, and the historical commits are retained by
an owner decision recorded in `verification.md`. This verdict covers the diagnostic change
and its evidence; it does not endorse the trailered history and does not claim the gate
covers GitHub message text or push-to-`master` commits. All open items are minors with a
named disposition.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** for this
verdict: editing it invalidates the digests this review was written against. Route the
following through `verification.md` (outside the staleness set) or a follow-up ticket:

- **Finding 1 (tests, follow-up ticket or a test commit beside the spec)** — capture
  `slog.Default()` in the table test (or assert stderr is empty) so the `invalid_target`
  row can actually fail; the throwaway probe in this session is the model. Disposition it
  in `verification.md` as applied or ticketed with the ticket number.
- **Finding 2 (spec, contract set)** — do not edit it to satisfy this review. Record in
  `verification.md` that the two boxes are complete but deliberately left unticked to keep
  the contract digests stable; if the owner wants `tasks.md`/`features.json` to state
  completion, that is a contract change and needs one more review round, not a silent edit.
- **Finding 3 (git)** — commit `review-round2.md` and `review-request-round2.json` together
  with this `review.md`, so round 2 stays auditable the way round 1 does.
- **Finding 4 (code)** — keep #508 (BUG-036) as the tracked follow-up; no contract change.
- **Finding 5 (code)** — no action; the decline is coherent. Revisit only if a drift test
  is ever asked for.
- **Finding 6 (process)** — no action in this spec; carry the residual wording into the
  archive note: master retains `ca636e4`/`fd6e4e1`/`aa3e8c6`, and the gate covers
  pull-request commits only.

**Archive**: `dotf spec archive BUG-030-profile-target-source` is advisable in this state
once the review artifacts above are committed — no `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]`
tags remain, this review is fresh against the recorded digests and signed by a pool member,
and `verification.md`'s Promotion candidates are answered (`yes: docs/lessons/lesson-045-…`,
`no:` with reasons for ADR and pattern). The archive gate does not inspect task
checkboxes, which is why Finding 2 is a handoff note rather than an archive blocker.

### Definition of Done check (closing pass)

| Check | Verdict |
|---|---|
| Debt | #508 tracks the cased-target false positive; Findings 1–3 are new and each carries a disposition above. No unrecorded defect found. |
| Knowledge | Lesson `docs/lessons/lesson-045-profile-target-source.md` and the runbook paragraph exist and match the code; no ADR warranted (precedence unchanged). |
| Board | #417 closed by #489; #506 closed by #510 (gate shipped); #508 open and labelled `bug`. |
| Review | This round-3 report. #489/#496 CI triaged: `review` job cancelled/skipped in both, CodeRabbit `NEUTRAL` (a notice, not coverage) — so those PRs were not independently reviewed before merge; that is history, not a claim of coverage. |
| Evidence | The commands and outputs in "Test and mutation evidence re-run in this session"; local lint unavailable by owner prohibition, CI lint result cited instead. |
