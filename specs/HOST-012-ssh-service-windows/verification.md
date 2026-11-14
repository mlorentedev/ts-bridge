---
tags: [spec, verification, templates]
created: "2026-11-13"
id: "HOST-012-ssh-service-windows"
type: spec-verification
status: active
owner: manu
---

# Verification - HOST-012-ssh-service-windows

## Evidence

- [x] AC1 (loader) -> `TestLoadFile_OSCapability` (`windows/ssh` accepted, `darwin/ssh` still cites #459), `TestLoadServeFile` (both OSes load the ssh+rdp file through `LoadServeFile` and `LoadFile`), `TestHostCmd_ConfigSSH` (setup and check reach the step runner on linux and windows with `Services == [{ssh 2222}]`; darwin stops with exit 1 before any step).
- [x] AC2 (setup) -> `TestWindowsSSHSetup`: converged runs nothing; sshd Manual and stopped runs `sc config sshd start=auto` then `net start sshd`; Automatic and stopped runs `net start sshd` alone; an absent rule runs `New-NetFirewallRule -DisplayName "Tailscale-SSH-Ingress"` after the read. `TestWindowsSSHOnlyHost`: an ssh-only host runs no `RDP configuration` and no `Firewall rule` step and reports `RDPPort` 0.
- [x] AC3 (capability) -> `TestWindowsSSHSetup` and `TestWindowsSSHCheck` rows: `NotPresent` with no `sshd` service is `manual`, no `sc config`/`net start`/`sc query` runs and no `Add-WindowsCapability` is ever run; `NotPresent` with a service (the MSI) is `ok`, converges sshd, and reads `sshd -T` from the `BINARY_PATH_NAME` in `sc qc sshd` (`TestWindowsSSHSetup`, `TestWindowsSSHCheck` rows; `TestWindowsServiceBinary` pins the parse: quoted with arguments, unquoted, absent, and a quote in the path escaped); `Staged`, an empty state and a failed read are `failed` in setup and `unverified` in check. `TestWindowsSSHSetupMissingCapabilityNamesTheInstall` pins the fix text (capability name, WSUS, "does not install").
- [x] AC4 (check, JSON) -> `TestWindowsSSHCheck`: no mutating command in any row; `changed` for a stopped or Manual sshd; `unverified` with `Unread` when `sc` fails; `Enabled`/`Active` map Automatic/Running. `TestWindowsServiceReadState`: the two `sc` reads always both run, an error only when one fails. `TestWindowsRDPBesideSSH` (through `checkFor`): services are rdp then ssh, the legacy keys come from rdp alone, `firewall_ok` false when only the ssh rule is exposed.
- [x] AC5 (installer rule) -> `TestWindowsSSHFirewallOKIsOneSemantic` (ok is true; manual, unverified and changed are false, per service and overall); `TestWindowsSSHInstallerRule`: absent or scoped is not an exposure; enabled for any address is `manual` with `Disable-NetFirewallRule -Name OpenSSH-Server-In-TCP`; a failed read or an answer without counts is `unverified`; nothing is run. `TestWindowsSSHSetup` asserts no mutating command mentions `OpenSSH-Server-In-TCP`. `TestWindowsSSHReadsUseStableNames` pins `-Name`, not a display name or group.
- [x] AC6 (port) -> `TestWindowsSSHSetup` and `TestWindowsSSHCheck` rows for 2222: match is `ok`, sshd on 22 is `manual` naming both, unreadable is `failed` in setup and `unverified` in check with the service still read, a missing capability reads no port; no row writes `sshd_config`.
- [x] AC7 (default host unchanged) -> every pre-existing Windows test passes unchanged (`TestSetupFor`, `TestCheckFor`, `TestWindowsRDPDenyState`, `TestWindowsFirewallRule`, the golden command sequences). `TestWindowsUnknownKindFails`: a declared kind with no Windows step is a failed step and an unread service.
- [x] Red proofs for the MSI case -> counting the service as installed disabled fails `TestWindowsSSHSetup` and `TestWindowsSSHCheck`; ignoring `BINARY_PATH_NAME` fails `TestWindowsSSHCheck`.
- [x] Red proofs (mutation) -> with the installer guard's exposure test disabled, `TestWindowsSSHSetup`, `TestWindowsSSHCheck`, `TestWindowsSSHInstallerRule`, `TestWindowsRDPBesideSSH` and `TestWindowsReadsReachTheirParsers` fail; with the capability gate removed, `TestWindowsSSHSetup` and the install-text test fail; with the port step forced on for port 22, `TestWindowsSSHSetup` and `TestWindowsSSHCheck` fail. Restored, all pass.
- [ ] e2e on a Windows host -> not run (no VM was started). `scripts/e2e/windows.ps1` replaces the case that asserted the #521 refusal with a read-only `host check` of an ssh file; `features.json` f7 stays `pending`.

## Test status

- `go test -race ./...`: all packages ok.
- `GOOS=windows go vet ./...` and `GOOS=windows go build ./...` clean; `GOOS=darwin go vet ./internal/host/ ./cmd/...` clean.
- `golangci-lint run` and `GOOS=windows golangci-lint run`: 0 issues.
- `gosec ./internal/host/...` (v2.30.0 through `go run`): 0 issues.
- `scripts/check-features-shape.sh`, `scripts/check-features-selectors.sh`: OK.

## What a real Windows host must confirm

No PowerShell ran in this change. These are the commands and the output shapes the parsers assume; every one is fed canned output in the tests, so only a Windows host can confirm them.

1. `powershell -NoProfile -ExecutionPolicy Bypass -Command "$ErrorActionPreference = 'Stop'; $c = Get-WindowsCapability -Online -Name 'OpenSSH.Server*' | Select-Object -First 1; Write-Output \"state=$($c.State)\""` prints exactly one line `state=Installed` or `state=NotPresent` (any other word, or `state=`, is reported as unreadable). Assumed: the `-Name` wildcard is accepted; an unelevated shell fails with a non-zero exit (Dism requires elevation) instead of printing an empty state.
2. `sc qc sshd` prints a `BINARY_PATH_NAME   : <path>` line (quoted when it holds spaces, possibly followed by arguments) and contains `AUTO_START` for Automatic (also `AUTO_START (DELAYED)`), `sc query sshd` contains `RUNNING` when running; both fail (exit 1060) when the service does not exist.
3. `powershell ... -Command "& \"$env:WINDIR\System32\OpenSSH\sshd.exe\" -T; exit $LASTEXITCODE"` exits non-zero without elevation or host keys (a missing sshd.exe is a non-terminating error, so the shell can instead exit 0 with no `port` line; both read as unreadable), and otherwise prints lower-case lines `port 22` and `listenaddress 0.0.0.0:22` (`listenaddress [::]:22` also parses: the port after the last colon). The binary is the one `BINARY_PATH_NAME` names, `%WINDIR%\System32\OpenSSH\sshd.exe` when `sc qc` names none (the capability's path).
4. `Get-NetFirewallRule -Name 'OpenSSH-Server-In-TCP'` finds the rule the capability installs (name, not display name `OpenSSH SSH Server (sshd)`); the read prints `total=<n>` and `exposed=<n>`, where exposed counts rules with `Enabled -eq 'True'`, `Direction -eq 'Inbound'`, `Action -eq 'Allow'`, an address filter `RemoteAddress -eq 'Any'` and a port filter whose `LocalPort` is the declared port or `Any`. Assumed: a stock rule prints `22` in `LocalPort` (a range such as `20-30` is not matched).
5. The ssh rule is created and read with the RDP rule's existing scripts under the name `Tailscale-SSH-Ingress`, which `sanitizeFirewallRule` accepts.

## Decisions made during implementation

- The firewall step functions gained `windowsFirewallSetupFor` and `windowsFirewallCheckFor` taking the step name and an `fwGuard`; the RDP entry points keep their signatures and command sequences.
- `sshdPorts` was extracted from the Linux `readSSHPorts` so both OSes parse `sshd -T` the same way; the Linux behaviour is unchanged (its tests pass untouched).
- `serviceState` now wraps `readServiceState`, which keeps the failure; the `serviceStep` command sequence is unchanged.
- The rule is `Tailscale-SSH-Ingress`, not "ts-bridge SSH": see the proposal's Decisions.

## Review

Not yet reviewed. An independent review (`dotf spec review`, a reviewer from `harness/reviewer-pool.json`) is a precondition of archiving, not of this draft.
