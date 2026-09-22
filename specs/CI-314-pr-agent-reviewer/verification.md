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
- [x] AC2 (dependabot/release excluded): the workflow still creates a run record, but the
      `review` job is `skipped`, so no PR-Agent step executes and no comment is posted.
      release-please: runs `34146702374`, `34081102575`. Dependabot: 17 of 17 dependabot runs in
      the latest 100 are `skipped`. The AC's "no run" is looser than the mechanism; "job
      skipped" is what is observed.
- [x] AC3 (push re-review): #316 push of `540ca7a` → run `33464880724` (`2026-09-01T03:04:36Z`,
      guard `success`); #325 pushes → runs `34098785105`, `34099871549`, `34101022805`, each
      publishing a fresh review.
- [ ] AC4 (coverage >= 90 % over 15 non-dependabot PRs): **11/11 reviewed so far, window 11/15.**
      Method: PRs created on or after `2026-09-01T02:49:11Z`, excluding `dependabot[bot]`
      authors, `release-please--*` heads and drafts; a PR counts as reviewed when any
      `github-actions[bot]` issue comment contains `## PR Reviewer Guide`. Counted: #316, #319,
      #320, #321, #323, #325, #336, #337, #338, #339, #340. Stays open until 15 PRs exist.
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

## Test status

- Config/CI parse: `python3 -c 'import yaml,tomllib,json; ...'` on all three files -> clean, no
  exceptions. Guard-step marker resolves to `## PR Reviewer Guide`.
- Marker provenance: `## PR Reviewer Guide` posted by `github-actions[bot]` verified in
  kubelab (215 PR comments, spot-check #1500/#1362) and dotfiles (172); observed here from #316
  onward (see AC1).
- `features.json`: f1 asserts `PR-Agent` runs before `Fail if no review was published` (order,
  not position — #323 inserted two steps ahead of `PR-Agent`); f1 and f2 both exit 0.
- Existing test suite: untouched (no Go/bats change).

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

- [ ] `status: archived` in `proposal.md`
- [ ] Folder -> `specs/archive/CI-314-pr-agent-reviewer/`
- [ ] Bitácora #314 -> Done with PR link
- [ ] Promotions executed
