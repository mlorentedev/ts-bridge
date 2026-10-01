[![CI](https://github.com/mlorentedev/ts-bridge/actions/workflows/ci.yml/badge.svg)](https://github.com/mlorentedev/ts-bridge/actions/workflows/ci.yml)
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
TS_AUTHKEY=tskey-auth-KEYID-SECRET   # From Tailscale admin panel
TS_TARGET=my-desktop:3389           # Host's MagicDNS name + RDP port
```

Then run the CLI:

```bash
# Run with .env config
ts-bridge connect

# Secure alternative: read auth key from a 0600 file (avoids child-process env exposure)
ts-bridge connect --target my-desktop:3389 --auth-key-file /path/to/authkey

# Interactive setup wizard
ts-bridge init

# Secure non-interactive setup
ts-bridge init --auth-key-file /path/to/authkey --target my-desktop:3389

# See all options
ts-bridge --help
```

> **Security Note:** Keys in environment variables or `.env` files are readable by child processes, and `--auth-key` is visible in the process list. Prefer managed onboarding:
>
> ```bash
> ts-bridge auth set --profile office
> ts-bridge connect --profile office
> ```
>
> The key is entered through a masked prompt and stored with owner-only permissions. `--auth-key-file` remains available as an explicit override.

### Dynamic mesh and private browser access

Use one loopback SOCKS5 listener for SSH, kubectl, curl, or other clients that
need multiple mesh destinations:

```powershell
.\ts-bridge.exe connect `
  --auth-key-file C:\Users\user\.ts-bridge\authkey `
  --socks5 127.0.0.1:1080
```

For a private HTTPS service whose public DNS is unavailable or blocked, use an
allow-listed route and an isolated Edge profile:

```powershell
.\ts-bridge.exe browser `
  --config C:\Users\user\.ts-bridge\apps-browser.yml `
  --auth-key-file C:\Users\user\.ts-bridge\authkey `
  --url https://forge.example.internal/
```

Browser mode preserves the original TLS SNI/Host, proxies only configured
origins, and leaves Windows hosts, DNS, system proxy, and the normal browser
profile unchanged. See
[`docs/runbooks/guide-multi-target-socks5.md`](docs/runbooks/guide-multi-target-socks5.md).

**Using a named profile (recommended when the host uses a non-default port):**

When the host runs `ts-bridge host setup` or `ts-bridge host check`, the output includes a shareable descriptor:

```
  Shareable descriptor:
  tsb://acemagic-office:45000?cp=saas
  (import with: ts-bridge import home "tsb://acemagic-office:45000?cp=saas")
```

Import it once, then connect by name:

```bash
ts-bridge import home "tsb://acemagic-office:45000?cp=saas"
ts-bridge connect --profile home    # resolves target+port automatically
```

`--profile` is additive: if `TS_TARGET` or `--target` is also set, it wins over the profile.
When the profile has a managed `credential` reference, `connect --profile`
loads it automatically; explicit key sources keep higher precedence.

### 2. Host Setup (Admin)

Ensure Tailscale is running on the target machine and RDP is enabled. Configure the host in three steps:

```powershell
# 1. Interactive setup wizard (creates .env)
ts-bridge host init

# 2. Configure host for RDP (Windows, requires admin)
ts-bridge host setup

# 3. Verify host readiness (read-only)
ts-bridge host check
```

For automation, use `--json`: `ts-bridge host setup --json` and `ts-bridge host check --json`.

> **Note:** The old `scripts/client/` launchers (`run.sh`, `run.ps1`, `bootstrap.{sh,ps1}`) have been removed. Use the CLI binary directly — it reads `.env` automatically.

## What You Get

| Feature | Description |
|---|---|
| **Zero-Admin VPN** | Connect from heavily restricted laptops without filing an IT ticket. |
| **Professional CLI** | Cobra-based subcommands: `browser`, `connect`, `import`, `discover`, `init`, `status`, `host`. Full `--help` and autocomplete. |
| **Dynamic Mesh Proxy** | One loopback SOCKS5 listener reaches multiple permitted mesh destinations without a TUN adapter. |
| **Private Browser Profile** | Launches isolated Edge with a local PAC and allow-listed origin-to-mesh routes. |
| **Headscale Support** | Compatible with open-source control planes (via `TS_CONTROL_URL`). |
| **Multi-Instance** | Run multiple bridges concurrently to connect to different machines. |
| **Ephemeral by Default** | Leaves no trace. The node is automatically removed from the network when the bridge closes. |
| **Health & Metrics** | Built-in HTTP health endpoints plus `ts-bridge status` for human-readable summaries. |

## MagicDNS

ts-bridge works with MagicDNS out of the box — no configuration changes needed.

### With MagicDNS (recommended)

If MagicDNS is enabled in your tailnet, use your device's name instead of its Tailscale IP:

```env
TS_TARGET=my-desktop:3389
```

This is more readable, resilient to IP changes, and works the same way whether you're on Tailscale SaaS or Headscale.

### Without MagicDNS (IP fallback)

If MagicDNS is not available (e.g., custom control plane without MagicDNS configured), use the Tailscale IP directly:

```env
TS_TARGET=100.82.151.104:3389
```

### Headscale note

When using a self-hosted Headscale instance, MagicDNS must be explicitly enabled in the Headscale configuration. Without it, fall back to IP-based targets.

### Headscale control plane blocked by the local network

If the local network DNS-sinkholes or TLS-inspects the Headscale control plane
but an SSH bastion can reach it, bootstrap the control connection through the
system OpenSSH client:

```bash
ts-bridge connect \
  --control-url https://vpn.example.com \
  --bootstrap-ssh deployer@bastion.example.com \
  --auth-key-file /path/to/headscale-authkey \
  --target mesh-host:3389
```

ts-bridge starts a loopback SOCKS proxy, resolves the control-plane hostname
from the SSH server, and preserves the original hostname for TLS verification
without editing the hosts file. SSH host-key verification remains enabled.
Use `--bootstrap-socks-addr` only when the default `127.0.0.1:1055` is occupied.

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
  |  Target: my-desktop:3389              |
  +---------------------------------------+
  Waiting for connections...
```
Connect using the address from the banner's `Local:` line. By default the bridge listens on `127.0.0.1:33389`:

```bash
mstsc /v:127.0.0.1:33389          # Windows RDP
xfreerdp /v:127.0.0.1:33389       # Linux RDP
ssh -p 33389 user@127.0.0.1       # SSH targets
```

To use a different port, set `TS_LOCAL_ADDR`:

```env
TS_LOCAL_ADDR=127.0.0.1:9999
```

### Structured signals (for scripts)

Callers that spawn `ts-bridge connect` programmatically get two line-oriented
signals with a stable `TOKEN key=value` grammar — no port-polling or stderr
parsing required:

```text
# stdout, once the tunnel is accepting connections (local = the actual bound address):
READY local=127.0.0.1:33389 target=my-desktop:3389

# stderr, if startup fails, before a non-zero exit:
ERROR reason=bad_authkey detail="invalid key: unable to validate API key"
```

`reason` is one of a stable set: `bad_authkey`, `control_plane_unreachable`,
`ssh_bootstrap_failed`, `unknown`. Read stdout line-by-line and react on the
`READY ` prefix; on early exit, read the `reason` token instead of guessing from
the exit code (which stays a generic `1`). Pass `--quiet` to suppress the
decorative banner — the `READY` and `ERROR` lines still print.

## Configuration

| Variable | Default | Description |
|---|---|---|
| `TS_AUTHKEY` | — | **Required** (unless `--auth-key-file` is used). Tailscale/Headscale auth key (`tskey-*` or `hskey-*`). Plaintext in env; use `--auth-key-file` for hardened deployments. |
| `TS_TARGET` | — | **Required** (unless `--profile` is used). Target host:port — supports both IPs and MagicDNS hostnames (e.g., `100.x.x.x:3389` or `my-desktop:3389`). |
| `TS_LOCAL_ADDR` | `127.0.0.1:33389` | Local bind address. |
| `TS_HOSTNAME` | — | Tailscale hostname (default: auto-derived from target). |
| `TS_CONTROL_URL` | — | Custom control plane URL for Headscale. |
| `TS_BOOTSTRAP_SSH` | — | OpenSSH endpoint (`user@host[:port]`) used to reach a blocked custom control plane. |
| `TS_BOOTSTRAP_SOCKS_ADDR` | `127.0.0.1:1055` | Loopback SOCKS listener created by the SSH bootstrap. |
| `TS_HEALTH_ADDR` | — | Enable health/metrics HTTP server. |
| `TS_VERBOSE` | `false` | Debug logging. |
| `TS_LOG_FORMAT` | `text` | `text` (console) or `json` (file). |

> **Minimal setup:** For most users, only an auth key (via `--auth-key-file` or `TS_AUTHKEY`) and `TS_TARGET` are needed. Everything else has sensible defaults.

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
