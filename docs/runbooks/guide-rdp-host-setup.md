---
id: "guide-rdp-host-setup"
type: runbook
status: active
tags: [operations, rdp, windows, host, tailscale]
created: "2026-02-26"
owner: manu
---

# Guide: RDP Host Configuration for ts-bridge

> How to configure a Windows host machine to accept RDP connections over Tailscale from ts-bridge clients.

## Prerequisites

| Requirement | Details |
|-------------|---------|
| Windows Edition | **Pro, Enterprise, Education, or Server**. Home cannot host RDP. |
| Tailscale | Installed and connected to the tailnet |
| Admin rights | For initial setup only |

## Automated Setup

Run `scripts/host/setup.ps1` as Administrator. It handles steps 1–5 automatically.

## Manual Steps

### 1. Enable Remote Desktop

```powershell
Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' `
    -Name "fDenyTSConnections" -Value 0
```

### 2. Verify RDP Port

```powershell
(Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp').PortNumber
# Default: 3389
```

### 3. Firewall — Allow RDP from Tailscale Subnet

```powershell
# Enable built-in RDP rules
Enable-NetFirewallRule -DisplayGroup "Remote Desktop"

# Create rule restricted to Tailscale CGNAT range
New-NetFirewallRule -DisplayName "Allow RDP over Tailscale" `
    -Direction Inbound -Protocol TCP -LocalPort 3389 `
    -RemoteAddress 100.64.0.0/10 -Action Allow -Profile Private
```

Restricting to `100.64.0.0/10` ensures RDP is only accessible via Tailscale, not the local LAN.

### 4. Tailscale Configuration

```powershell
tailscale up --unattended    # Stay connected without user logged in
tailscale ip -4              # Note IP for client .env
```

Disable key expiry for the host in the [admin console](https://login.tailscale.com/admin/machines).

### 5. Network Profile

Tailscale adapter must be "Private" (usually automatic):

```powershell
Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" } |
    Set-NetConnectionProfile -NetworkCategory Private
```

## Authentication

NLA (Network Level Authentication) is on by default — keep it enabled.

### What Works

| Auth Method | Works? | Notes |
|-------------|--------|-------|
| Local account + password | Yes | Simplest approach |
| Microsoft account + password | Yes | Username: `MicrosoftAccount\user@outlook.com` |
| Domain account (AD) | Yes | Requires DC reachable from host |

### What Does NOT Work

| Auth Method | Why | Fix |
|-------------|-----|-----|
| Passwordless Microsoft account | RDP can't use Hello/FIDO/Authenticator | Set traditional password at account.microsoft.com |
| Windows Hello PIN | PIN is device-local, not remote-capable | Use password |
| Blank password | Windows blocks remote login with empty passwords | Set a password |

### CredSSP Gotcha

If client and host have different Windows Update levels, CredSSP "Encryption Oracle Remediation" errors can occur. Patch both machines to the same level.

## Tailscale ACLs

Default ACLs allow all traffic. If custom ACLs are in use, ensure ts-bridge ephemeral nodes can reach RDP:

```json
{
  "action": "accept",
  "src": ["autogroup:members"],
  "dst": ["host-machine:3389"],
  "proto": "tcp"
}
```

If using tagged auth keys (e.g., `tag:bridge`), the source must match the tag.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "Home edition doesn't support Remote Desktop" | Windows Home | Upgrade to Pro |
| Connection refused | Firewall blocking | Create explicit rule for `100.64.0.0/10` |
| "CredSSP encryption oracle" | Patch mismatch | Update both machines |
| Connection drops after reboot | Tailscale not a service | `Get-Service Tailscale`, ensure Running |
| "Credentials did not work" | Passwordless account | Set traditional password |
| Third-party AV blocking | AV conflicts with Tailscale WFP | Add Tailscale exception |
| RDP works locally, not over Tailscale | ACLs restricting | Check ACL rules |
| Host disappears from tailnet | Key expired | Disable key expiry in admin console |

## References

- [Tailscale: Access remote desktops using Windows RDP](https://tailscale.com/docs/solutions/access-remote-desktops-using-windows-rdp)
- [Tailscale: Secure a Windows RDP server](https://tailscale.com/kb/1095/secure-rdp-windows)
- [Tailscale: Firewall ports](https://tailscale.com/kb/1082/firewall-ports)
