---
tags: [spec, tasks, templates]
created: "2026-11-12"
id: "BUG-739-foreign-firewall"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-739-foreign-firewall

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers** (optional, additive — borrowed from `github/spec-kit`, adapt-not-adopt per #141):
> - `[P]` — this task has **no dependency on another unchecked task**, so it is safe to run in parallel (fan out to a `Workflow`, or just batch). TDD chains (test → implement → refactor of the *same* behavior) are sequential and must NOT carry `[P]`; independent behaviors can.
> - `[AC<n>]` — this task helps satisfy **acceptance criterion #`<n>`** from `proposal.md`. Lets `/spec check` map coverage deterministically; omit it and the check falls back to semantic judgment.

## Implementation

- [x] [AC1] [AC3] Failing table test `TestForeignFirewall`: firewalld running / not running (252) / absent / failing, through `firewallCheckStep` and `firewallSetupStep`, asserting the outcome, the name, no write, no policy note, `linuxFirewallOK` false
- [x] [AC1] [AC3] `readForeignFirewall` (firewalld probe) ahead of the backend choice in `readLinuxFirewall`; the v6 read is skipped when found; a dedicated step in setup and check
- [x] [AC2] [AC3] Failing rows for nft: `inet filter` with an input hook, iptables-nft and tailscaled tables only, a table with no input hook, a failed listing; plus `nftForeignTable` parser cases
- [x] [AC2] nft probe in `readForeignFirewall`
- [x] [AC4] Existing fixtures that put `nft` or `firewall-cmd` on PATH answer the new probes; the full host suite passes
- [x] The runbook-key test covers the new messages
- [x] [AC5] `scripts/e2e/linux.sh`: native `inet` table case in the iptables section
- [x] Docs: ADR-017 Amendment B line, parity doc, runbook troubleshooting rows (firewalld, nftables)

## Closing

- [x] `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`)
- [x] `make e2e-linux`; firewalld probed by hand with the built binary on a guest
- [x] verification.md filled; spec review committed
- [x] PR with `Closes #739`
