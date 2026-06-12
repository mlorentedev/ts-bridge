[![CI](https://github.com/mlorentedev/ts-bridge/actions/workflows/ci.yml/badge.svg)](https://github.com/mlorentedev/ts-bridge/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/mlorentedev/ts-bridge/branch/master/graph/badge.svg)](https://codecov.io/gh/mlorentedev/ts-bridge)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mlorentedev/ts-bridge)](https://go.dev/)
[![Docs](https://img.shields.io/badge/docs-live-brightgreen)](https://mlorentedev.github.io/ts-bridge/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

# ts-bridge

On-demand Tailscale TCP bridge for non-admin machines. Connect to remote resources securely from locked-down environments.

## The Problem

Working with secure networks often requires VPNs like Tailscale. However, native Tailscale clients require administrator privileges to install and create persistent network interfaces. In many enterprise, corporate, or locked-down environments, users do not have admin rights on their client machines, completely blocking access to critical remote resources via Tailscale.

## The Solution

ts-bridge runs a full, standalone Tailscale node purely in userspace using `tsnet`. It acts as a local proxy, forwarding TCP traffic (like RDP, SSH, or HTTP) through the encrypted mesh network.

| | Native Tailscale | ts-bridge |
|---|---|---|
| **Admin rights on client** | Required | **None needed** |
| **Kernel footprint** | persistent TUN/TAP | **Zero** (userspace) |
| **Installation** | System package | **Portable binary** |
| **Node persistence** | Remains on tailnet | **Ephemeral** (auto-deletes) |

## Quick Install

### 1. Client Machine (No Admin)

Download the binary from [Releases](https://github.com/mlorentedev/ts-bridge/releases) and create a `.env` file:

```env
TS_AUTHKEY=tskey-auth-kXXXXXXXXX   # From Tailscale admin panel
TS_TARGET=100.82.151.104:3389       # Host's Tailscale IP + RDP port
```

Then run the CLI:

```bash
# Run with .env config
ts-bridge connect

# Or run with inline flags (overrides .env)
ts-bridge connect --target 100.82.151.104:3389 --auth-key tskey-auth-xxxxx

# Interactive setup wizard
ts-bridge init
```

### 2. Host Setup (Admin)

Ensure Tailscale is running on the target machine and RDP is enabled:

```powershell
# Configure host for RDP (Windows, requires admin)
ts-bridge host setup

# Verify host readiness (read-only)
ts-bridge host check
```

> **Note:** The old `scripts/client/` launchers (`run.sh`, `run.ps1`, `bootstrap.{sh,ps1}`) have been removed. Use the CLI binary directly — it reads `.env` automatically.

## What You Get

| Feature | Description |
|---|---|
| **Zero-Admin VPN** | Connect from heavily restricted laptops without filing an IT ticket. |
| **Professional CLI** | Cobra-based subcommands: `connect`, `init`, `status`, `host`. Full `--help` and autocomplete. |
| **Headscale Support** | Compatible with open-source control planes (via `TS_CONTROL_URL`). |
| **Multi-Instance** | Run multiple bridges concurrently to connect to different machines. |
| **Ephemeral by Default** | Leaves no trace. The node is automatically removed from the network when the bridge closes. |
| **Health & Metrics** | Built-in HTTP health endpoints plus `ts-bridge status` for human-readable summaries. |

## Before/After (The Workflow)

### Before (Native Tailscale on locked-down PC)
```bash
> tailscale up
Error: Administrator privilege is required to install or start the Tailscale service.
```

### After (ts-bridge)
```bash
> ts-bridge connect
  +---------------------------------------+
  |      TAILSCALE BRIDGE                   |
  +---------------------------------------+
  |  Host:   tsb-office-laptop-a1b2c3     |
  |  Local:  127.0.0.1:33389              |
  |  Target: 100.82.151.104:3389          |
  +---------------------------------------+
  Waiting for connections...
```
Connect using the address from the banner's `Local:` line. In auto mode the port is hash-derived from the `33389-34388` range, so it differs per machine/instance:

```bash
# Read the actual port from the banner above (example shows :33389)
mstsc /v:127.0.0.1:33389          # Windows RDP
xfreerdp /v:127.0.0.1:33389       # Linux RDP
ssh -p 33389 user@127.0.0.1       # SSH targets
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `TS_AUTHKEY` | — | **Required**. Tailscale/Headscale auth key (`tskey-*` or `hskey-*`). |
| `TS_TARGET` | — | **Required**. Target IP/hostname and port (e.g., `100.x.x.x:3389`). |
| `TS_LOCAL_ADDR` | `127.0.0.1:33389` | Local bind address. |
| `TS_CONTROL_URL` | — | Custom control plane URL for Headscale. |
| `TS_INSTANCE_NAME` | — | Stable alias for deterministic port selection. |
| `TS_PORT_RANGE` | `33389-34388` | Port range for auto mode. |
| `TS_HEALTH_ADDR` | — | Enable health/metrics HTTP server. |
| `TS_VERBOSE` | `false` | Debug logging. |
| `TS_LOG_FORMAT` | `text` | `text` or `json`. |

For the full configuration reference (all env vars, YAML config, CLI flags), see the [Configuration](https://mlorentedev.github.io/ts-bridge/configuration/) and [CLI Reference](https://mlorentedev.github.io/ts-bridge/cli-reference/) pages.

## Architecture

```text
┌─────────────────────────┐
│ CLIENT (Non-Admin)      │
│ RDP/SSH → :33389        │
│    ↓                    │
│ ts-bridge (userspace)   │
└────┬────────────────────┘
     │ encrypted via WireGuard (DERP/STUN)
┌────▼────────────────────┐
│ HOST (Admin)            │
│ Tailscale (native)      │
│    ↓                    │
│ RDP/SSH Server          │
└─────────────────────────┘
```

## Documentation

Project-bound knowledge lives in [`docs/`](docs/) (docs-as-code):

- [`docs/adr/`](docs/adr/) — Architecture Decision Records
- [`docs/runbooks/`](docs/runbooks/) — operational procedures (deploy, RDP host setup, multi-device ops)
- [`docs/troubleshooting/`](docs/troubleshooting/) — known errors, security audit, release issues
- [`docs/lessons.md`](docs/lessons.md) — accumulated gotchas and post-mortems

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for development setup, testing, and PR guidelines.

## License

MIT — see [LICENSE](LICENSE).
