---
tags: [spec, tasks, templates]
created: "2026-08-31"
---

# Tasks - CI-314-pr-agent-reviewer

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created stacked on `chore/reviewer-registry-port` (`ci/pr-agent-reviewer`) — the
      workflow reads `harness/review-attestation.json`, which only exists after #311 lands.
- [x] `proposal.md` complete; acceptance criteria testable.
- [x] No open questions left (surfaced in `proposal.md` Risks).
- [x] Work-gate: issue #314 open (verified over REST; GraphQL throttle bypass recorded in
      `proposal.md` frontmatter).

## Implementation (config + CI — the "test" is parsing + the post-merge observation)

- [x] [AC1] Author `.pr_agent.toml`: reasoning-class model chain, `auto_describe` off,
      `auto_improve` off, repo-specific `extra_instructions` and `[ignore]` globs.
- [x] [AC1] Author `.github/workflows/pr-agent.yml`: commit-pinned action, event-keyed
      concurrency, dependabot + release-please exclusions, `Fail if no review was published`
      guard reading the marker from the registry at base ref.
- [x] [AC1] Add `github-actions` (`## PR Reviewer Guide`) to `harness/review-attestation.json`,
      with `$comment` recording that the marker is proven in siblings but unproven here.
- [x] Validate: `yaml.safe_load` on the workflow, `tomllib` on the config, `json` on the
      registry — all parse; guard-step marker resolves.
- [ ] [AC1] After merge: first non-draft PR carries `## PR Reviewer Guide` and the guard is
      green (needs a real PR + real inference).
- [ ] [AC2] After merge: observe a Dependabot bump and a release-please PR produce no run.
- [ ] [AC3] After merge: push a fix and confirm a re-review.

## Closing

- [x] Every acceptance criterion mapped (AC1-AC5 in `program proposal.md`).
- [ ] `verification.md` filled with post-merge evidence.
- [x] No Go change; no new dependency; diff is CI/config/spec only.
- [x] PR opened referencing this spec folder and `Closes #314`.

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/CI-314-pr-agent-reviewer/features.json`):

```json
[
  {
    "id": "CI-314-pr-agent-reviewer-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
