---
id: "CI-314-pr-agent-reviewer"
type: spec
status: draft # draft | implementing | verifying | archived
created: "2026-08-31"
issue: "mlorentedev/ts-bridge#314"   # repo#NNN — GitHub issue / Project item that tracks this spec
# Work-gate status: `--force-no-gate` was used at scaffold time because `gh issue view`
# (GraphQL) hit user 13562150's exhausted secondary rate limit. The gate was verified open
# over REST instead: `gh api repos/mlorentedev/ts-bridge/issues/314` -> state=open. See tasks.
tags: [spec, proposal]
template_version: "1.0"
---

# CI-314-pr-agent-reviewer

> **Naming**: file lives at `<repo>/specs/CI-314-pr-agent-reviewer/proposal.md`. `CI-314-pr-agent-reviewer` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

This repo's only machine reviewer is CodeRabbit, and it is a vendor's quota. Measured over
the 29 most recent non-dependabot PRs (2026-08-31): 17 (59 %) carried `Review limit reached`
and 2 more were skipped, so **two thirds of PRs merge read by no machine**, and because
CodeRabbit publishes no check-run here (`required checks = [test, lint, security]`) nothing in
CI says so. This feature adds a reviewer whose capacity is ours rather than a vendor's, so a
spent quota elsewhere stops meaning "no review".

## What

After this PR, on every non-draft, non-dependabot, non-release PR a second reviewer
(PR-Agent on NaN inference) runs and posts its review as a `github-actions[bot]` comment. If
that reviewer reports success but publishes no review, the `Fail if no review was published`
step goes RED — a silent absence is impossible to mistake for a pass. The reviewer capability
therefore no longer depends on CodeRabbit's availability.

## Out of scope

- The attestation GATE (a separate required check that blocks merge on unreviewed PRs). That
  is its own change (planned as `dotf pr attestation`; see the harness/ note in AGENTS.md)
  and deliberately lands after this reviewer is measured in this repo.
- Replacing or retiring CodeRabbit. It stays `advisory`, opportunistic.
- Changing required status checks (`test`,`lint`,`security` stay the merge gate).
- Any Go code change or new project dependency.
- Reviewing Dependabot or release-please PRs — both excluded, for reasons measured in the issue.

## Risks / open questions

- **Marker unproven here yet.** `## PR Reviewer Guide` is verified in the sibling repos (215
  hits in kubelab, 172 in dotfiles, all posted by `github-actions[bot]`) but has never
  appeared in ts-bridge because this workflow has not run here. The first green run is the
  verification; until then the registry entry is a declaration of intent, not of an actor.
- **NaN concurrency is shared.** The fallback chain exists because the limit is per-model
  (`mimo-v2.5` primary, `deepseek-v4-flash` fallback; inherited measurement from #1205/#1107).
- **`issue:` was recorded by hand** because `dotf spec init --issue` needs GraphQL and the
  account is inside a secondary rate limit; gate #314 verified open over REST instead.

## Acceptance criteria

- [ ] First non-draft PR after merge carries a `## PR Reviewer Guide` comment from
      `github-actions[bot]`, and the `Fail if no review was published` step is green.
- [ ] A Dependabot PR and a release-please PR produce no pr-agent run (observed, not inferred).
- [ ] Pushing a fix re-triggers a review (`handle_push_trigger` + `push_commands`).
- [ ] Reviewer coverage over the next 15 non-dependabot PRs is >= 90 %, counted the same way
      as the table in the issue. If not, the fall-through chain is the suspect, not the metric.
- [ ] A PR that CodeRabbit declines still gets read (vendor quota no longer the constraint).

## References

- Bitácora board: the GitHub issue / Project item tracking this spec (see the `issue:` frontmatter field)
- Related ADR: `<repo>/docs/adr/adr-XXX.md` (if any)
- Related patterns: `00_meta/patterns/<pattern>.md` (if any)
