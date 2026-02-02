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

## Installation

### Host Machine (Admin Rights Required)

1. Install [Tailscale](https://tailscale.com/download) normally
2. Run the setup script:

```powershell
# Run as Administrator
cd scripts\host
PowerShell -ExecutionPolicy Bypass -File .\setup.ps1
```

Note the Tailscale IP shown (e.g., `100.82.151.104`).

### Client Machine (No Admin Rights)

1. Download from [Releases](https://github.com/mlorentedev/ts-bridge/releases)
2. Extract and configure:

```bash
tar -xzf ts-bridge-linux-amd64.tar.gz
cd ts-bridge-linux-amd64
cp .env.example .env
# Edit .env: set TS_AUTHKEY and TS_TARGET
```

3. Run:

```bash
# Linux/macOS
./scripts/client/run.sh

# Windows
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1
```

## Usage

### Connect via RDP

```bash
# Linux
xfreerdp /v:127.0.0.1:33389 /u:Username /cert:ignore

# Windows
mstsc /v:127.0.0.1:33389
```

### Command Line

```bash
./ts-bridge -version    # Show version
./ts-bridge -v          # Verbose logging
```

## Configuration

Create `.env` from `.env.example`:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TS_AUTHKEY` | Yes | - | Tailscale auth key ([generate](https://login.tailscale.com/admin/settings/keys)) |
| `TS_TARGET` | Yes | - | Host IP:PORT (e.g., `100.x.x.x:3389`) |
| `TS_LOCAL_ADDR` | No | `127.0.0.1:33389` | Local bind address |
| `TS_HOSTNAME` | No | `ts-bridge` | Node name in Tailscale |
| `TS_STATE_DIR` | No | `./ts-state` | State directory |
| `TS_TIMEOUT` | No | `30s` | Connection timeout |
| `TS_MAX_CONNECTIONS` | No | `1000` | Max concurrent connections |
| `TS_HEALTH_ADDR` | No | (disabled) | Health endpoint (e.g., `127.0.0.1:8080`) |
| `TS_VERBOSE` | No | `false` | Enable debug logging |
| `TS_LOG_FORMAT` | No | `text` | Log format (`text` or `json`) |

### Health Endpoint

When `TS_HEALTH_ADDR` is set:

```bash
curl http://127.0.0.1:8080/health   # {"status":"ok"}
curl http://127.0.0.1:8080/metrics  # Connection stats
```

## How It Works

```text
┌─────────────────────────────────────────────────────────────────────┐
│  CLIENT (Non-Admin)                                                 │
│                                                                     │
│   RDP Client ──▶ ts-bridge ──▶ tsnet (userspace WireGuard)         │
│   127.0.0.1:33389              No admin required                    │
│                                                                     │
│   ┌─────────────────────────────────────────────┐                   │
│   │ Firewall: UDP blocked, HTTPS allowed        │◀── Tunnels via   │
│   └─────────────────────────────────────────────┘    DERP relay    │
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

1. ts-bridge creates ephemeral Tailscale node via [tsnet](https://pkg.go.dev/tailscale.com/tsnet)
2. WireGuard runs in userspace (no kernel module, no admin)
3. If UDP blocked, uses DERP relay over HTTPS
4. All traffic end-to-end encrypted
5. Node auto-deletes from Tailscale on exit

## Limitations

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| TCP only | No UDP (VoIP, games) | Use for RDP, SSH, HTTP |
| DERP latency | +50-200ms when relayed | Acceptable for RDP |
| Auth key expiry | Default 90 days | Use long-lived keys |
| Single target | One host per instance | Run multiple instances |

## Security

- **No admin footprint**: Runs entirely in userspace
- **Ephemeral nodes**: Auto-delete from Tailscale
- **E2E encryption**: WireGuard encryption even through DERP
- **Local only**: Binds to 127.0.0.1 by default
- **Secure state**: Directory created with 0700 permissions

## Documentation

- [Troubleshooting Guide](docs/TROUBLESHOOTING.md) - Common issues and solutions
- [Operations Guide](docs/OPERATIONS.md) - Production deployment
- [Contributing](CONTRIBUTING.md) - Development setup, testing, releases

## Support

For questions, bugs, or feature requests, please [open an issue](https://github.com/mlorentedev/ts-bridge/issues).

## License

[MIT](LICENSE)
