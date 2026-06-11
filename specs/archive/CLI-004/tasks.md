---
tags: [spec, tasks, CLI-004]
created: "2026-06-10"
---

# Tasks - CLI-004

## Setup

- [ ] Branch from master: feat/cli-host
- [ ] Depends on CLI-001 (cmd/ package exists)

## Implementation

- [ ] Create cmd/host.go with host subcommand group
- [ ] Implement host setup (Windows): RDP enable, firewall, Tailscale unattended, sleep disable
- [ ] Implement host setup (Linux): xrdp detection, UFW/iptables config
- [ ] Implement host check: Tailscale IP, RDP port, firewall status
- [ ] Add --json flag to host check
- [ ] Admin/elevation check with clear error message

## Testing

- [ ] go test ./... green
- [ ] golangci-lint run clean

## Closing

- [ ] PR < 250 lines diff (excluding tests)
- [ ] PR references issue #51