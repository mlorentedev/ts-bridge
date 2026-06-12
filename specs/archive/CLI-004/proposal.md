---
id: "CLI-004"
type: spec
status: proposed
created: "2026-06-10"
tags: [spec, cli, host, setup, windows, linux]
issue: 51
---

# CLI-004: Implement ts-bridge host subcommand

## Why

Replace scripts/host/setup.ps1 with a cross-platform Go subcommand. ts-bridge host setup configures RDP access on the host machine; ts-bridge host check verifies readiness.

## What

Implement ts-bridge host setup and ts-bridge host check.

### ts-bridge host setup

On Windows, replicates setup.ps1 logic:
- **Admin check FIRST**: detect if running as admin. If not, print clear error with instructions to re-run from an elevated terminal. Do NOT attempt any operations.
- Enable RDP via registry (HKLM:\System\CurrentControlSet\Control\Terminal Server\fDenyTSConnections)
- Enable Tailscale unattended mode (tailscale up --unattended)
- Configure Windows firewall rule for RDP on Tailscale interface
- Enable UPnP services (SSDPSRV, upnphost)
- Set Tailscale network profile to Private
- Optionally disable sleep (--no-sleep flag to skip)

On Linux:
- Detect xrdp installation
- Configure UFW/iptables firewall rule for RDP port
- Print Tailscale IP

On macOS:
- Print message: "Host setup is not applicable on macOS. Use the client mode (ts-bridge connect) to connect to a remote host."

### ts-bridge host check

Read-only verification (no admin required for check):
- Check if RDP is enabled and on which port
- Check Tailscale status and IP
- Check firewall rules
- Print human-readable summary
- --json flag for machine-readable output

### CLI Flags

```
ts-bridge host setup [flags]
  --no-sleep          Skip disabling sleep mode
  --firewall-rule NAME  Custom firewall rule name (default: Tailscale-RDP-Ingress)

ts-bridge host check [flags]
  --json              Output JSON
```

## Out of scope

- Host deployment automation (Ansible, etc.)
- Multi-host management

## Dependencies

- Depends on CLI-001 (Cobra scaffold)

## Acceptance criteria

- [ ] ts-bridge host setup on Windows: detects non-admin and errors with clear message
- [ ] ts-bridge host setup on Windows (as admin): enables RDP, firewall, Tailscale unattended
- [ ] ts-bridge host check prints Tailscale IP, RDP port, firewall status
- [ ] ts-bridge host check --json outputs valid JSON
- [ ] Linux: detects xrdp, configures firewall
- [ ] macOS: prints "not applicable" message
- [ ] go test ./... green
- [ ] golangci-lint run clean
- [ ] PR < 300 lines diff (excluding tests)

## References

- ADR-008
- Issue #51
- scripts/host/setup.ps1