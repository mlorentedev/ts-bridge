---
tags: [spec, tasks, templates]
created: "2026-10-22"
---

# Tasks - CLI-476-setup-exit-status

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from master: `fix/issue-476-setup-exit-status`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions" (option B chosen by the owner, `manual` exits 0)

## Implementation

- [x] [AC1] [AC2] [AC3] Failing `TestHostSetupCmd_ExitStatus`: failed step (text, JSON) exits 2 with the result printed; ok/changed/manual exits 0; not elevated exits 1. Needs injectable `hostSetup` and `hostIsElevated`
- [x] [AC1] [AC2] Implement `setupConvergence`: an `ExitError` with code 2 when any step is `failed`
- [x] [AC4] Amend ADR-017 §2 with setup's exit status and the `setup && check` idiom
- [x] CLI reference and parity doc updated

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [x] Type checks pass
- [x] Lint passes
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [x] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/CLI-476-setup-exit-status/features.json`):

```json
[
  {
    "id": "CLI-476-setup-exit-status-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
