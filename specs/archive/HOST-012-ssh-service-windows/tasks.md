---
tags: [spec, tasks]
created: "2026-11-13"
id: "HOST-012-ssh-service-windows"
type: spec-tasks
status: active
owner: manu
---

# Tasks - HOST-012-ssh-service-windows

## Implementation

- [x] [AC1] Flip the loader tests (`file_test.go`, `servefile_test.go`, `host_config_test.go`): windows accepts `ssh`
- [x] [AC3] Failing tests `TestWindowsSSHCapability` (read shapes, setup and check outcomes)
- [x] [AC4] Failing tests `TestWindowsServiceCheckStep`, then the read-only `serviceCheckStep` and `readServiceState`
- [x] [AC5] Failing tests `TestWindowsSSHInstallerRule` and the script pins, then the parameterized firewall steps
- [x] [AC6] Failing test `TestWindowsSSHPort`, then `sshWindowsPortStep` sharing `sshdPorts` with Linux
- [x] [AC2] [AC4] [AC7] Failing tests `TestSetupWindowsSSH`, `TestCheckWindowsSSH`, `TestCheckForWindowsSSHOnly`; `windowsServices` drives `setupWindows` and `checkWindows`
- [x] [AC2] [AC4] `TestWindowsReadsReachTheirParsers` gains the capability and installer-rule reads
- [x] Docs: ADR-017 §5 and Amendment A, parity doc, `host-serve.md`, `host serve` help, `windows.ps1` case

## Closing

- [x] `go test -race ./...`, `GOOS=windows go vet ./... && go build ./...`, `golangci-lint run`, `gosec ./internal/host/...`
- [ ] `make e2e-windows` (run by the lead on a Windows host; not run by the author)
- [x] features.json replays; verification.md filled
- [ ] spec review
- [ ] Draft PR with `Closes #521`
