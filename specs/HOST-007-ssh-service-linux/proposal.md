---
id: "HOST-007-ssh-service-linux"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-12"
issue: "mlorentedev/ts-bridge#521"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# HOST-007-ssh-service-linux

## Why

ADR-017 §5 lists `ssh` as a service on Linux (OpenSSH, `systemctl enable --now`), but no step exists: the host file loader rejects `kind: ssh` on Linux as an entry with no step in this version (#458). A Linux host that offers only SSH, or SSH beside xrdp, cannot be converged or checked, and `host check`'s text output is RDP-shaped (`RDP:` line, `NOT OK (rdp ...)` for any host), so an ssh-only host would be reported as a broken RDP host (#521, last comment, from #638).

This is the Linux half of #521. The Windows half (OpenSSH Server capability, `sshd` service, firewall rule) follows in a separate PR.

## What

- **Loader:** the `linux` row of `osCapabilities` accepts `ssh`; `windows` keeps rejecting it citing #521.
- **Steps:** `setupLinux` and `checkLinux` follow the declared services in order (`linuxServices`; a host declaring nothing is still the xrdp host). An `ssh` service adds, in `setup` and `check`:
  1. `SSH server`: reads which unit exists (`ssh` on Debian and Ubuntu, `sshd` on Fedora, RHEL and Arch; never guessed), then `systemctl enable --now <unit>` on a difference, using the same state read as xrdp (`readUnit`, #485: `static` counts as enabled, `masked` is `failed` in setup and `manual` in check, an answer ts-bridge does not interpret is `unverified`/`failed`). Where systemd socket-activates sshd, an enabled and active `<unit>.socket` stands in for the idle service. A missing OpenSSH server is `manual` with the package names; ts-bridge installs nothing.
  2. `SSH port` (only for a declared port other than 22): reads the listener from `sshd -T`. A match is `ok`; a mismatch is `manual` naming both ports; an unreadable configuration is `failed` in setup and `unverified` in check. ts-bridge never edits `sshd_config`.
  3. `Firewall configuration (ssh)`: the existing firewall steps, called for the ssh port (`firewallSetupFor`, `firewallCheckStep`), so the rule is scoped to `tailscale0` like RDP's (ADR-017 Amendment B) and every read (ufw, iptables, ip6tables, policy, firewalld/nftables) applies unchanged. The RDP step keeps its name.
- **Operator's unscoped SSH allow is never deleted.** The RDP firewall steps delete the unscoped allow earlier releases wrote for the RDP port (#631), since only ts-bridge ever wrote it there. For port 22 the same rule (`ufw allow 22/tcp`, `-A INPUT -p tcp --dport 22 -j ACCEPT`) is the operator's. `sshFirewallRead` reclassifies those as foreign: setup adds the scoped rule and reports `manual`, deleting nothing, and `check` reports `manual`.
- **`check`:** `services[]` gains one entry per declared service in declaration order; the legacy keys are unchanged (`rdp_*` from rdp alone; `firewall_ok` over every declared service). The text output builds one line per `services[]` entry (`SSH:`, `RDP:`); the RDP line keeps its text. A result with no services keeps the `RDP: no service declared` line.
- **`setup` summary:** an ssh-only host declares no RDP port, so `SetupResult.RDPPort` is 0 and the summary prints no `RDP Port` and no `TS_TARGET` built from port 0.
- **Docs:** ADR-017 §5 and Amendment A's note, the parity doc, `docs/runbooks/host-serve.md`.

## Out of scope

- Windows `ssh` (OpenSSH Server capability, `sshd` service, scoped firewall rule): the next PR on #521. The loader keeps rejecting `ssh` on Windows.
- macOS (#459), `vnc`.
- Moving sshd's listener or editing `sshd_config`; installing the OpenSSH package.
- Per-service descriptors and per-service `setup` client targets: an ssh-only `setup` prints no target (see Risks).
- `host init` still writes an RDP-only file.

## Risks / open questions

- **A missing OpenSSH server is `manual`, not `failed`.** The issue's acceptance text says `ok | changed | failed` per ADR-017 §2, but §2 also defines `manual` as "the OS reserves this for the user", and installing a package is what ts-bridge declines to do. `manual` exits 0 from `setup`; `check` is not `ok`, so `host check` exits 2.
- **The declared port is verified only when it is not 22.** The host file cannot tell a declared 22 from an omitted one, and an undeclared port is left to the OS (ADR-017 §5), so sshd on a non-default port with nothing declared still reads `ok` for 22. A declared non-default port is read from `sshd -T`, which needs root: unprivileged `check` is `unverified` for it.
- **`ssh` goes first in the unit lookup**, because on Debian `ssh.service` carries `sshd` as an alias and `systemctl is-enabled sshd` answers `alias`, which ts-bridge does not interpret.
- **Socket activation is read, not assumed.** `ssh.socket` (Ubuntu 22.10+) is consulted only when the service is not already enabled and running. A host with the socket enabled and the service idle reads `ok`. This reading is unit-tested through the fake runner; it has not been run on an Ubuntu guest.
- **A typical Ubuntu host (`ufw allow 22/tcp`, or `OpenSSH` app profile) will read `manual`.** The first is an unscoped allow that voids the `tailscale0` scope (Amendment B) and is never deleted; the operator scopes it, or removes it. The app-profile allow is not parsed by the ufw read (`ufw allow OpenSSH` is not counted), so setup appends the scoped rule beside it.
- The firewall message for a reclassified allow says "legacy unscoped rule", the RDP wording; the fix names the rule and says setup does not delete rules it did not write.
- `printSetupSummary` also skips the RDP lines on a Windows host whose registry read returns port 0; that case printed `RDP Port: 0` before.

## Acceptance criteria

- [ ] [AC1] The loader accepts `kind: ssh` on Linux (with a port, without one) and still rejects it on Windows citing #521, through `LoadFile` and the `host setup`/`host check` commands; `LoadServeFile` is unchanged.
- [ ] [AC2] `setup` on an ssh-only host reads the unit (`ssh`, else `sshd`), runs `systemctl enable --now <unit>` only when it is not enabled and running, reads the firewall for the ssh port before writing a `tailscale0`-scoped rule, runs no xrdp step, and runs no command on a converged host (`ok` for every step).
- [ ] [AC3] `check` reads the same state, runs no mutating command, reports `unverified` where the state cannot be read, `manual` for a missing OpenSSH server (install named, nothing installed) and a masked unit.
- [ ] [AC4] `check --json` lists `ssh` in `services[]` after rdp where both are declared, with the legacy RDP keys unchanged; `firewall_ok` is true only when every declared service's firewall allows it.
- [ ] [AC5] `check` text output prints one line per `services[]` entry: an ssh-only host prints `SSH:` and no `RDP` line.
- [ ] [AC6] Setup never deletes an unscoped allow for port 22 (`ufw allow 22/tcp`, an iptables `--dport 22` copy): it adds the scoped rule and reports `manual`.
- [ ] [AC7] A declared non-default ssh port is compared with `sshd -T`: `ok`, `manual` naming both ports, or `unverified`/`failed` when unreadable; sshd's configuration is never written.
- [ ] [AC8] A host that declares nothing, or only rdp, keeps its step list, step names and legacy output byte for byte (the existing Linux tests pass unchanged).

## References

- #521, #450, #458, #485; ADR-017 §2, §5, Amendment B; lesson 040 (every OS branch table-tested through the `Runner` seam).
