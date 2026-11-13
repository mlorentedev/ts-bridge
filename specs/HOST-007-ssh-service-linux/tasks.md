---
tags: [spec, tasks]
created: "2026-11-12"
id: "HOST-007-ssh-service-linux"
type: spec-tasks
status: active
owner: manu
---

# Tasks - HOST-007-ssh-service-linux

## Implementation

- [x] [AC1] Flip the loader tests: linux accepts `ssh`, windows still cites #521; the CLI wiring guard moves to `vnc`, a new test pins ssh reaching the step runner on linux and stopping on windows
- [x] [AC2] [AC3] [AC8] Failing table tests `TestReadSSH`, `TestSetupSSH`, `TestCheckSSH`, `TestCheckSSHDefaultStaysRDP`
- [x] [AC2] [AC3] `linuxUnit` generalizes the xrdp unit steps; `readUnit` replaces the xrdp-only read; `linuxServices` drives `setupLinux` and `checkLinux`
- [x] [AC2] [AC3] `internal/host/linux_ssh.go`: unit lookup, socket credit, `SSH server` and firewall steps
- [x] [AC6] Failing test `TestSetupSSHKeepsTheOperatorsOwnAllow` (red: setup deleted `ufw allow 22/tcp` and the iptables copy), then `sshFirewallRead`
- [x] [AC7] `TestSSHPort`, `sshPortStep`
- [x] [AC4] `TestCheckJSON_ListsSSHBesideRDP`
- [x] [AC5] `TestPrintCheckText_ServiceLines`, `serviceLines`
- [x] `TestPrintSetupSummary_NoRDPPort`, the summary skips the RDP lines at port 0
- [x] Docs: ADR-017 §5 and Amendment A, parity doc, `docs/runbooks/host-serve.md`
- [x] `scripts/e2e/linux.sh`: ssh-only host file case (written, not run by the author)

## Closing

- [x] `go test -race ./...`, `golangci-lint run` with `GOOS=linux`, `darwin`, `windows`
- [ ] `make e2e-linux` (run by the lead)
- [ ] features.json replays; verification.md filled; spec review
- [ ] PR with `Refs #521` (the Windows half remains)
