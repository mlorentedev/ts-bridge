---
tags: [spec, tasks]
created: "2026-11-12"
id: "BUG-733-v6-input-policy"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-733-v6-input-policy

## Implementation

- [x] [AC1] [AC2] [AC3] Failing table test `TestIP6Policy`; `TestIP6Exposure` moves to a v6 DROP fixture
- [x] [AC1] `readIP6` returns the v6 policy; `withIP6` (or a sibling) applies it; `linuxFirewallOK` reads it
- [x] [AC4] `scripts/e2e/linux.sh`: v6 ACCEPT case, then close v6 with keep-open rules first
- [x] Docs: ADR-017 Amendment B line, parity doc firewall row, troubleshooting row

## Closing

- [x] `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`)
- [x] `make e2e-linux`
- [x] verification.md filled; spec review committed
- [x] PR with `Closes #733`
