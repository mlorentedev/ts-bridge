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

- [ ] Pin the decision side: `TestCheckForLinux` row, a tailnet accept in an operator chain reads `changed` / "No firewall rule found"
- [ ] [AC1] [AC2] [AC3] Failing rows: `TestCheckForLinux` (port allow and port-less allow in a chain, scoped, narrowed jump, ufw host, unreadable chain), plus a walk-level test (nested chain, loop)
- [ ] `ip6Walk` → `iptWalk(tool, skip, exposed)`; `ip6Jumps`/`ip6Jump` → `iptJumps`/`iptJump`; v6 call unchanged in behaviour
- [ ] [AC1] [AC2] v4 walk from `iptablesRead`, ufw flag threaded from `readUFW`; non-INPUT lines appended to `foreign`
- [ ] [AC3] `chainUnread` on `firewallRead`: noted in the message, `ok` → `unverified`
- [ ] [AC4] `scripts/e2e/linux.sh`: jumped-chain case
- [ ] Docs: parity doc Linux firewall row, ADR-017 note
- [ ] File the v6 unreadable-chain follow-up

## Closing

- [ ] `go test -race ./...`, `golangci-lint run`, `gosec ./...`
- [ ] `make e2e-linux`
- [ ] features.json replays; verification.md filled; spec review committed
- [ ] PR with `Closes #762`
