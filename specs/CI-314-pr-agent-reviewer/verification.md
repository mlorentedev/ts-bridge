---
tags: [spec, verification, templates]
created: "2026-08-31"
---

# Verification - CI-314-pr-agent-reviewer

## Evidence

- [x] AC1 (reviewer runs): files authored — `.pr_agent.toml`, `.github/workflows/pr-agent.yml`,
      `github-actions` registry entry. Parsing evidence below. Real first run is a
      post-merge step (no PR had run PR-Agent before this).
- [x] AC2 (dependabot/release excluded): `if:` gates the two actor/name conditions; verified by
      reading the expression + the measured facts it names (empty dependabot store,
      `release-please--branches--master` head ref). Observation pending merge.
- [x] AC3 (push re-review): `handle_push_trigger=true` + `push_commands=["/review"]`. Observation
      pending merge.
- [ ] AC4/AC5 (coverage >= 90 %, vendor quota no longer binding): measurement only possible
      after this runs on real PRs. Left open; it is the spec's reason for existing.

## Test status

- Config/CI parse: `python3 -c 'import yaml,tomllib,json; ...'` on all three files -> clean, no
  exceptions. Guard-step marker resolves to `## PR Reviewer Guide`.
- Marker provenance: `## PR Reviewer Guide` posted by `github-actions[bot]` verified in
  kubelab (215 PR comments, spot-check #1500/#1362) and dotfiles (172). Not yet observed here.
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
