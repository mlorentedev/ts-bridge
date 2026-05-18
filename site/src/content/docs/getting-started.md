---
title: Getting Started
description: Download, configure, and run ts-bridge on Linux, Windows, or macOS.
---

## Download

Grab the latest release from [GitHub Releases](https://github.com/mlorentedev/ts-bridge/releases). Builds are available for six platforms:

| OS | Architecture | Archive |
|----|-------------|---------|
| Linux | amd64 | `ts-bridge-<version>-linux-amd64.tar.gz` |
| Linux | arm64 | `ts-bridge-<version>-linux-arm64.tar.gz` |
| Windows | amd64 | `ts-bridge-<version>-windows-amd64.zip` |
| Windows | arm64 | `ts-bridge-<version>-windows-arm64.zip` |
| macOS | amd64 | `ts-bridge-<version>-darwin-amd64.tar.gz` |
| macOS | arm64 | `ts-bridge-<version>-darwin-arm64.tar.gz` |

Each archive includes the binary, `.env.example`, launch scripts, and the README.

## Configure

Copy `.env.example` to `.env` and set the two required variables:

```bash
cp .env.example .env
```

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

### Linux / macOS

```bash
./scripts/client/run.sh
```

Or with a bootstrap script that generates `.env` for you:

```bash
./scripts/client/bootstrap.sh \
  --authkey tskey-auth-kXXXXXX \
  --target 100.82.151.104:3389 \
  --instance office-laptop
```

### Windows

```powershell
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1
```

Or bootstrap:

```powershell
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\bootstrap.ps1 `
  -AuthKey tskey-auth-kXXXXXX `
  -Target 100.82.151.104:3389 `
  -Instance office-laptop
```

### Direct binary

```bash
# Build from source
go build -o ts-bridge .

# Run with verbose logging
./ts-bridge -v
```

## Connect

Once ts-bridge is running, it prints a banner with the local port:

```text
  +---------------------------------------+
  |      TAILSCALE BRIDGE v1.4.0          |
  +---------------------------------------+
  |  Host:   tsb-office-laptop-a1b2c3-... |
  |  Local:  127.0.0.1:33412              |
  |  Target: 100.82.151.104:3389          |
  +---------------------------------------+
  Waiting for connections...
```

Point your RDP client at the local address:

```bash
# Linux (FreeRDP)
xfreerdp /v:127.0.0.1:33412 /u:Username /cert:ignore

# Windows (built-in)
mstsc /v:127.0.0.1:33412

# macOS (Microsoft Remote Desktop)
# Add PC -> 127.0.0.1:33412
```

For SSH targets, use any SSH client:

```bash
ssh -p <LOCAL_PORT> user@127.0.0.1
```

## Health endpoint

Set `TS_HEALTH_ADDR` to enable the HTTP health and metrics server:

```bash
TS_HEALTH_ADDR=127.0.0.1:8080
```

```bash
curl http://127.0.0.1:8080/health/live   # {"status":"ok"} -- process alive
curl http://127.0.0.1:8080/health/ready  # {"status":"ok"} -- tsnet tunnel up
curl http://127.0.0.1:8080/metrics       # Connection stats (JSON)
```

## Host setup

The machine you connect **to** (the host) needs Tailscale installed natively with admin rights. The host requires:

- Windows Pro, Enterprise, Education, or Server (Home edition cannot host RDP)
- Remote Desktop enabled in Settings > System > Remote Desktop
- Firewall rule allowing TCP 3389 from the Tailscale subnet (`100.64.0.0/10`)

The included `scripts/host/setup.ps1` automates this on Windows. Run it as Administrator.
