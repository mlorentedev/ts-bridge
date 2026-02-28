# ts-bridge

TCP bridge for tunneling connections through Tailscale's encrypted mesh network without requiring administrator privileges on the client machine.

## Overview

Connect via RDP/SSH from a **non-admin machine** to an **admin machine** through restrictive firewalls using Tailscale's userspace networking.

| Machine | Admin Rights | Tailscale | Role |
|---------|--------------|-----------|------|
| **Client** | No | Not installed (uses tsnet) | Initiates connection |
| **Host** | Yes | Installed natively | Receives connection |

## Why ts-bridge?

| Requirement | Native Tailscale | ts-bridge |
|-------------|------------------|-----------|
| Admin rights on client | **Yes** | **No** |
| Kernel module | Yes | No (userspace) |
| Software installation | Required | Portable binary |
| Leaves traces | Yes | No (ephemeral) |
| Works on locked-down machines | No | **Yes** |

## Control Plane Support

ts-bridge works with both **Tailscale SaaS** (default) and **self-hosted [Headscale](https://github.com/juanfont/headscale)**.

| | Tailscale SaaS | Headscale |
|---|---|---|
| **Setup** | Default, no extra config | Set `TS_CONTROL_URL` |
| **Auth key prefix** | `tskey-auth-*` | `hskey-auth-*` |
| **Ephemeral cleanup** | Automatic | Requires `--ephemeral` flag on the pre-auth key |
| **Minimum version** | Any | Headscale v0.28.0+ requires tsnet >= v1.74 (ts-bridge v1.3.0+) |

### Headscale Quick Setup

```bash
# .env
TS_CONTROL_URL=https://vpn.example.com
TS_AUTHKEY=hskey-auth-xxxxx
TS_TARGET=100.64.0.5:3389
```

Generate the auth key on the Headscale server:
```bash
headscale preauthkeys create --user <ID> --reusable --ephemeral --expiration 8760h
```

> **Important:** The `--ephemeral` flag must be on the **pre-auth key**, not just in ts-bridge config.
> Without it, nodes persist as offline entries after disconnect.

## Quick Start

### 1. Host Machine (Admin Rights Required)

Install [Tailscale](https://tailscale.com/download) normally, then run the setup script:

```powershell
# Run as Administrator
cd scripts\host
PowerShell -ExecutionPolicy Bypass -File .\setup.ps1
```

Note the Tailscale IP shown (e.g., `100.82.151.104`). For Headscale, use `tailscale up --login-server=https://vpn.example.com`. See [Host Setup Guide](#host-setup-guide) for manual steps and troubleshooting.

### 2. Client Machine (No Admin Rights)

Download from [Releases](https://github.com/mlorentedev/ts-bridge/releases), extract, and configure:

```bash
tar -xzf ts-bridge-linux-amd64.tar.gz
cd ts-bridge-linux-amd64
cp .env.example .env
```

Edit `.env` — only two variables are required:

```bash
TS_AUTHKEY=tskey-auth-kXXXXXXXXX   # From Tailscale admin or Headscale (hskey-auth-*)
TS_TARGET=100.82.151.104:3389       # Host's Tailscale/Headscale IP + RDP port
```

### 3. Run

```bash
# Linux/macOS
./scripts/client/run.sh

# Windows
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1
```

### 4. Connect

```bash
# Linux (xfreerdp)
xfreerdp /v:127.0.0.1:33389 /u:Username /cert:ignore

# Windows (mstsc)
mstsc /v:127.0.0.1:33389

# macOS (Microsoft Remote Desktop)
# Add PC → 127.0.0.1:33389
```

## Host Setup Guide

The host machine (the one you connect **to**) needs specific configuration to accept RDP connections over Tailscale.

### Requirements

| Requirement | Details |
|-------------|---------|
| **Windows Edition** | Pro, Enterprise, Education, or Server. **Home edition cannot host RDP.** |
| **Tailscale** | Installed and connected to the tailnet |
| **Admin rights** | Needed for initial setup only |

### Step 1: Enable Remote Desktop

Settings > System > Remote Desktop > Toggle **On**.

Or via PowerShell (Admin):
```powershell
Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' `
    -Name "fDenyTSConnections" -Value 0
```

### Step 2: Configure Authentication

**Network Level Authentication (NLA)** is enabled by default — keep it on.

The account you connect with **must have a traditional password set**. The following do NOT work for RDP:

| Auth Method | Works? | Fix |
|-------------|--------|-----|
| Local account + password | Yes | — |
| Microsoft account + password | Yes | Username: `MicrosoftAccount\user@outlook.com` |
| Microsoft account (passwordless) | **No** | Set a password at account.microsoft.com > Security |
| Windows Hello PIN | **No** | Use password instead |
| Blank/empty password | **No** | Set a password on the account |

### Step 3: Configure Firewall

The automated `setup.ps1` handles this. For manual setup:

```powershell
# Enable built-in RDP rules
Enable-NetFirewallRule -DisplayGroup "Remote Desktop"

# Restrict RDP to Tailscale subnet only (recommended)
New-NetFirewallRule -DisplayName "Allow RDP over Tailscale" `
    -Direction Inbound -Protocol TCP -LocalPort 3389 `
    -RemoteAddress 100.64.0.0/10 -Action Allow -Profile Private
```

### Step 4: Tailscale Configuration

```powershell
# Enable unattended mode (stays connected without user logged in)
tailscale up --unattended

# Verify Tailscale IP
tailscale ip -4
```

**Recommended:** Disable key expiry for the host machine in the [Tailscale admin console](https://login.tailscale.com/admin/machines) so it doesn't silently drop off the tailnet.

### Common Host Issues

| Symptom | Cause | Fix |
|---------|-------|-----|
| "Your Home edition doesn't support Remote Desktop" | Windows Home | Upgrade to Pro or use a different host |
| RDP connection refused | Firewall blocking Tailscale subnet | Create explicit rule for `100.64.0.0/10` on TCP 3389 |
| "CredSSP encryption oracle" error | Mismatched Windows Update levels | Patch both client and host to latest |
| Connection drops after host reboot | Tailscale not running as service | Verify Tailscale service: `Get-Service Tailscale` |
| "The credentials did not work" | Passwordless Microsoft account | Set a traditional password |
| Works locally but not over Tailscale | Tailscale ACLs restricting access | Check [ACL rules](https://login.tailscale.com/admin/acls) allow TCP 3389 |
| Third-party antivirus blocking | AV conflicts with Tailscale WFP rules | Add Tailscale exception in AV settings |

## Configuration Reference

Create `.env` from `.env.example`. Only `TS_AUTHKEY` and `TS_TARGET` are required — everything else has sensible defaults.

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `TS_AUTHKEY` | Auth key. Tailscale SaaS: [generate here](https://login.tailscale.com/admin/settings/keys). Headscale: `headscale preauthkeys create`. Prefix: `tskey-` or `hskey-`. | `tskey-auth-kXXXXXX` |
| `TS_TARGET` | Host address on the mesh network. Supports IP or MagicDNS hostname. | `100.82.151.104:3389` or `my-desktop:3389` |

### Optional

| Variable | Default | Description | Example |
|----------|---------|-------------|---------|
| `TS_LOCAL_ADDR` | `127.0.0.1:33389` | Local address to bind the bridge listener. Change if the default port conflicts. | `127.0.0.1:43389` |
| `TS_CONTROL_URL` | _(Tailscale default)_ | Custom control plane URL. Set this to use a self-hosted [Headscale](https://github.com/juanfont/headscale) server. | `https://vpn.example.com` |
| `TS_HOSTNAME` | `ts-bridge` | Node name shown in the Tailscale admin console. | `bridge-workpc` |
| `TS_STATE_DIR` | `./ts-state` | Directory for Tailscale node state. Auto-created with `0700` permissions. | `/tmp/ts-bridge-state` |
| `TS_TIMEOUT` | `30s` | Timeout for Tailscale initialization and dial. Go duration format. | `1m`, `45s` |
| `TS_MAX_CONNECTIONS` | `1000` | Maximum concurrent connections before rejecting new ones. | `50` |
| `TS_HEALTH_ADDR` | _(disabled)_ | Address for health/metrics HTTP server. | `127.0.0.1:8080` |
| `TS_VERBOSE` | `false` | Enable debug logging. Also available as `-v` flag. | `true` |
| `TS_LOG_FORMAT` | `text` | Log output format. | `text` or `json` |

### Health Endpoint

When `TS_HEALTH_ADDR` is set:

```bash
curl http://127.0.0.1:8080/health/live   # {"status":"ok"} — process alive
curl http://127.0.0.1:8080/health/ready  # {"status":"ok"} — tsnet tunnel up
curl http://127.0.0.1:8080/metrics       # Connection stats (JSON)
```

### Command Line

```bash
./ts-bridge -version    # Show version
./ts-bridge -v          # Verbose logging (same as TS_VERBOSE=true)
```

## How It Works

```text
┌─────────────────────────────────────────────────────────────────────┐
│  CLIENT (Non-Admin)                                                 │
│                                                                     │
│   RDP Client ──▶ ts-bridge ──▶ tsnet (userspace WireGuard)         │
│   127.0.0.1:33389              No admin required                    │
│                                                                     │
│   ┌─────────────────────────────────────────────────┐               │
│   │ Firewall: UDP blocked, HTTPS allowed            │◀── Tunnels   │
│   └─────────────────────────────────────────────────┘    via DERP   │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              │ Tailscale Network (WireGuard encrypted)
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│  HOST (Admin)                                                       │
│                                                                     │
│   Tailscale (Native) ──▶ RDP Server :3389                          │
│   100.x.x.x                                                         │
└─────────────────────────────────────────────────────────────────────┘
```

1. ts-bridge creates ephemeral node via [tsnet](https://pkg.go.dev/tailscale.com/tsnet) on Tailscale SaaS or Headscale
2. WireGuard runs in userspace (no kernel module, no admin)
3. If UDP blocked, uses DERP relay over HTTPS
4. All traffic end-to-end encrypted (WireGuard + RDP TLS)
5. Node auto-deletes on exit (Headscale: requires `--ephemeral` pre-auth key)

## Limitations

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| TCP only | No UDP (VoIP, games) | Use for RDP, SSH, HTTP |
| DERP latency | +50-200ms when relayed | Acceptable for RDP |
| Auth key expiry | Default 90 days (Tailscale), configurable (Headscale) | Use long-lived keys or `--expiration 8760h` on Headscale |
| Single target | One host per instance | Run multiple instances with different `TS_TARGET` |
| Windows Home | Cannot host RDP | Use Windows Pro/Enterprise on host |

## Security

- **No admin footprint**: Runs entirely in userspace
- **Ephemeral nodes**: Auto-delete from Tailscale on exit
- **E2E encryption**: WireGuard encryption even through DERP relay
- **Local only**: Binds to `127.0.0.1` by default
- **Secure state**: Directory created with `0700` permissions

## Documentation

- [Contributing](CONTRIBUTING.md) - Development setup, testing, releases

## Support

For questions, bugs, or feature requests, please [open an issue](https://github.com/mlorentedev/ts-bridge/issues).

## License

[MIT](LICENSE)
