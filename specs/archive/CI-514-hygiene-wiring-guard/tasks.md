---
tags: [spec, tasks, templates]
created: "2026-10-31"
---

# Tasks - CI-514-hygiene-wiring-guard

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Worktree from `origin/master@fcb7746` via `dotf worktree add --issue 514`: `fix/hygiene-wiring-guard`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"
- [x] Confirmed the check needs its own `repo-hygiene.yml` step. The owner first kept that edit
      (same split as CI-491), then asked for it in this PR on 2026-11-01.

## Implementation

- [x] Refactor: move `strip_comments` and `job_steps` from `test-site-pr-workflow.sh` into
      `scripts/tests/helpers/workflow_yaml.bash` (job name as an argument); the sibling stays
      green (26 fixtures).
- [x] [AC1] [AC2] [AC3] Commit fixtures under `scripts/tests/fixtures/hygiene-wiring/`, generated
      from the real workflow, and `scripts/tests/test-hygiene-wiring.sh` with a stub check.
      Expected RED: every red fixture exits 0.
- [x] [AC1] [AC2] [AC3] Implement the check (file-wide `continue-on-error` key, job `if:`/`needs:`,
      guarded `run:` keyed by parent with no `if:`). GREEN: fixtures + real workflow.
- [x] [AC4] Negative probes: `continue-on-error: true` on each guarded step of a copy of the real
      workflow, passed as the optional argument; both exit 1.
- [x] Mutants of the check (five); `crlf-valid` added for the CRLF accepting side.
- [x] `repo-hygiene.yml` runs the check as its own step, right after the fixture suite.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [x] Type checks pass (no Go change)
- [x] Lint passes (`shellcheck -x` on the new and refactored scripts)
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/CI-514-hygiene-wiring-guard/features.json`):

```json
[
  {
    "id": "CI-514-hygiene-wiring-guard-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
