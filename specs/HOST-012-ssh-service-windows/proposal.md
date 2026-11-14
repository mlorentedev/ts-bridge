---
id: "HOST-012-ssh-service-windows"
type: spec
status: draft # draft | implementing | verifying | archived
created: "2026-11-13"
issue: "mlorentedev/ts-bridge#521"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# HOST-012-ssh-service-windows

## Why

ADR-017 §5 lists `ssh` on Windows (OpenSSH Server), but `host setup` and `host check` reject `kind: ssh` there: the `windows` row of `osCapabilities` carries `planned: {ssh: "#521"}`. A Windows host that offers SSH, alone or beside RDP, cannot be converged or checked. HOST-007 (PR #765) shipped the Linux half; this is the Windows half that keeps #521 open.

## What

- **Loader:** the `windows` row of `osCapabilities` accepts `ssh` and drops the `planned` entry. `LoadServeFile` is unchanged.
- **Steps:** `setupWindows` and `checkWindows` follow the declared services in order (`windowsServices`; a host declaring nothing is still the RDP host). The shared steps (Tailscale service, unattended mode, SSDPSRV, upnphost, network profile, power) run once, whatever is declared. An `ssh` service adds, in `setup` and `check`:
  1. `OpenSSH Server capability`: read with `Get-WindowsCapability -Online -Name 'OpenSSH.Server*'`. `Installed` is `ok`; `NotPresent` is `manual` naming `Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0` (and that a managed host needs Windows Update, WSUS or a Features-on-Demand source); ts-bridge installs nothing, as on Linux. An answer that is not one of those two, or a failed read, is `failed` in setup and `unverified` in check (ADR-017 §2).
  2. `SSH server` (only when the capability is `Installed`): the `sshd` service set to Automatic and Running, through `serviceStep` in setup and a read-only `serviceCheckStep` in check. `check` reads `sc qc`/`sc query` and reports `changed` for drift and `unverified` when a read fails.
  3. `SSH port` (only when the capability is `Installed` and the declared port is not 22): `sshd -T` through `& "$env:WINDIR\System32\OpenSSH\sshd.exe" -T`. A match is `ok`, a mismatch `manual` naming both ports, an unreadable configuration `failed` in setup and `unverified` in check. ts-bridge never writes `sshd_config`.
  4. `Firewall rule (ssh)`: the existing Windows firewall steps for the ssh port with the ts-bridge-owned rule `Tailscale-SSH-Ingress`, scoped to the tailnet ranges like the RDP rule (Amendment B). It runs even when the capability is missing, as on Linux.
- **The installer's own rule is never touched.** The OpenSSH Server capability installs `OpenSSH-Server-In-TCP`, which allows any address. Like the built-in Remote Desktop group, an enabled inbound allow of it on the declared port from any address turns the firewall step `manual`, naming `Disable-NetFirewallRule -Name OpenSSH-Server-In-TCP` as the operator's action; `firewall_ok` stays false. It is read by `-Name`, not by its localized display name. Unlike the Remote Desktop group, an absent rule is a normal state (capability not installed, or the operator removed it), not an unknown one.
- **`check`:** `services[]` gains an `ssh` entry in declaration order (`enabled` = Automatic start, `active` = Running, `unread` when the capability or the service could not be read). The legacy RDP keys are unchanged when `rdp` is also declared. Text output already builds its lines from `services[]`.
- **`setup` summary:** an ssh-only Windows host reports `RDPPort` 0, as an ssh-only Linux host does.
- **Docs and e2e script:** ADR-017 §5 (Windows ssh paragraph) and Amendment A's note, the parity doc, `docs/runbooks/host-serve.md`, `host serve` help text, and the `windows.ps1` case that asserted the refusal (rewritten read-only; not run).

## Decisions

- **The ssh firewall rule is named `Tailscale-SSH-Ingress`**, beside `Tailscale-RDP-Ingress`. `sanitizeFirewallRule` admits only `A-Za-z0-9_-`, so a name with a space ("ts-bridge SSH") fails validation before any read. The name is a constant: `host.firewall.rule` is documented as the RDP rule and is not reused.
- **A missing OpenSSH Server is `manual`, and no `sshd` step runs.** Without the capability the service does not exist, so `sc config sshd` would fail where the contract says `manual`.
- **The firewall guard is parameterized, not copied.** `windowsFirewallSetupStep` and `windowsFirewallCheckStep` keep their signatures for RDP and delegate to variants taking the step name and the guard (`builtinGroupStep` for RDP, `sshInstallerRuleStep` for ssh).

## Out of scope

- Installing the capability, or moving the listener by writing `sshd_config`.
- Detecting an OpenSSH installed by another route (the Win32-OpenSSH MSI): see Risks.
- macOS (#459), `vnc`, per-service descriptors, `host init` writing an ssh file.
- Starting a Windows VM or running `make e2e-windows`: the author could not run any PowerShell here. The commands and output shapes the parsers assume are listed in `verification.md` for a real Windows host to confirm.

## Risks / open questions

- **No Windows host was available.** Every PowerShell read is exercised through the `Runner` seam with canned output and through `scriptRunner` (lesson 040, #601), which proves the script prints the keys the parser reads, not that Windows prints what the tests feed. The assumed shapes are listed in `verification.md`.
- **An OpenSSH installed by the Win32-OpenSSH MSI (no capability) reads as `NotPresent`**, so setup reports `manual` for a host whose `sshd` exists. The owner decided on the capability read; a service-existence fallback is possible later.
- **`Get-WindowsCapability` needs an elevated shell.** Unprivileged `check` reads `unverified` for the capability, so the ssh service entry is `unread`.
- **`sshd -T` needs the host keys**, which `sshd` generates on its first start. A host whose service never ran reads the port as unreadable (`failed` in setup, after the service step started it, so the next run reads it; `unverified` in check).
- **Only `OpenSSH-Server-In-TCP` is read.** Any other rule an operator wrote that opens the port to every address is not looked at, as with RDP.
- **The installer rule's port is compared as a string** (`22`, or `Any`); a range such as `20-30` is not matched.

## Acceptance criteria

- [ ] [AC1] The loader accepts `kind: ssh` on Windows (with a port, without one), `LoadServeFile` is unchanged, and `host setup`/`host check` reach the step runner for it.
- [ ] [AC2] `setup` on an ssh-only Windows host reads the capability, runs no RDP step, converges `sshd` to Automatic and Running only when it is not already, reads the firewall rule before writing a tailnet-scoped one, and runs no mutating command on a converged host.
- [ ] [AC3] A missing capability is `manual` with the `Add-WindowsCapability` fix in setup and check, no `sc` command runs, and nothing is installed; an unreadable capability is `failed` in setup and `unverified` in check.
- [ ] [AC4] `check` runs no mutating command, reports `changed` for a stopped or Manual `sshd`, `unverified` when `sc` cannot be read, and an `ssh` entry in `services[]` after `rdp` where both are declared with the legacy RDP keys unchanged.
- [ ] [AC5] An enabled any-address `OpenSSH-Server-In-TCP` allow on the declared port turns the ssh firewall step `manual` with `Disable-NetFirewallRule -Name OpenSSH-Server-In-TCP`, `firewall_ok` false, and is never disabled or deleted; an absent rule does not.
- [ ] [AC6] A declared port other than 22 is compared with `sshd -T`: `ok`, `manual` naming both ports, or `failed`/`unverified` when unreadable; `sshd_config` is never written.
- [ ] [AC7] A host that declares nothing, or only rdp, keeps its step list, step names and command sequence byte for byte (the existing Windows tests pass unchanged).

## References

- #521, #450, #458; ADR-017 §2, §5, Amendment B; HOST-007-ssh-service-linux; lessons 040 and #601 (reads tested through their real scripts).
