---
tags: [spec, tasks, templates]
created: "2026-10-30"
---

# Tasks - BUG-492-ts-input-chain

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch `fix/issue-492-ts-input-chain` from `origin/master`
- [x] `proposal.md` complete; scope, read-failure and out-of-scope decided by the owner on #492 (2026-10-30), follow-up #526
- [x] Rule shape replayed from tailscale.com v1.104.2 and recorded on #492

## Implementation

- [ ] [AC1] [AC2] [AC5] Failing `TestIptablesTSInput`: one table of INPUT + `ts-input` pairs in printed form (tailscaled default, deny before the jump, deny after it, tailscaled's rules without the ACCEPT, a qualified jump)
- [ ] [AC1] [AC2] [AC3] `iptInlineChain` evaluates the chain at an unqualified jump's position; `iptablesDecide` is unchanged
- [ ] [AC1] [AC3] [AC4] Failing setup/check cases: the tailscaled default appends nothing, an unreadable `ts-input` means `failed`/`unverified`, and no second read without a jump
- [ ] [AC3] [AC4] `readIptables` reads `ts-input` only behind the jump and reports a failed read as unknown; existing call lists gain the second read where their fixture jumps

## Closing

- [ ] Every acceptance criterion covered by a named test
- [ ] `features.json` entries with executable verification
- [ ] `go test -race ./...`, `gofmt -l .`, `go vet` on linux/windows/darwin; `golangci-lint run` locally or the CI `lint` job
- [ ] Lesson in `docs/lessons/`, `docs/design/host-client-parity.md` row updated
- [ ] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/BUG-492-ts-input-chain/features.json`):

```json
[
  {
    "id": "BUG-492-ts-input-chain-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
