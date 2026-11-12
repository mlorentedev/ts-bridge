---
tags: [spec, tasks, templates]
created: "2026-11-12"
id: "BUG-720-v6-jumped-chains"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-720-v6-jumped-chains

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from master: `fix/issue-720-v6-jumped-chains`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [AC2] Capture tailscaled's v6 `ts-input` with `scripts/e2e/capture-tsfw.sh`; fixture plus the `ip6tables -S ts-input` call in `TestIP6Exposure`
- [x] [AC1] [AC3] [AC4] Red rows in `TestIP6Exposure` / a jump table test: allow in ts-input, narrowed jumps, two deep, loop, LOG, failed chain read
- [x] [AC1] [AC3] [AC4] Implement the jump walk in `readIP6`, generalise `ip6Exposed` to a chain
- [x] [AC5] e2e case in `scripts/e2e/linux.sh`; `make e2e-linux`
- [x] Runbook row for a v6 allow inside a jumped chain

## Closing

- [ ] Every acceptance criterion from `proposal.md` is covered by at least one test
- [ ] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [ ] Type checks pass
- [ ] Lint passes
- [ ] No unrelated changes in the diff (no scope creep)
- [ ] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/BUG-720-v6-jumped-chains/features.json`):

```json
[
  {
    "id": "BUG-720-v6-jumped-chains-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
