---
tags: [spec, tasks, templates]
created: "2026-11-12"
id: "BUG-735-ufw-incoming-allow"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-735-ufw-incoming-allow

## Implementation

- [x] [AC1] [AC2] [AC3] Failing table test `TestUFWPolicy`; `TestManualRewriteClearsSuccess` gains the ufw rewrites
- [x] [AC1] `readUFW` keeps the v4 policy; one gate (iptables or active ufw) replaces `backend == backendIPT` in `withPolicy`, `withV6Policy`, `linuxFirewallOK`; ufw-specific fix text
- [x] [AC3] [AC5] Review round 1: `TestUFWPolicyUnread`, `TestUFWPolicyMessage`; an unread policy on an active ufw is unverified; the note names ufw
- [x] [AC4] `scripts/e2e/linux.sh`: `ufw default allow incoming` case before ufw is removed, then restore `deny`
- [x] Docs: ADR-017 Amendment B line, parity doc firewall rows, runbook troubleshooting row

## Closing

- [x] `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`)
- [ ] `make e2e-linux`
- [ ] verification.md filled; spec review committed
- [x] PR with `Closes #735`
