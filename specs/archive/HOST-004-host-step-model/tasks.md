---
tags: [spec, tasks, templates]
created: "2026-10-21"
---

# Tasks - HOST-004-host-step-model

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from master: `feat/issue-457-host-step-model`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation — PR A (engine + mechanical port)

- [x] [AC2] Recording `Runner` fake and golden-argv tests for Linux and Windows setup/check, plus the macOS stub (fail first: `setupFor`/`checkFor` undefined)
- [x] [AC1] `Outcome`, `Fix`, derived `Success`; test the derivation table
- [x] [AC2] `Runner` seam + `execRunner`; port Linux steps into `linux_steps.go` (untagged)
- [x] [AC2] Port Windows steps into `windows_steps.go` (untagged); split to PR A2 if over the LOC cap
- [x] [AC2] macOS stub into `darwin_steps.go`; tagged files keep only `isElevatedImpl`
- [x] Shared per-step logging; `go test -race ./...`, `GOOS=windows/darwin go vet`, lint

## Implementation — PR B1 (additive contract)

- [x] [AC3] `checkFor` becomes the read half of the setup steps; test that check issues no mutating argv
- [x] [AC4] JSON: `steps[].outcome`, `steps[].fix`, `services[]`; derived legacy keys; `smoke.bats` unchanged

## Implementation — PR B2 (exit code)

- [x] [AC5] `check` exit code from step outcomes; update `smoke.bats` and `docs/qa-coverage.md`

## Closing

- [ ] Every acceptance criterion from `proposal.md` is covered by at least one test
- [ ] Every acceptance criterion has a matching entry in `features.json`
- [ ] Lint passes
- [ ] No unrelated changes in the diff
- [ ] `verification.md` filled in
- [ ] PRs opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/HOST-004-host-step-model/features.json`):

```json
[
  {
    "id": "HOST-004-host-step-model-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
