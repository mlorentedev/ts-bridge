---
spec: "CI-314-pr-agent-reviewer"
verdict: "FAIL"
reviewed_sha: "27a0838440899923778322125038ec18d4fbe880"
reviewer: "nan/deepseek-v4-flash"
date: "2026-09-22"
---
## Adversarial review

**Scope**: CI-314-pr-agent-reviewer — whole change, `git diff ebb544d2530f1504f366ebe2655579f045195e7d...HEAD` (27 commits, 65 files; the CI-314 lineage is #315 → #318/#323/#325/#328 → #336/#346, plus co-landed PRs for CI-322/SEC-212/ARCH-186 inside the same range).
**Sources**: `specs/CI-314-pr-agent-reviewer/{proposal,tasks,verification}.md` + `features.json`; `.github/workflows/pr-agent.yml`; `.pr_agent.toml`; `harness/review-attestation.json`, `harness/reviewer-pool.json`; GitHub REST (actions runs, issue comments, branch protection); the guard script re-executed locally against the live API and against a stubbed `gh`.

### Spec and task alignment

- **AC1, AC3, AC5 verified independently.** Run `33464237552` (#316) and run `35682751703` (#336) both show every step `success` including `Fail if no review was published`; `## PR Reviewer Guide` from `github-actions[bot]` exists on both. Push re-trigger runs `34098785105`, `34099871549`, `34101022805` all exist and are green. CodeRabbit posted only `Review limit reached`/`Review skipped` on #337/#339/#340 (bodies read over REST) and all three carry a PR-Agent guide.
- **AC4 verified as counted.** Eligible set recomputed from scratch (created ≥ `2026-09-01T02:49:11Z`, no `dependabot[bot]` author, no `release-please--*` head, not draft) = exactly the 15 PRs listed in `verification.md` (316, 319, 320, 321, 323, 325, 336, 337, 338, 339, 340, 346, 347, 349, 350); every one carries ≥1 `github-actions[bot]` comment containing the marker (counts 1–3). No eligible PR is missing from the list, and the stricter alternative reading of AC4 (counting release-please #324 in the denominator) still yields 15/16 = 93.75 % ≥ 90 %.
- **AC2 does not hold as written** — see F1. It is ticked `[x]` in both `proposal.md` and `tasks.md`.
- `features.json` f1 (`PR-Agent` ordered before the guard step) and f2 (model string + registry marker) both exit 0; both remain `"state": "pending"` with empty `evidence`, which is correct under the file's own pass-state rule. Neither check can observe guard semantics (ordering + parse only), so neither is evidence for F1.
- No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags in any spec file.
- Whole-range sanity: `go build ./...`, `go test ./...` (10 packages) pass; `actionlint .github/workflows/pr-agent.yml` exit 0 (shellcheck active); `scripts/check-actions-pinned.sh` and `scripts/check-workflow-permissions.sh` both `OK`.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | spec vs. observed behaviour | The Dependabot exclusion is stated at **PR** level but implemented at **actor** level, so the contract is false. `proposal.md` Out of scope: "Reviewing Dependabot or release-please PRs — both excluded"; AC2: "On a Dependabot PR … PR-Agent does not execute and posts no comment (observed, not inferred)". Observed: PR-Agent executed **and posted a review** on three Dependabot PRs (#327 `2026-09-22T02:03:34Z`, #328 `02:28:54Z`, #330 `02:40:41Z`), and on #331 the job ran and the guard **failed**. Two further routes reach the same place: a `pull_request` run re-triggered on a Dependabot branch by a human (branch update) has `actor=mlorentedev`, so the `github.actor != 'dependabot[bot]'` clause does not fire; and the `issue_comment` branch of the `if:` carries no actor/fork test at all. Downstream damage already in the record: #331's `## Review triage` states "PR-Agent — not run — `review` job skipped on this PR" while run `35681109538` ran the job and failed the guard, so a red run of this feature is undispositioned and its own triage comment contradicts it. | Runs `35677943007`, `35679575404`, `35680277625` are `event=pull_request`, `head_branch=dependabot/*`, `actor=mlorentedev`, conclusion `success`; runs `35684272053`/`35679712246`/`35299157031` are the same shape with `actor=dependabot[bot]` and are `skipped`. Run `35681109538` (#331) job `review` = `failure` at step `Fail if no review was published`. Marker comments confirmed via `repos/mlorentedev/ts-bridge/issues/{327,328,330}/comments`. | UNTESTED (no automated test; manual reproduction — the guard script extracted from the YAML, run against the live API with each run's `STARTED`: rc=0 for #327/#328/#330, rc=1 for #331) | spec (`proposal.md` AC2 + Out of scope; `tasks.md` AC2 line) — **contract set**, so fix + re-review |
| Minor | REAL | spec accuracy | `proposal.md` Out of scope: "None of the 347 PRs to date came from a fork (measured 2026-09-22 over REST)". The fork conclusion is right; the count is not. The repo has **221** PRs (352 issues+PRs), and 0 of them are forks. | `gh api "repos/mlorentedev/ts-bridge/pulls?state=all&per_page=100&page={1,2,3}"` → 100+100+21; `select(.head.repo.fork==true)` → 0 across all three pages | UNTESTED (measurement, not a test path) | spec (`proposal.md`) — contract set |
| Minor | REAL | spec staleness | `proposal.md` Why: "required checks are `[test, lint, security]`" and Out of scope: "(`test`,`lint`,`security` stay the merge gate)". Branch protection now requires `["test","lint","security","hygiene"]`, and `repo-hygiene.yml` (job name `hygiene`) was added inside this same range, so the spec's own window changed what it declares frozen. The argument it supports (CodeRabbit publishes no check-run) is unaffected. | `gh api repos/mlorentedev/ts-bridge/branches/master/protection` → `contexts:["test","lint","security","hygiene"]`; `git show ebb544d:…/repo-hygiene.yml` → absent at base | UNTESTED (measurement) | spec (`proposal.md`) — contract set |
| Minor | REAL | guard robustness | A `github-actions[bot]` comment whose `body` is `null` makes the guard abort with a raw jq error (`null (null) and string … cannot have their containment checked`) instead of its own diagnosis. Fails **closed** (rc=5, red step), so it is not a false-green path, but the operator gets no actionable message. Already known and ticketed. | Reproduced locally with a stubbed `gh` in both bash and zsh: null body → rc=5; empty-string body → rc=1 with the intended message | UNTESTED in-repo; ticketed as #345 | tests (fixture) / code — outside the contract set |
| Minor | REAL | test coverage | The guard — the one component here that can be wrong in a security-adjacent way — has **no automated test in this repo**: nothing under `scripts/tests/`, no Go test, and no workflow step references the marker or the registry. `features.json` f1/f2 are env/parse proxies. The mitigation is observational (the guard fired red on #331 and on run `35685536231`), not regression-proof; the sibling port carries `tests/pr-agent-config.bats`, this port did not bring one. | `grep -rn "pr-agent\|PR Reviewer Guide\|review-attestation" --include=*_test.go --include=*.bats --include=*.sh` → no matches outside `scripts/check-*` | UNTESTED | tests |
| Minor | SPECULATIVE | concurrency | `cancel-in-progress: true` applies per (PR, event) group; `pull_request` and `issue_comment` runs for the same PR are in different groups, so a failing run's guard window (`updated_at >= started`) can be satisfied by a *concurrent* successful run's edit of the persistent comment. That marks a run green that published nothing — harmless in effect, because the PR does have a fresh review. Not reproduced. | `concurrency.group` uses `github.event_name`; guard compares `updated_at` to the run's own stamp only | UNTESTED | code (only if a stricter binding is wanted) — surface only |
| Question | REAL | exercised surface | The `issue_comment` trigger is documented in the workflow but not named anywhere in the contract, and it is the narrower path: no `[ignore]` globs and no `extra_instructions` survive it (documented in `verification.md`). It has never executed here — all 51 `issue_comment` runs in the last 100 are `skipped`, i.e. no `/review` has ever been typed. Confirm that an undeclared, unexercised trigger belongs in the archived contract. | workflow `on: issue_comment`; run list group-by-conclusion for `event=issue_comment` → `skipped: 51` | UNTESTED | spec (name it) or code (drop it) — contract set if named |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Four of five criteria reproduce exactly against the live repo; AC2 is ticked but its own wording is contradicted by three Dependabot PRs carrying PR-Agent reviews and one red undispositioned guard run. |
| Verification       | B | Evidence is per-run, timestamped and reproducible (`verification.md` method reproduced AC4 to the PR), but it omits the human-triggered Dependabot runs and the `35681109538` guard failure, which is precisely the counter-evidence to AC2. |
| Scope              | B | The CI-314 surface is confined to `pr-agent.yml`, `.pr_agent.toml` and one registry entry, but `tasks.md`'s "diff is CI/config/spec only" does not hold for the launcher-resolved range, which also carries Go, `go.mod`/`go.sum` and lockfile changes from co-landed PRs. |
| Reliability        | B | Fail-closed paths verified by re-execution (stale comment, human-authored marker, absent comment, missing registry, pagination) and observed firing in production; residual: null-body abort and a "Most likely cause: NaN concurrency" message that is asserted even when the guard's own evidence cannot distinguish causes. |
| Maintainability    | B | Dense why-comments, small `read_marker`, actionlint+shellcheck clean, no dead config; the weak point is a ~46-line shell guard living inline in YAML with no test harness in-repo. |
| Handoff-readiness  | B | Spec, tasks and verification all updated in-session with run IDs and a lesson recorded; but a contract claim (AC2), the required-checks list and one PR-count are stale, and the #331 triage record contradicts the run it describes. |

### Verdict
FAIL

One **REAL Major** in the contract set: `proposal.md` (and `tasks.md`) assert a Dependabot-PR-level exclusion that the actor-keyed implementation does not provide, with three reviews on Dependabot PRs and one failure of this feature's own guard as evidence, plus a triage record that misstates the run. Rubric has one C, no D — but the Major decides. The implementation's intent (skip runs whose credential would be missing) is coherent and correctly implemented for Dependabot-triggered runs; what is wrong is the contract's claim, and the repo's own rule (lesson-032) makes a contract edit cost a review round.

### Recommended next steps

Contract set — these are the point of this FAIL, and they invalidate this verdict, so they precede re-review:

- `proposal.md`: restate the exclusion as actor/trigger keyed, matching the issue text and the workflow comment — "Dependabot-triggered runs are skipped on `github.actor` because that is the same field GitHub reads to select the (empty) Dependabot secret store; a human-triggered run on a Dependabot branch, and an explicit `/review` comment, do execute". Correct AC2 and its evidence line to that claim.
- `proposal.md`: replace "347 PRs" with the measured count (221 PRs, 0 forks) and update the required-checks references to `test, lint, security, hygiene`.
- `tasks.md`: reword the `[AC2]` task to the same actor-keyed claim.
- Then re-run `dotf spec review CI-314-pr-agent-reviewer`; the archive gate must stay untouched until that returns passing.

Outside the contract set — free to land in the same round or as follow-ups, and they do not extend the review:

- `verification.md`: record run `35681109538` (#331, guard `failure`) and `35685536231` (#340, guard `failure`) as the guard's observed negative path, and correct #331's `## Review triage` line via a PR comment so the durable record matches the run.
- `verification.md`: add the human-triggered Dependabot runs to the AC2 evidence paragraph; the "17 of 17 skipped" figure is only true for `actor=dependabot[bot]` runs and should say so.
- tests: add an in-repo fixture for the guard (bash + zsh) covering pass, stale-stamp, non-bot author, absent comment, head-registry bootstrap and the two-page comment list — the null-body case is already #345; the six scenarios above are already written as throwaway stubs and would port directly.
- code (optional): guard the jq `contains` against a null body (`(.body // "") | contains($marker)`) as part of #345.

**Archive advisability**: not advisable yet. `dotf spec archive CI-314-pr-agent-reviewer` will refuse this verdict (FAIL is not a passing review), which is the intended behaviour; the next round after the contract edit is the path in.
