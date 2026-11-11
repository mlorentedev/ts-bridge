---
tags: [spec, tasks, templates]
created: "2026-11-11"
id: "HOST-011-control-liveness"
type: spec-tasks
status: active
owner: manu
---

# Tasks - HOST-011-control-liveness

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch: `fix/issue-565-control-liveness` (worktree `fix-issue-565-control-liveness`)
- [x] `proposal.md` complete; the signal and bound come from the 2026-11-11 measurement
- [x] No open question blocks the work (the two risks are stated with their mitigation)

## Implementation

- [ ] [AC2] Failing test: `WatchControlOnline` stays ready through two offline samples and goes down on the third; a status error counts as offline
- [ ] [AC2] Implement `WatchControlOnline(ctx, status, ticks, r)` in `internal/serve/control.go`
- [ ] [AC1] Failing test: after a down, the first online sample marks `control-online` up; `/health/ready` carries `control_plane_unreachable` while it is down
- [ ] [AC1] Implement the up path
- [ ] [AC1] [AC3] Failing test: `watchServeControl` starts both watchers on a node with a local API, and the bus watcher's existing tests still pass unchanged
- [ ] [AC1] Wire the poller in `cmd/cli/host_serve.go` (10 s ticker, `lc.Status`)
- [ ] [AC1] e2e: `run.sh mesh linux` stops the Headscale container after `READY`, the guest polls `/health/ready` for the 503 and its reason, restarts it, and polls for the 200 (`host.mesh.health_addr` is already in `mesh_linux.sh`)

## Closing

- [ ] Every acceptance criterion is covered by a test and a `features.json` entry with a non-vacuous command
- [ ] `go test -race ./...`, `golangci-lint run`, `gosec ./...` clean
- [ ] `make e2e-mesh-linux` green, with the outage case
- [ ] `verification.md` filled in
- [ ] PR opened with `Closes #565`
- [ ] Independent spec review before archive

## Machine-readable features

This spec emits a sibling `features.json` (alongside this file) following [[pattern-feature-list-as-primitive]]. The JSON is the harness-facing contract: each acceptance criterion maps to ≥1 feature with `id`, `behavior`, `verification` (executable command), `state` (lifecycle), and `evidence` (harness-captured output).

**Pass-state gating:** the agent CANNOT write `"state": "passing"` — only the harness, after running `verification` and capturing exit code 0, may set that terminal state. Reviewers must reject PRs where features.json contains `passing` entries with empty `evidence`.

Minimal `features.json` skeleton (drop into `<repo>/specs/HOST-011-control-liveness/features.json`):

```json
[
  {
    "id": "HOST-011-control-liveness-f1",
    "behavior": "<one-line copy of an acceptance criterion>",
    "verification": "<single shell command; exit 0 means pass>",
    "state": "pending",
    "evidence": ""
  }
]
```
