---
tags: [spec, tasks, templates]
created: "2026-06-23"
---

# Tasks - CFG-002

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.

## Setup

- [ ] Branch created from main: `feat/CFG-002`
- [ ] `proposal.md` is complete and acceptance criteria are testable
- [ ] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

> TDD order, one commit each. New package `internal/profile` holds the descriptor + store.

- [ ] Test: `Descriptor.String()` emits `tsb://host:port?cp=…` and `Parse()` round-trips it (SaaS + Headscale `cp`); malformed input returns an error, never panics
- [ ] Implement `internal/profile` `Descriptor` + `Parse`/`String`
- [ ] Test: absent `v` defaults to version 1; unknown query params are ignored (tolerant); unknown major version returns a clear error
- [ ] Implement version handling (optional `v`, default 1, tolerant parse)
- [ ] Test: `.tsbprofile` YAML fragment (#185-shaped, with `descriptor_version`) decodes to the same `Descriptor`; unknown keys ignored
- [ ] Implement file encode/decode (tolerant reader)
- [ ] Test: importing a descriptor writes a named profile; re-importing the same descriptor is idempotent (single, uncorrupted entry)
- [ ] Implement the minimal profile store (`profiles: { <name>: { target, control_url } }`) read/write
- [ ] Test: `host check` (and `host setup` output) includes a descriptor carrying the detected port — mocked `PortNumber=45000` → contains `45000`, not `3389`
- [ ] Implement descriptor emission in host setup/check, fed by the existing per-OS detected port (emission is OS-agnostic)
- [ ] Test: `import "tsb://h:45000?cp=saas"` produces a profile whose target is `h:45000`
- [ ] Implement the `import` path (subcommand or `discover --import`)
- [ ] Test: `connect --profile <name>` resolves the target from the store and dials that port
- [ ] Implement `--profile` on connect (load store → set Target; additive to `TS_TARGET`)
- [ ] Test: `discover` records the correct port for the selected host instead of defaulting to 3389
- [ ] Implement port capture in discover
- [ ] Test: control-plane parity — a descriptor with `cp=<Headscale URL>` imports and connects identically to `cp=saas`
- [ ] Test: backward compat — existing `TS_TARGET=host:port` works with no profile present (regression)
- [ ] Docs: update `.env.example`, `README`, `docs/` for the `import` flow and `discover --port` behavior

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

Minimal `features.json` skeleton (drop into `<repo>/specs/<feature-id>/features.json`):

```json
[
  {
    "id": "CFG-002-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
