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
- [x] AC7 (port) -> `TestSSHPort` (match, listenaddress with a port, sshd elsewhere = manual naming both, unreadable = failed in setup and unverified in check, the service itself staying read, no port in the answer; never a `sshd_config` write or restart), `TestSSHPortDefaultAndMissingReadNothing`.
- [x] AC8 (default host unchanged) -> every pre-existing Linux test passes unchanged (`TestSetupForLinux`, `TestCheckForLinux`, `TestXRDP`, `TestXRDPStepOutcomes`, the xrdp port tests), and `TestCheckSSHDefaultStaysRDP`.
- [x] e2e on the Linux VM -> `make e2e-linux` at `981e84d`: 138 pass, 0 fail, 0 xfail, 0 xpass (log `scripts/e2e/out/linux-20261113T063111Z.log`), the three ssh-only cases included. The first run, at `1ba33a6`, failed one case: `ssh service refused, citing #521`, written on master before this change and still asserting the refusal this change removes. `981e84d` rewrote it to assert the declaration converges (`ssh service accepted on Linux, no #521 refusal`). `features.json` f6 stays `pending` in the closed contract set; this line records the run. The guest is Ubuntu with `ssh.service` enabled, so the socket-activated read has still not run on a real host (#767).

## Test status

- `go test -race ./...`: all packages ok.
- `golangci-lint run`: 0 issues with `GOOS=linux`, `GOOS=darwin` and `GOOS=windows`; `GOOS=windows go vet ./internal/host/ ./cmd/cli/` clean.
- `gosec`: not installed locally; the CI `security` job runs it.

## Decisions made during implementation

- The xrdp unit steps became `linuxUnit` methods, and `readXRDP` became `readUnit(r, unit, binary)`, so xrdp and ssh share one systemd read and one outcome mapping; the xrdp messages are byte for byte as before (the existing tests pin them).
- `SSH server` for a missing OpenSSH is `manual`, not `failed` (see Risks in the proposal).
- `services[].unread` for ssh follows the unit step alone, as for xrdp: an unverified firewall or an unreadable `sshd -T` is its own step and leaves `enabled` and `active` meaningful (`TestCheckSSH/not_root`, red before the fix).
- The firewall steps are called through `firewallSetupFor(r, read, port)`, a split of `firewallSetupStep` so the ssh path can hand in its reclassified read. No firewall read function is touched.
- `SetupResult.RDPPort` is 0 on a host that declares no rdp; `printSetupSummary` skips the RDP lines at 0.

## Review

Round 1 (nan/deepseek-v4-flash, PASS, at `981e84d`; this `review.md`). Dispositions:

- **Minor REAL, AC6's add half is untested:** applied. The row `ufw allow 22/tcp and no scoped rule` asserts that `ufw allow in on tailscale0 to any port 22 proto tcp` runs and that nothing is deleted.
- **Minor REAL, the check half of the reclassification is untested:** applied. The `TestCheckSSH` row `an operator-owned unscoped allow: firewall_ok false, manual` covers it. Red proof: with the ufw reclassification removed from `sshFirewallRead`, both new rows and the existing ufw row fail; restored, they pass.
- **Minor THEORETICAL, convergence target on a socket-activated host:** ticketed as #767. It is the owner's decision, A or B.
- **Minor REAL, `rdp_port: 0` in setup JSON undocumented:** applied in ADR-017 §3.
- **Minor SPECULATIVE, AC2 says "runs no command":** declined. The contract set is closed by this verdict, and the tests encode the intended reading: no mutating command.
- **Minor SPECULATIVE, AC4 says "after rdp" while the What says "declaration order":** declined for the same reason. Declaration order is the rule the code and the What follow.
- **Question, no `default` in the Linux service switch:** ticketed as #768.
- **Minor REAL, the RDP "legacy unscoped rule" wording on an operator's ssh allow:** declined for now. The proposal names it as a known rough edge, and the message still names the exact rule and leaves it in place. It can be reworded with #767 if that path is touched.
- **Minor REAL, no Promotion candidates section:** applied below.
- **Minor REAL, no real-guest run:** applied. The e2e line above records it.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-059-2026-11-12.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 §5 and Amendment A were amended in place, which is the decision's home
