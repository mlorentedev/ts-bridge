---
tags: [spec, verification, templates]
created: "2026-11-12"
id: "HOST-007-ssh-service-linux"
type: spec-verification
status: active
owner: manu
---

# Verification - HOST-007-ssh-service-linux

## Evidence

- [x] AC1 (loader) -> `TestLoadFile_OSCapability` (`linux/ssh` accepted, `windows/ssh` still cites #521), `TestLoadServeFile` (linux accepted, windows rejected), `TestHostCmd_ConfigSSHOnLinux` (setup and check reach the step runner on linux with `Services == [{ssh 2222}]`; windows stops with exit 1 before any step), `TestHostSetupCmd_ConfigOSCapability` (the wiring guard, moved to `vnc`).
- [x] AC2 (setup) -> `TestSetupSSH`: converged runs nothing; enabled-but-stopped runs `systemctl enable --now ssh`; a Fedora host (`ssh` not-found) runs `systemctl enable --now sshd`; a closed port runs `ufw allow in on tailscale0 to any port 22 proto tcp`; no xrdp step on an ssh-only host; rdp then ssh in declaration order with the rdp firewall step name unchanged. `TestSetupSSHOnlyReportsNoRDPPort`. `TestReadSSH` pins the unit lookup (ssh, then sshd; the Debian `alias` answer never consulted; socket credit; masked not rescued by its socket; no systemctl).
- [x] AC3 (check) -> `TestCheckSSH`: no `run` call in any row; no systemctl is `unverified` with `Unread`; not installed is `manual`; `TestSetupSSHMissingServerNamesTheInstall` pins the package names and "does not install".
- [x] AC4 (JSON) -> `TestCheckSSH` (rdp+ssh: services in order, legacy keys from rdp alone, `FirewallOK` false when only the ssh port is closed), `TestCheckJSON_ListsSSHBesideRDP`.
- [x] AC5 (text) -> `TestPrintCheckText_ServiceLines`: an ssh-only host prints `SSH:` and neither `RDP` nor `no service declared`; stopped and unread rows; rdp+ssh prints one line each. `TestPrintCheckText_Service` (the existing RDP rows) passes unchanged.
- [x] AC6 (operator's allow) -> `TestSetupSSHKeepsTheOperatorsOwnAllow`. Red proof: with `sshFirewallRead`'s reclassification disabled the test fails with `setup deleted a rule ts-bridge did not write: "ufw delete allow 22/tcp"` and `"iptables -D INPUT -p tcp --dport 22 -j ACCEPT"` (twice, one per copy), and the firewall step reads `changed` ("Legacy unscoped firewall rule deleted"). Restored, it passes.
- [x] AC7 (port) -> `TestSSHPort` (match, listenaddress with a port, sshd elsewhere = manual naming both, unreadable = failed in setup and unverified in check with `Unread`, no port in the answer; never a `sshd_config` write or restart), `TestSSHPortDefaultAndMissingReadNothing`.
- [x] AC8 (default host unchanged) -> every pre-existing Linux test passes unchanged (`TestSetupForLinux`, `TestCheckForLinux`, `TestXRDP`, `TestXRDPStepOutcomes`, the xrdp port tests), and `TestCheckSSHDefaultStaysRDP`.
- [ ] e2e on the Linux VM -> `scripts/e2e/linux.sh` gains three ssh-only cases. Written, shellchecked, not run by the author; the lead runs `make e2e-linux`.

## Test status

- `go test -race ./...`: all packages ok.
- `golangci-lint run`: 0 issues with `GOOS=linux`, `GOOS=darwin` and `GOOS=windows`; `GOOS=windows go vet ./internal/host/ ./cmd/cli/` clean.
- `gosec`: not installed locally; the CI `security` job runs it.

## Decisions made during implementation

- The xrdp unit steps became `linuxUnit` methods, and `readXRDP` became `readUnit(r, unit, binary)`, so xrdp and ssh share one systemd read and one outcome mapping; the xrdp messages are byte for byte as before (the existing tests pin them).
- `SSH server` for a missing OpenSSH is `manual`, not `failed` (see Risks in the proposal).
- The firewall steps are called through `firewallSetupFor(r, read, port)`, a split of `firewallSetupStep` so the ssh path can hand in its reclassified read. No firewall read function is touched.
- `SetupResult.RDPPort` is 0 on a host that declares no rdp; `printSetupSummary` skips the RDP lines at 0.
