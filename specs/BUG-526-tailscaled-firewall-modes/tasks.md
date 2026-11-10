---
tags: [spec, tasks, templates]
created: "2026-11-09"
id: "BUG-526-tailscaled-firewall-modes"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-526-tailscaled-firewall-modes

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch `fix/issue-526-tailscaled-firewall-modes`
- [x] Capture harness and verbatim fixtures (94f320c, eb43f7d, a264c19); lesson 055 corrected (b04a956)

## Implementation

- [ ] [AC1] [AC2] Captured fixtures pin nodivert (`unverified`) and nftables on nf_tables (present): `TestReadIptablesCapturedModes`
- [ ] [AC3] Failing test: an unscoped v6 allow makes check and setup `manual`, naming `ip6tables` and its `-D` fix, with no write
- [ ] [AC3] Read `ip6tables -S INPUT` in `readLinuxFirewall`; report in the firewall steps; `FirewallOK` false
- [ ] [AC4] Failing test: a failed v6 read adds a note and keeps the outcome; no `ip6tables` changes nothing
- [ ] [AC5] `scripts/e2e/linux.sh`: v6 unscoped allow reported, then `ok` once deleted
- [ ] `docs/runbooks` Linux host note on what the v6 read reports

## Closing

- [ ] `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`, `GOOS=windows`), `gosec ./...`
- [ ] `make e2e-linux` on a real VM
- [ ] `verification.md` filled with the command output
