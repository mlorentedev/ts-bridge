---
tags: [spec, tasks, templates]
created: "2026-10-22"
---

# Tasks - BUG-034-linux-host-idempotent

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from master: `fix/issue-454-linux-host-idempotent`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [AC3] Failing tests for the `ufw show added` parser: `allow <port>/tcp`, `allow <port>` and an interface-scoped allow match; `33890`, `out`, `route` and `deny` do not
- [x] [AC3] Implement the ufw status parser
- [x] [AC2] [AC5] Failing tests for the iptables read: `-C` exit 0 present, exit 1 absent, other exit or no exit code unreadable
- [x] [AC2] [AC5] Implement the exit-code read through the `Runner`
- [x] [AC1] [AC2] Failing `TestSetupFor` cases: converged host makes no `run` call; missing rule runs `ufw allow <port>/tcp` or `iptables -A` once; unreadable state is `failed`
- [x] [AC1] [AC2] Implement the state-first firewall step for setup, sharing the read with check
- [x] [AC4] Failing tests for xrdp: not installed, enabled+active, disabled, inactive, unreadable, for setup and check
- [x] [AC4] Implement the xrdp service step (`systemctl is-enabled` / `is-active`, `enable --now`) and fill `services[].enabled` / `active`
- [x] [AC5] Update `TestCheckFor` golden argv and outcomes for the single-backend read
- [x] Update `docs/design/host-client-parity.md` (L1 to L4 closed) and the `host` help text

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

Minimal `features.json` skeleton (drop into `<repo>/specs/BUG-034-linux-host-idempotent/features.json`):

```json
[
  {
    "id": "BUG-034-linux-host-idempotent-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
