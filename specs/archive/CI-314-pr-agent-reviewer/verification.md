---
tags: [spec, verification, templates]
created: "2026-08-31"
---

# Verification - CI-314-pr-agent-reviewer

## Evidence

Post-merge evidence below was re-read over REST on 2026-09-22 (Actions runs + issue comments);
every run ID and timestamp is checkable with `gh api repos/mlorentedev/ts-bridge/actions/runs/<id>`.

- [x] AC1 (reviewer runs): #315 merged `2026-09-01T02:49:11Z`. The next non-draft PR, #316,
      ran `pr-agent` (run `33464237552`, `2026-09-01T02:53:47Z`) with `PR-Agent` and
      `Fail if no review was published` both `success`; `github-actions[bot]` posted
      `## PR Reviewer Guide` at `02:54:49Z`. Reproduced on the current pin (v0.45.0): #336, run
      `35682751703`, every step `success` including `Fail if the reviewer has no credential`,
      guide posted `2026-09-22T03:23:31Z`.
- [x] AC2 (Dependabot-triggered runs and release-please excluded): the workflow still creates a
      run record, but the `review` job is `skipped`, so no PR-Agent step executes and no comment
      is posted. release-please: runs `34146702374`, `34081102575`. Dependabot: every run
      **triggered by `dependabot[bot]`** in the latest 100 is `skipped` (17 on 2026-09-21, 13 on
      2026-09-22 as the window moved). **Runs a human triggers on a Dependabot branch execute**:
      in the same window 3 succeeded and posted reviews (#327 `35677943007`, #328 `35679575404`,
      #330 `35680277625`), 1 failed its guard (#331 `35681109538`) and 1 was cancelled. The first
      wording of this AC ("a Dependabot PR produces no run") was PR-level while the `if:` keys on
      `github.actor`. The contract was corrected after the round-5 review.
- [x] AC3 (push re-review): #316 push of `540ca7a` → run `33464880724` (`2026-09-01T03:04:36Z`,
      guard `success`); #325 pushes → runs `34098785105`, `34099871549`, `34101022805`, each
      publishing a fresh review.
- [x] AC4 (coverage >= 90 % over 15 non-dependabot PRs): **15/15 reviewed (100 %), window closed
      2026-09-22.** Method: PRs created on or after `2026-09-01T02:49:11Z`, excluding
      `dependabot[bot]` authors, `release-please--*` heads and drafts; a PR counts as reviewed when
      any `github-actions[bot]` issue comment contains `## PR Reviewer Guide`. Counted: #316, #319,
      #320, #321, #323, #325, #336, #337, #338, #339, #340, #346, #347, #349, #350 (re-counted over
      REST on 2026-09-22; none unreviewed), so the criterion's fallback clause (suspect the model fall-through chain) did not apply.
- [x] AC5 (a PR CodeRabbit declines still gets read): CodeRabbit posted only `Review limit
      reached` on #337 and #339, and declined a later push on #340; all three carry a PR-Agent
      `## PR Reviewer Guide` comment.

Observed publication shape: on PRs with several pushes (#319, #320, #321, #323, #325) PR-Agent
could not update its persistent comment and posted an extra `## Standalone PR Review` comment
that also carries `## PR Reviewer Guide`, so one PR can hold 2–3 marker-bearing comments. The
guard stays correct (it binds to a comment touched after the run's start stamp); this is
upstream behaviour (`pr_reviewer.py::_as_non_authoritative_review`), not fixed here.

Known asymmetry: a manual `/review` (`issue_comment`) run does not apply `.pr_agent.toml`
(upstream only calls `apply_repo_settings` when the payload has `pull_request.html_url`), so
only settings duplicated as env survive on that path. No `issue_comment` run has executed here.

Guard negative path, observed rather than simulated: `Fail if no review was published` went
**red** on run `35681109538` (#331, human-triggered on a Dependabot branch) and on run
`35685536231` (#340). In both, a PR-Agent run reported success without publishing a comment the
run could bind to. #331's own triage comment said the job was skipped; a correction comment was
posted on #331 on 2026-09-22.

## Test status

- Config/CI parse: `python3 -c 'import yaml,tomllib,json; ...'` on all three files -> clean, no
  exceptions. Guard-step marker resolves to `## PR Reviewer Guide`.
- Marker provenance: `## PR Reviewer Guide` posted by `github-actions[bot]` verified in
  kubelab (215 PR comments, spot-check #1500/#1362) and dotfiles (172); observed here from #316
  onward (see AC1).
- `features.json`: f1 asserts `PR-Agent` runs before `Fail if no review was published` (order,
  not position — #323 inserted two steps ahead of `PR-Agent`); f1 and f2 both exit 0.
- Existing test suite: untouched (no Go/bats change).

## Adversarial review round 5 (FAIL, 2026-09-22) — dispositions

Reviewer `nan/deepseek-v4-flash`, `reviewed_sha` `27a0838`.

| Finding | Disposition |
|---|---|
| **Major**: the Dependabot exclusion is described per PR but implemented per actor; PR-Agent reviewed #327/#328/#330 and failed on #331 | **Contract corrected** (`proposal.md` What, Out of scope and AC2; `tasks.md` AC2) to describe the actor-keyed exclusion and name the human-triggered path. Evidence above. The #331 triage record was corrected on the PR |
| Minor: "347 PRs" | **Corrected**: 221 PRs, 0 forks. 347 was the highest PR *number*, not the count; that mistake was the implementer's |
| Minor: the required checks quoted as `[test, lint, security]` | **Corrected**: the text keeps the historical value and notes that CI-322 added `hygiene` on 2026-09-22 |
| Minor: a `null` comment body makes the guard abort with a raw jq error | **Already ticketed: #345** (fails closed, no false green) |
| Minor: the guard has no automated test in this repo | **Folded into #345**, which now also asks for a bash + zsh fixture suite for the guard (pass, stale stamp, non-bot author, no comment, head-registry bootstrap, second page, null body) |
| Speculative: concurrent `pull_request` and `issue_comment` runs could let one bind to the other's comment edit | **Declined, surface only.** Not reproduced; the effect is benign, because the PR does hold a fresh review. No `issue_comment` run has ever executed here (51 of 51 skipped at review time, 64 of 64 on re-count) |
| Question: the `issue_comment` trigger is not named in the contract | **Named** in `proposal.md` What, including that `.pr_agent.toml` does not apply on it |

## Adversarial review round 6 (PASS-WITH-GAPS, 2026-09-22) — dispositions

Reviewer `agy/gemini-3.1-pro-high`, `reviewed_sha` `bf53cf2`. It confirms the round-5 Major is
fixed by the contract correction and advises archiving.

| Finding | Disposition |
|---|---|
| Minor: `features.json` maps only AC1 (f1, f2); AC2–AC5 have no machine-readable feature | **Declined, with the reason.** AC2–AC5 are observations over live GitHub state: which runs were skipped, which PRs carry a bot comment, how many PRs fall in a window. A `verification` command for them would call the network and return a different answer as the window moves, which is not what the harness's exit-0 contract measures. Each has its counting method and its evidence here instead. Adding entries would also make this review stale by construction (same trade as SEC-212's F6) |
| Minor: the guard's `jq` aborts on a `null` body | **Ticketed: #345** (fails closed) |
| Minor: the guard has no automated test | **Ticketed: #345**, widened on 2026-09-22 with the seven fixture scenarios |
| Speculative: concurrent `pull_request` / `issue_comment` runs | **Declined**, as in round 5: not reproduced, the effect is benign, and 64/64 `issue_comment` runs were skipped |

## Decisions made during implementation

- `repo_context_files = ["AGENTS.md"]` only — no CLAUDE.md exists at root or under `.claude/`
  (checked); a context file the reviewer cannot read asserts nothing.
- Dependabot exclusion keys on `github.actor` because it is the same field GitHub uses to pick
  the (empty) Dependabot secrets store; name-keyed exclusions are only for release-please
  (`release-please--branches--master` is authored by a human PAT, so actor matching would not
  fire).
- `model_weak` absent: `auto_describe` is off so it would do nothing, and the default model is
  the one the reviewer pool rejects by name.
- The guard fails LOUD rather than omitting: a credential failure becomes a red job, never a
  green one with no review (the exact failure this repo's #314 records as the status quo).

## Promotion candidates

- [x] Lesson for `docs/lessons/`? Yes — the "scope: verify the reviewer can answer before
      trusting its silence" pattern, and the unproven-marker-before-first-run honesty. Add
      to lesson-030 or a new lesson in the PR.
- [ ] ADR-worthy? Likely yes for the attestation GATE decision (separate spec), not for the
      reviewer itself. Deferred to the gate spec.
- [ ] New pattern? Already covered by `pattern-derived-fact-drift` / the pool's own entry.

## Archive checklist

- [x] `status: archived` in `proposal.md` (`dotf spec archive`, 2026-09-22, round-6 review accepted as fresh)
- [x] Folder -> `specs/archive/CI-314-pr-agent-reviewer/`
- [x] Bitácora #314 -> Done with PR link (#314 closed by #315; board item set to Done with #353)
- [x] Promotions executed: lesson-030 (reviewer silence) and lesson-033 (the exclusion described by the wrong field). The ADR is deferred to the attestation-gate spec, and the pattern is already covered
