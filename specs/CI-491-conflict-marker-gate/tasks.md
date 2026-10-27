---
tags: [spec, tasks, templates]
created: "2026-10-22"
---

# Tasks - CI-491-conflict-marker-gate

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch supplied by the user from `master@a576980`: `fix/issue-491-conflict-marker-gate`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"
- [x] Confirmed the gate needs `.github/workflows/repo-hygiene.yml` (each guard is an explicit
      step); the owner chose to keep that edit and land the scripts here

## Implementation

- [x] [AC1] [AC2] [AC3] [AC4] Write `scripts/tests/test-conflict-markers.sh` with runtime-built
      repositories and assembled markers, pointed at the wished-for guard.
      Expected RED: guard not found, exit 2.
- [x] [AC1] [AC2] [AC3] Implement `scripts/check-conflict-markers.sh` (`git grep -nIE` on open
      and close markers, explicit exit-code handling, fail closed outside a work tree).
- [x] [AC4] GREEN under bash and zsh; the suite exits 1 when a shell is missing.
- [x] [AC1] [AC2] [AC3] Mutate the guard (separator back in the regex, no-match not OK,
      end-of-line form dropped, findings exit 0): each mutant reds the suite.
- [x] Lesson 048 and its index row; `check-lessons.sh` and the new guard green on the real tree.
- [x] Owner adds the two `repo-hygiene.yml` steps (guard on the real tree; fixture suite after
      the zsh install).

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [x] Type checks pass (no Go change)
- [x] Lint passes (`shellcheck` on both scripts)
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/CI-491-conflict-marker-gate/features.json`):

```json
[
  {
    "id": "CI-491-conflict-marker-gate-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
