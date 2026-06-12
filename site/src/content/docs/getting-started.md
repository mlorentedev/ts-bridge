---
title: Getting Started
description: Download, configure, and run ts-bridge on Linux, Windows, or macOS.
---

import { Tabs, TabItem } from '@astrojs/starlight/components';

## Download

Grab the latest release from [GitHub Releases](https://github.com/mlorentedev/ts-bridge/releases). Builds are available for six platform/architecture combinations.

<Tabs>
  <TabItem label="Linux (amd64)">

```bash
# Download
curl -LO https://github.com/mlorentedev/ts-bridge/releases/latest/download/ts-bridge-linux-amd64.tar.gz

# Extract
tar -xzf ts-bridge-linux-amd64.tar.gz
cd ts-bridge-linux-amd64

# (Optional) Install to PATH
sudo mv ts-bridge /usr/local/bin/
```

  </TabItem>
  <TabItem label="Linux (arm64)">

```bash
# Download
curl -LO https://github.com/mlorentedev/ts-bridge/releases/latest/download/ts-bridge-linux-arm64.tar.gz

# Extract
tar -xzf ts-bridge-linux-arm64.tar.gz
cd ts-bridge-linux-arm64

# (Optional) Install to PATH
sudo mv ts-bridge /usr/local/bin/
```

  </TabItem>
  <TabItem label="Windows (amd64)">

```powershell
# Download and extract manually from GitHub Releases
# OR use PowerShell:
Invoke-WebRequest -Uri "https://github.com/mlorentedev/ts-bridge/releases/latest/download/ts-bridge-windows-amd64.zip" -OutFile "ts-bridge.zip"
Expand-Archive -Path "ts-bridge.zip" -DestinationPath "ts-bridge"
cd ts-bridge

# (Optional) Add to PATH
$env:Path += ";$pwd"
```

  </TabItem>
  <TabItem label="macOS (amd64)">

```bash
# Download
curl -LO https://github.com/mlorentedev/ts-bridge/releases/latest/download/ts-bridge-darwin-amd64.tar.gz

# Extract
tar -xzf ts-bridge-darwin-amd64.tar.gz
cd ts-bridge-darwin-amd64

# (Optional) Install to PATH
sudo mv ts-bridge /usr/local/bin/
```

  </TabItem>
  <TabItem label="macOS (arm64)">

```bash
# Download
curl -LO https://github.com/mlorentedev/ts-bridge/releases/latest/download/ts-bridge-darwin-arm64.tar.gz

# Extract
tar -xzf ts-bridge-darwin-arm64.tar.gz
cd ts-bridge-darwin-arm64

# (Optional) Install to PATH
sudo mv ts-bridge /usr/local/bin/
```

  </TabItem>
</Tabs>

## Configure

Copy `.env.example` to `.env` and set the two required variables:

<Tabs syncKey="os">
  <TabItem label="Linux / macOS">

```bash
cp .env.example .env
```

  </TabItem>
  <TabItem label="Windows">

```powershell
copy .env.example .env
```

  </TabItem>
</Tabs>

Edit the `.env` file with your auth key and target:

```bash
# .env
TS_AUTHKEY=tskey-auth-kXXXXXXXXX   # From Tailscale admin console
TS_TARGET=100.82.151.104:3389       # Host Tailscale IP + port
```

For Headscale, use `hskey-auth-*` keys and set `TS_CONTROL_URL`:

```bash
# .env (Headscale)
TS_AUTHKEY=hskey-auth-xxxxx
TS_TARGET=100.64.0.5:3389
TS_CONTROL_URL=https://vpn.example.com
```

### Required variables

| Variable | Description | Example |
|----------|-------------|---------|
| `TS_AUTHKEY` | Auth key. Tailscale: [generate here](https://login.tailscale.com/admin/settings/keys). Headscale: `headscale preauthkeys create`. Prefix: `tskey-` or `hskey-`. | `tskey-auth-kXXXXXX` |
| `TS_TARGET` | Host address on the mesh network. Supports IP or MagicDNS hostname. | `100.82.151.104:3389` |

### Optional variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TS_LOCAL_ADDR` | `127.0.0.1:33389` | Local bind address. Auto-derived in auto mode when unset. |
| `TS_CONTROL_URL` | _(Tailscale default)_ | Custom control plane URL for self-hosted Headscale. |
| `TS_HOSTNAME` | `ts-bridge` | Node name in the admin console. Auto-generated per run in auto mode. |
| `TS_STATE_DIR` | `./ts-state` | Directory for node state. Created with `0700` permissions. Ephemeral temp dir in auto mode. |
| `TS_AUTO_INSTANCE` | `true` | Auto mode toggle. Set `false` to disable auto behavior. |
| `TS_MANUAL_MODE` | `false` | Force legacy persistent mode. Takes precedence over `TS_AUTO_INSTANCE`. |
| `TS_INSTANCE_NAME` | _(empty)_ | Stable instance alias for deterministic local port selection. |
| `TS_PORT_RANGE` | `33389-34388` | Port range for auto mode (`START-END`). |
| `TS_TIMEOUT` | `30s` | Timeout for tsnet initialization (control-plane handshake). Go duration format. |
| `TS_DIAL_TIMEOUT` | `5s` | Per-connection target dial timeout, distinct from `TS_TIMEOUT`. Keeps stuck dials from holding a slot across retries. Go duration format. _(v1.8.0+)_ |
| `TS_DRAIN_TIMEOUT` | `15s` | Timeout for graceful drain of active connections on shutdown. Go duration format. |
| `TS_MAX_CONNECTIONS` | `1000` | Maximum concurrent connections before rejecting new ones. |
| `TS_IDLE_TIMEOUT` | _(disabled)_ | Close connections after this period of no traffic in either direction. Go duration format (e.g. `30m`). Default `0` disables. Useful for reclaiming slots from abandoned RDP sessions. _(v1.6.0+)_ |
| `TS_DIAL_RETRIES` | `3` | Maximum retries for transient target dial failures. `0` disables retry. _(v1.7.0+)_ |
| `TS_DIAL_BACKOFF_BASE` | `1s` | Base backoff for dial retries (multiplied by `2^attempt`, plus jitter). Go duration format. _(v1.7.0+)_ |
| `TS_DIAL_BACKOFF_MAX` | `30s` | Cap on backoff per retry attempt. Must be ≥ `TS_DIAL_BACKOFF_BASE`. _(v1.7.0+)_ |
| `TS_HEALTH_ADDR` | _(disabled)_ | Address for health/metrics HTTP server. |
| `TS_VERBOSE` | `false` | Enable debug logging. Also available as `-v` flag. |
| `TS_LOG_FORMAT` | `text` | Log output format (`text` or `json`). |

## Run

<Tabs syncKey="os">
  <TabItem label="Linux / macOS">

```bash
# Run with .env config
./ts-bridge connect

# Run with verbose logging
./ts-bridge connect -v

# Run with all flags inline (overrides .env)
./ts-bridge connect --target 100.82.151.104:3389 --auth-key tskey-auth-kXXXXXXXXX

# Interactive setup wizard
./ts-bridge init
```

  </TabItem>
  <TabItem label="Windows">

```powershell
# Run with .env config
.\ts-bridge.exe connect

# Run with verbose logging
.\ts-bridge.exe connect -v

# Run with all flags inline (overrides .env)
.\ts-bridge.exe connect --target 100.82.151.104:3389 --auth-key tskey-auth-kXXXXXXXXX

# Interactive setup wizard
.\ts-bridge.exe init
```

  </TabItem>
</Tabs>

### Config precedence

1. CLI flags (highest)
2. Environment variables (`TS_*`)
3. YAML config file (`--config`)
4. Built-in defaults (lowest)

## Connect

Once ts-bridge is running, it prints the local port. Connect your RDP client:

<Tabs syncKey="os">
  <TabItem label="Linux">

```bash
# FreeRDP
xfreerdp /v:127.0.0.1:<LOCAL_PORT> /u:Username /cert:ignore

# Remmina
remmina --new-protocol RDP --server 127.0.0.1:<LOCAL_PORT>

# SSH
ssh -p <LOCAL_PORT> user@127.0.0.1
```

  </TabItem>
  <TabItem label="Windows">

```batch
:: Built-in Remote Desktop Connection
mstsc /v:127.0.0.1:<LOCAL_PORT>

:: PowerShell
Start-Process mstsc -ArgumentList "/v:127.0.0.1:<LOCAL_PORT>"
```

  </TabItem>
  <TabItem label="macOS">

```bash
# Microsoft Remote Desktop
# Open app → Add PC → 127.0.0.1:<LOCAL_PORT>

# SSH
ssh -p <LOCAL_PORT> user@127.0.0.1
```

  </TabItem>
</Tabs>

For SSH targets, use any SSH client on any platform:

```bash
ssh -p <LOCAL_PORT> user@127.0.0.1
```

## Health endpoint

Set `TS_HEALTH_ADDR` to enable the HTTP health and metrics server:

```bash
TS_HEALTH_ADDR=127.0.0.1:9090
```

```bash
curl http://127.0.0.1:9090/health/live   # {"status":"ok"} -- process alive
curl http://127.0.0.1:9090/health/ready  # {"status":"ok"} -- tsnet tunnel up
curl http://127.0.0.1:9090/metrics       # Connection stats (JSON)
```

Or use the `status` subcommand for a human-readable summary:

```bash
# One-shot summary
ts-bridge status

# Continuous watch
ts-bridge status --watch --interval 2s

# Raw JSON metrics
ts-bridge status --json
```

## Host setup

The machine you connect **to** (the host) needs Tailscale installed natively with admin rights.

<Tabs syncKey="os">
  <TabItem label="Windows">

### Requirements
- Windows Pro, Enterprise, Education, or Server (Home edition cannot host RDP)
- Remote Desktop enabled in Settings → System → Remote Desktop
- Administrator privileges

### Automated setup

```powershell
# Run as Administrator
ts-bridge host setup
```

This configures:
1. Tailscale unattended mode (`tailscale up --unattended`)
2. Tailscale service to auto-start
3. UPnP services (SSDP, UPnP Device Host)
4. Tailscale network profile set to Private
5. RDP enabled via registry
6. Windows Firewall rule for RDP on TCP 3389
7. (Optional) Power settings — sleep disabled

### Verify

```powershell
# Read-only check
ts-bridge host check

# JSON output for automation
ts-bridge host check --json
```

### Custom firewall rule

```powershell
ts-bridge host setup --firewall-rule "MyCustomRDPRule"
```

  </TabItem>
  <TabItem label="Linux">

### Requirements
- `xrdp` installed (`sudo apt install xrdp`)
- `ufw` or `iptables` for firewall
- Root/sudo privileges

### Automated setup

```bash
sudo ts-bridge host setup
```

This configures:
1. Detects `xrdp` installation
2. Opens firewall for RDP port (3389) via `ufw` or `iptables`
3. Prints the Tailscale IP for client configuration

### Verify

```bash
# Read-only check
ts-bridge host check

# JSON output for automation
ts-bridge host check --json
```

  </TabItem>
  <TabItem label="macOS">

Host setup is **not applicable** on macOS. macOS machines act as clients only — use `ts-bridge connect` to connect to a remote Windows or Linux host.

  </TabItem>
</Tabs>

### Manual host setup (Windows)

For hosts that cannot run the binary, use the PowerShell setup script:

```powershell
# Run as Administrator
.\scripts\host\setup.ps1
```