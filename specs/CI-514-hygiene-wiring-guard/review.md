---
spec: "CI-514-hygiene-wiring-guard"
verdict: "PASS"
reviewed_sha: "c7e0ae04f83b8a749256371c36a26f854a6fbffa"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-01"
---

## Adversarial review

**Scope**: `CI-514-hygiene-wiring-guard` (mlorentedev/ts-bridge#514) — whole change,
`git diff fcb7746148f6fe2bf88d53bcd8bf428579f23ac9...HEAD` (3 commits, 25 files, +1627/−52).
**Sources**: `specs/CI-514-hygiene-wiring-guard/{proposal,tasks,verification}.md` + `features.json`;
`scripts/tests/test-hygiene-wiring.sh`, `scripts/tests/helpers/workflow_yaml.bash`,
`scripts/tests/test-site-pr-workflow.sh`, `scripts/tests/fixtures/hygiene-wiring/*`,
`.github/workflows/repo-hygiene.yml`. Contract digests match `review-request.json`
(`dotf spec digest` — all three identical), so this verdict is fresh and not stale.

### Spec and task alignment

Every ticked task has diff evidence; nothing is claimed that the tree does not show.

- `[x]` fixtures + stub RED, then implementation → `test-hygiene-wiring.sh` and 17 fixtures present.
- `[x]` negative probes (AC4) → reproduced myself; the `features.json` f4 command exits 0.
- `[x]` five mutants → I re-derived six mutants, all red (below).
- `[x]` `repo-hygiene.yml` runs the check as its own step → lines 42–44, right after the suite (line 41).
- `[x]` refactor to `helpers/workflow_yaml.bash`, sibling stays green → 26 fixtures, exit 0.
- `[ ]` "PR opened referencing this spec folder" — not yet, expected at this phase.
- `proposal.md` "Out of scope" (not editing the archived CI-491 `f4`) is respected: no archived file is touched.

Acceptance criteria, refuted one by one with commands I ran (not assertions I read):

| AC | Verdict | Evidence |
|---|---|---|
| 1 — `continue-on-error` anywhere fails | **met** for the plain key spelling | fixtures `guard-coe`, `suite-coe`, `job-coe`, `guard-coe-false`, `other-step-coe-expression`, `crlf-guard-coe` all exit 1; mutant M1 (rule removed) turns `crlf-guard-coe` green → the fixtures genuinely pin it |
| 2 — `if:` on guarded step/job, `needs:`, missing or nested `run:` fails | **met** | mutants M2 (`^if:` never matches) → `guard-if` green; M4 (`^job\.needs:`) → `job-needs` green; M5 (`^job\.if:`) → `job-if` green; `guard-missing`/`suite-missing`/`guard-under-env`/`guard-in-comment` exit 1 |
| 3 — current workflow, comment mentioning coe, AI-trailer `if:`, CRLF accepted; CRLF red case stays red | **met** | real file accepted (`OK (17 fixtures + …)`); `crlf-valid.yml` is genuinely CRLF (71 CR / 71 lines) and green; `crlf-guard-coe.yml` (72/72) red; `comment-mentions-coe` green |
| 4 — negative probes on the real workflow exit non-zero | **met** | the `features.json` f4 one-liner run here: exit 0, both probes non-zero |

Fail-closed property (argued, not assumed): I tried to smuggle the gate past the check with structural
rewrites — flow-style job/step, block-scalar `run: |`, quoted `run: "bash …"`, doubled spaces, a
duplicate guarded step — all produce `missing step`/`missing contract`, i.e. red. Nothing I attempted
made the checker green while removing the gate step's plain `run:`. Inline first-key forms are caught
too, which the fixtures do not cover: `- if: false` as a step's first key → `forbidden: if condition on
'bash scripts/check-conflict-markers.sh'` (exit 1); `- continue-on-error: true` as a step's first key →
`forbidden: continue-on-error` (exit 1); `continue-on-error` at job level → exit 1.

### Findings

| # | Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|---|----------|---------|------|---------|----------|---------------------------|--------------|
| 1 | Minor | THEORETICAL | detection scope | `continue-on-error` is matched only in the unquoted `key:` spelling. `continue-on-error : true` (space before colon) and `"continue-on-error": true` — both valid YAML resolving to the same key — leave the check **green** | Reproduced: inserting either into the guarded step of a copy of the real workflow → `test-hygiene-wiring: OK`, exit 0. PyYAML maps both spellings to key `continue-on-error`. This is the proposal's declared out-of-scope class ("not a defence against a workflow written to evade it"), so it needs a deliberate spelling, not an accident | UNTESTED (no fixture for a spaced/quoted key; `guard-coe`/`job-coe` cover the plain spelling) | tests (optional hardening: `continue-on-error[[:space:]]*:` plus an optional `"`), e.g. fixtures `guard-coe-spaced`, `job-coe-quoted` |
| 2 | Minor | REAL | test fixtures | `valid.yml` no longer matches the real workflow: the three-line new step is absent, yet `verification.md` and AC3 call it "a copy of the real workflow". The 17 fixtures are hand-maintained 72–74-line near-duplicates of one file, and the base already drifted inside the same PR | `diff valid.yml .github/workflows/repo-hygiene.yml` → only the new step (lines 42–44) differs. No correctness consequence: the real file is checked separately at the end of the suite | Named: the check's real-file run (`check_workflow "$WORKFLOW"`) passes; the drift itself is UNTESTED | tests + `verification.md` (regenerate `valid.yml`, or derive the fixtures from the real workflow at run time) |
| 3 | Minor | THEORETICAL | self-guard | Nothing asserts the check's own step stays wired. Deleting the `Conflict-marker gate stays wired…` step (or putting `continue-on-error` on it — the acknowledged residual risk) returns CI to the pre-#514 state silently; `features.json` f5 is a grep no CI step runs | Read of `repo-hygiene.yml` + the suite: the check validates the guard/suite wiring, never its own step; no other CI job runs f5 | UNTESTED (f5 not wired into any job) | code + tests (a "this step exists" assertion inside an already-required guard, or wire f5 in) — or accept: the same self-reference holds for every guard in this repo |
| 4 | Minor | SPECULATIVE | portability | Proposal declares the check bash-only, while AGENTS.md asks `scripts/*.sh` to run under bash **and** zsh and the CI-491 sibling suite is certified under both | I ran `zsh scripts/tests/test-hygiene-wiring.sh`: exit 0, `OK (17 fixtures + …)`. So the declared narrowing costs nothing today — it is a convention gap, not a portability bug | Named: the zsh invocation above passes | tests (optional `THW_SHELLS`-style two-shell run) |
| 5 | Minor | REAL | spec artifacts | `proposal.md` "What" omits the extraction of `strip_comments`/`job_steps`/`step_has` into `scripts/tests/helpers/workflow_yaml.bash`, which edits a sibling guard (`test-site-pr-workflow.sh`, +4/−52). `tasks.md` records it, the proposal does not. Separately, all four acceptance criteria are still `[ ]` in `proposal.md` although `tasks.md`'s closing box claims coverage (the archived CI-491 proposal ticks its own) | `git diff` of `test-site-pr-workflow.sh`; `proposal.md` AC list | Named: `bash scripts/tests/test-site-pr-workflow.sh` → `OK (26 fixtures + …)` proves the refactor is behaviour-preserving; the documentation gaps are UNTESTED by nature | spec set (`proposal.md`) — see the routing note below: on this verdict the contract set is closed, so carry both lines into the PR body / `verification.md` instead of editing `proposal.md` now (checkbox ticks are digest-normalised, so ticking those four boxes alone would not stale this review) |
| 6 | Question | — | premise | The whole gate blocks a merge only while `hygiene` is a **required** status check on `master`. Branch protection lives outside the repo | Not reachable from this worktree; no token used, no network call made | — | out of scope (UNVERIFIED) |

No Blocker. No Major. Nothing in the diff touches secrets, authz, injection surfaces, or async paths;
the change is a read-only lint over one committed YAML file plus one CI step.

**Test deletions**: none. `test-site-pr-workflow.sh` lost 52 lines of function bodies, not cases — its
26 fixtures are unchanged and green, which is the "stronger proof that remains" the retention bar asks
for. No history question arises (`git log` shows the extraction commit is the whole of it).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness | B | All four criteria verified and negative paths are dense (17 fixtures); one reproduced detection gap (#1) sits in the declared out-of-scope evasion class |
| Verification | A | Every claim in `verification.md` reproduced here, including the "equivalent mutant" CR-strip claim (re-ran: still green, so the claim is honest), the f4/f5 commands, `shellcheck -x` (exit 0) and `actionlint -shellcheck=shellcheck` (exit 0) |
| Scope | B | Diff matches `tasks.md`; the sibling-guard refactor is documented in tasks but missing from the proposal's "What" (#5) |
| Reliability | A | Fails closed on every structural rewrite I attempted (missing file, missing job, nested/quoted/flow-style `run:`, duplicate step); exit paths all name the reason |
| Maintainability | B | Small functions, comments explain WHY, one shared reader instead of copies; the 17 hand-maintained 73-line fixture copies already drifted once inside this PR (#2) |
| Handoff-readiness | B | `verification.md` carries commands + outputs and answers the promotion questions; ACs unticked in the proposal and the "PR opened" box still open (#5) |

### Verdict

**PASS** (adversarial). Rubric has no C and no D, findings are minors plus one out-of-repo question,
and nothing observed justifies blocking.

`dotf spec archive` is **advisable** once the PR is opened and merged: the contract digests are
unchanged from `review-request.json`, no `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain, and this
file carries the required non-`PENDING` frontmatter.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed** by this PASS — none of the
items below requires editing it, and editing it would invalidate this review.

1. **[tests, optional]** Add `guard-coe-spaced` (and a quoted-key variant) to close #1, per the
   proposal's own worst-case framing; decline-and-record is equally legitimate because the threat model
   is a regression lint, not an evasion defence.
2. **[tests, optional]** Regenerate `valid.yml` from the current `repo-hygiene.yml`, or generate the
   fixtures from it in-test, so #2 cannot silently recur.
3. **[tests, optional]** Wire the "the check's own step is present" assertion (#3) — or state in
   `verification.md` that it is accepted as the same self-reference every guard in this repo carries.
4. **[verification.md]** Record dispositions for #1–#5 there (applied / ticketed / declined with a
   reason). #5's two documentation lines and the four unticked AC boxes belong in the PR body or
   `verification.md`, not in a contract edit.
5. **[board]** #514 is picked up; it closes with this PR (`Closes #514`).
