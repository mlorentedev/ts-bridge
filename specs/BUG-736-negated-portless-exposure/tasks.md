---
tags: [spec, tasks]
created: "2026-11-12"
id: "BUG-736-negated-portless-exposure"
type: spec-tasks
status: active
owner: manu
---

# Tasks - BUG-736-negated-portless-exposure

## Implementation

- [x] [AC1] [AC2] [AC3] Failing table test `TestExposureShapes`: each shape through `iptUnscoped` and `ip6Exposed`
- [x] [AC1] [AC2] `iptExposes` replaces `iptPortCovers` in both reads; `iptUnscoped`'s doc comment updated
- [x] [AC1] A step-level row: v4 `-j ACCEPT` under DROP is `manual`, setup deletes nothing, `firewall_ok` false
- [x] [AC4] `scripts/e2e/linux.sh`: v4 `! --dport 22` and v6 `-p tcp -j ACCEPT` cases
- [x] Docs: parity doc Linux firewall row

## Closing

- [x] `go test -race ./...`, `golangci-lint run` (host, `GOOS=linux`)
- [x] `make e2e-linux`
- [ ] features.json replays; verification.md filled; spec review committed
- [x] PR with `Closes #736`
