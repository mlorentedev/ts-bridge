---
spec: "HOST-012-ssh-service-windows"
verdict: "PASS"
reviewed_sha: "113bd1d5b8d9ff5acebff98eb7ba0f4933ccb798"
reviewer: "agy/gemini-3.8-flash-high"
date: "2026-11-14"
---

## Adversarial review

**Scope**: `HOST-012-ssh-service-windows` (`mlorentedev/ts-bridge#521`)
**Sources**: `specs/HOST-012-ssh-service-windows/{proposal,tasks,verification,features}.md`, `git diff 7de966d94948e207b8efbe1963eb5c7fe40651f4...HEAD`

### Spec and task alignment

All 7 acceptance criteria (AC1–AC7) defined in [proposal.md](file:///Users/manu/Projects/worktrees/ts-bridge/host-012-ssh-service-windows-first-3c3f13/specs/HOST-012-ssh-service-windows/proposal.md) are fully addressed and verified by automated tests:
- **AC1 (Loader capability)**: The loader in `internal/host/file.go` accepts `kind: ssh` on Windows and drops the `#521` planned entry. Verified by `TestLoadFile_OSCapability`, `TestLoadServeFile`, and `TestHostCmd_ConfigSSH`.
- **AC2 (Setup convergence & idempotency)**: `setupWindows` converges `sshd` and its firewall rule without touching RDP on an SSH-only host, reporting `RDPPort` 0. Once converged, no mutating commands run. Verified by `TestWindowsSSHSetup` ("converged" row) and `TestWindowsSSHOnlyHost`.
- **AC3 (Capability & Win32-OpenSSH MSI detection)**: A missing capability with no `sshd` service reports `manual` naming `Add-WindowsCapability` without mutating the service. A missing capability with an existing `sshd` service (Win32-OpenSSH MSI) reports `ok` and converges `sshd` from the service's own binary path. Unreadable capability reads report `failed` in setup and `unverified` in check. Verified by `TestWindowsSSHSetup`, `TestWindowsSSHCheck`, `TestWindowsSSHSetupMissingCapabilityNamesTheInstall`, and `TestWindowsServiceBinary`.
- **AC4 (Check read-only verification)**: `checkWindows` runs zero mutating commands. Reports `changed` for stopped or demand-start `sshd`, `unverified` when `sc` fails, and preserves legacy RDP fields while appending `ssh` to `services[]`. Verified by `TestWindowsSSHCheck` and `TestWindowsRDPBesideSSH`.
- **AC5 (Installer firewall rule isolation)**: An enabled inbound allow for `OpenSSH-Server-In-TCP` matching the declared port or port range from `Any` remote address reports `manual` with `Disable-NetFirewallRule` fix and `firewall_ok: false`. It is never disabled or deleted by ts-bridge. An absent rule is normal. The ts-bridge rule `Tailscale-SSH-Ingress` is reserved and protected against clobbering. Verified by `TestWindowsSSHInstallerRule`, `TestWindowsSSHFirewallOKIsOneSemantic`, `TestSSHInstallerRuleReadFailsClosed`, and `TestWindowsSSHReadsUseStableNames`.
- **AC6 (Listener port comparison)**: Non-default SSH ports (e.g. 2222) are compared against `sshd -T` (invoking the executable from `BINARY_PATH_NAME` or `%WINDIR%\System32\OpenSSH\sshd.exe`). Services passing `-f` arguments are left unverified/failed rather than misprobed. `sshd_config` is never modified. Verified by `TestWindowsSSHSetup`, `TestWindowsSSHCheck`, and `TestSSHWindowsPortStepServiceArguments`.
- **AC7 (Default host preservation)**: Hosts declaring nothing or only RDP execute unchanged command sequences byte-for-byte. Verified by pre-existing test suites (`TestSetupFor`, `TestCheckFor`, `TestWindowsRDPDenyState`, `TestWindowsFirewallRule`).

All implementation tasks in [tasks.md](file:///Users/manu/Projects/worktrees/ts-bridge/host-012-ssh-service-windows-first-3c3f13/specs/HOST-012-ssh-service-windows/tasks.md) are completed. `make e2e-windows` is explicitly scoped for verification on a physical Windows guest by the operator (per task list and proposal Out of Scope).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | verification | `verification.md` item 4 under "What a real Windows host must confirm" still notes `(a range such as 20-30 is not matched)`, which was superseded by commit `34312ee` implementing port range matching in `psReadSSHInstallerRule`. | `verification.md#L40` vs `windows_ssh.go#L46` and `proposal.md#L51` | `TestSSHInstallerRuleReadFailsClosed` | `specs/HOST-012-ssh-service-windows/verification.md` |
| Minor | THEORETICAL | code | In `serviceBinary`, parsing an unquoted `BINARY_PATH_NAME` uses `strings.Index(..., ".exe")`. If an unquoted path contained `.exe` in an intermediate folder name (e.g. `C:\tools.executable\sshd.exe`), it would match at the folder rather than the binary name. | `internal/host/windows_ssh.go#L112` | UNTESTED | `internal/host/windows_ssh.go` |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All acceptance criteria (AC1–AC7) are verified, fail-closed negative paths covered, and edge cases handled. |
| Verification       | A | Reproducible tests run with race detector, linters (golangci-lint, gosec) clean, real PowerShell script shapes pinned via `scriptRunner`. |
| Scope              | A | Strict alignment with proposal and ADR-017 §5; ~240 executable lines added, well within the 300 LOC cap. |
| Reliability        | A | Idempotent step execution, fail-closed parsing on unreadable states, no touching of installer rules or `sshd_config`. |
| Maintainability    | A | Functions ≤25 lines, CC ≤6 across all new functions, comprehensive package documentation and inline rationales. |
| Handoff-readiness  | A | Spec artifacts, ADR-017 §5, parity docs, and runbooks fully updated; features.json verified. |

### Verdict
PASS

### Recommended next steps
- Implementer may update the parenthetical in [verification.md](file:///Users/manu/Projects/worktrees/ts-bridge/host-012-ssh-service-windows-first-3c3f13/specs/HOST-012-ssh-service-windows/verification.md#L40) to remove the obsolete note stating ranges are not matched (`verification.md` is outside the contract set and will not invalidate this review).
- Consider using `strings.LastIndex` in `serviceBinary` for unquoted `.exe` path extraction in follow-up cleanup.
- Running `dotf spec archive HOST-012-ssh-service-windows` is **advisable** once the PR is approved.
