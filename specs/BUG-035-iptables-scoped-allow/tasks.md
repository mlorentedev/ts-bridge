---
tags: [spec, tasks, templates]
created: "2026-10-22"
---

# Tasks - BUG-035-iptables-scoped-allow

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch created from master: `fix/issue-479-iptables-scoped-allow`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions" (measured in `ubuntu:24.04`, iptables 1.8.10 and ufw 0.36.2)

## Implementation

- [x] [P] [AC1] [AC3] Failing `TestIptablesCovers` table built from verbatim `iptables -S INPUT` lines: scoped, negated-interface, multiport, range, `--ports`, conntrack and comment-qualified ACCEPTs cover; another port, udp, `--sport`, `! --dport`, `! --dports`, DROP, a jump, no destination port and a comment spelling a covering rule do not
- [x] [AC1] [AC3] Implement `iptablesCovers` (comment value removed before splitting; flags looked up by name)
- [x] [AC2] [AC5] Failing `TestIptablesRule` / `TestSetupForLinux` / `TestCheckForLinux` cases: the read is `output iptables -S INPUT`; covered is `ok` with no `-A`; uncovered appends once; a `ts-input` jump changes nothing; a non-zero `-S` is unknown
- [x] [AC2] [AC5] Switch `readLinuxFirewall` from `-C` to `-S INPUT`
- [x] [P] [AC4] Failing `TestUFWAllows` cases: `from <net>`, `in on tailscale0`, `from <net> proto tcp` count; `proto udp`, `app <name>`, source port only do not
- [x] [AC4] Implement: a full-form allow with no destination port covers every port
- [x] Container run: a pre-existing `-i tailscale0` ACCEPT, two `host setup` runs, `-S INPUT` still one rule; an empty INPUT gets one rule after two runs
- [x] Lesson and parity doc updated

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command
- [x] `go vet` for linux, windows, darwin
- [x] `golangci-lint run` passes
- [x] No unrelated changes in the diff
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/BUG-035-iptables-scoped-allow/features.json`):

```json
[
  {
    "id": "BUG-035-iptables-scoped-allow-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
