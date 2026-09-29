---
tags: [spec, tasks, templates]
created: "2026-09-28"
---

# Tasks - CLI-363-ssh-tunnel-bootstrap

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from main: `feat/ssh-tunnel-bootstrap`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [P] [AC1] Add failing table-driven tests for bootstrap flag/env/YAML precedence and validation.
- [x] [AC1] Add `BootstrapSSH` and `BootstrapSOCKSAddr` to config, YAML, environment, and Cobra wiring.
- [x] [P] [AC2] [AC3] Add failing tests for SSH endpoint parsing, safe OpenSSH arguments, readiness, early exit, and cleanup.
- [x] [AC2] [AC3] Implement the isolated `internal/bootstrapssh` process lifecycle.
- [x] [P] [AC2] Add failing tests proving exact-host proxy selection preserves the original control-plane URL and delegates unrelated URLs.
- [x] [AC2] Configure the Tailscale HTTP proxy hook before `tsnet.Server.Up` and monitor the SSH process from the bridge context.
- [x] [AC3] Add production CLI-path tests proving bootstrap failures do not call the runner or emit `READY`.
- [x] [P] [AC4] Update `.env.example`, README/config documentation, ADR-005, and lesson-014.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [x] `go vet ./...` passes
- [x] Lint passes
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [x] PR opened referencing this spec folder: #372, #373, #374

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/CLI-363-ssh-tunnel-bootstrap/features.json`):

```json
[
  {
    "id": "CLI-363-ssh-tunnel-bootstrap-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
