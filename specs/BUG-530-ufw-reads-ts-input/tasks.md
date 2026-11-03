---
tags: [spec, tasks, templates]
created: "2026-11-02"
---

# Tasks - BUG-530-ufw-reads-ts-input

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from main: `fix/bug-530-ufw-reads-ts-input`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

> Replace these with the actual steps for this feature. Keep them small (one commit each) and in TDD order.
> The `[P]` / `[AC<n>]` markers are optional — see the legend above. Behaviors 1 and 2 below are independent, so their *first* test task carries `[P]`.

- [x] [AC1] Add the captured ufw-host fixtures (INPUT A/C) to `linux_steps_test.go`. Add a failing `readLinuxFirewall` row: ufw shows `22/tcp`, INPUT A plus `tsInputDefault` → `rulePresent`, backend ufw.
- [x] [AC1] In the ufw branch of `readLinuxFirewall`, when `ufwDecide` returns absent, use `readIptables`. Its present answer stands; its unknown answer propagates.
- [x] [AC1] Add failing setup/check rows: no `ufw allow` is run and check is `ok`.
- [x] [AC2] Add a failing row: ufw host with INPUT C → `copies == 2`, and check reports the BUG-480 copies step. Then count copies on ufw hosts.
- [x] [AC3] Add failing rows: the iptables read fails, or there is a qualified jump with nothing decided → unknown, and setup writes nothing.
- [x] [AC4] Regression rows: ufw allow or deny decides first; neither source covers → `ufw allow <port>/tcp` as before.
- [x] [AC5] Run the mutants: skip the iptables read, and treat its error as absent. Record which rows fail in `verification.md`.
- [x] Keep each function under 40 lines and complexity under 10 (`gocyclo`/lint). Extract a `readUFW` helper if the branch grows.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` (see below) with a non-vacuous verification command
- [x] Type checks pass
- [x] Lint passes
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/BUG-530-ufw-reads-ts-input/features.json`):

```json
[
  {
    "id": "BUG-530-ufw-reads-ts-input-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
