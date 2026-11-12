---
tags: [spec, tasks, templates]
created: "2026-11-11"
id: "BUG-691-permissive-input-policy"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-691-permissive-input-policy

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Setup

- [x] Branch `fix/issue-691-permissive-input-policy`, built on master after #714 (both touch the Linux firewall steps)
- [x] `proposal.md` complete; the outcome was decided by the owner on #691

## Implementation

- [x] [AC1] [AC2] [AC3] Failing table test: policy ACCEPT/DROP x rule present/absent x iptables/ufw, plus an already-unverified step
- [x] [AC1] [AC2] Read `-P INPUT <policy>` in `readIptables`; `withPolicy` promotes `ok`/`changed` to `manual` and adds the note and fix; `linuxFirewallOK` false under ACCEPT on the iptables backend
- [x] [AC2] Assert setup runs no `iptables -P` command
- [x] [AC4] `scripts/e2e/linux.sh`: `manual` under the default ACCEPT, then DROP (loopback, established, SSH first) before the existing iptables cases
- [x] Docs: `docs/design/host-client-parity.md` and the Linux notes say what a permissive policy reports
- [x] File the IPv6 policy ticket

## Closing

- [x] `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`); `gosec` is not installed locally, the CI `security` job runs it
- [x] `make e2e-linux` on the VM
- [x] `features.json` verifications replay with exit 0; `verification.md` filled
- [x] PR opened with `Closes #691`
