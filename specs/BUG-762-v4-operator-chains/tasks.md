---
tags: [spec, tasks]
created: "2026-11-13"
id: "BUG-762-v4-operator-chains"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-762-v4-operator-chains

## Implementation

- [x] Pin the decision side: `TestCheckForLinux` row, a tailnet accept in an operator chain reads `changed` / "No firewall rule found"
- [x] [AC1] [AC2] [AC3] Failing rows: `TestCheckForLinux` (port allow and port-less allow in a chain, scoped, narrowed jump, ufw host, unreadable chain), plus a walk-level test (nested chain, loop)
- [x] `ip6Walk` → `iptWalk(tool, skip, exposed)`; `ip6Jumps`/`ip6Jump` → `iptJumps`/`iptJump`; v6 call unchanged in behaviour
- [x] [AC1] [AC2] v4 walk from `iptablesRead`, ufw flag threaded from `readUFW`; non-INPUT lines appended to `foreign`
- [x] [AC3] `chainUnread` on `firewallRead`: noted in the message, `ok` → `unverified`
- [x] [AC4] `scripts/e2e/linux.sh`: jumped-chain case
- [x] Docs: parity doc Linux firewall row, runbook rows (ADR-017 unchanged: no new outcome or doctrine)
- [x] File the v6 unreadable-chain follow-up (#763)

- [x] Review round 1 fixes: `chainUnread` survives the ufw fold, `0.0.0.0/0` jumps followed, shared helpers renamed `ipt*`, rows for each

## Closing

- [x] `go test -race ./...`, `golangci-lint run` (gosec: CI `security` job; not installed locally)
- [x] `make e2e-linux`
- [ ] features.json replays; verification.md filled; spec review committed
- [x] PR with `Closes #762` (#764)
