---
title: CLI Reference
description: Complete reference for all ts-bridge subcommands, flags, and usage patterns.
---

## Overview

```
ts-bridge [command]

Available Commands:
  auth        Manage profile-scoped credentials
  browser     Open an isolated browser through an allow-listed mesh proxy
  connect     Start the TCP bridge
  host        Host setup and verification commands
  init        Interactive setup wizard
  status      Show bridge health and metrics summary
  version     Print version information

Flags:
  -v, --verbose   Enable verbose logging
```

`--config` is not a global flag. `connect`, `browser`, `init` and `host` each take
their own `--config`, documented in their sections.

## `ts-bridge auth`

Manage keys without creating credential files manually or exposing values in
command-line arguments.

```bash
# Masked prompt; defaults the credential name to the profile name
ts-bridge auth set --profile office

# Use an explicit shared credential name
ts-bridge auth set shared-key --profile office

ts-bridge auth status --profile office
ts-bridge auth list
ts-bridge auth remove --profile office
```

`auth set --stdin` is available for explicit automation. Existing credentials
are never overwritten without `--force`.

## `ts-bridge connect`

Start the TCP bridge that forwards local connections to a remote target over the Tailscale mesh network.

```
ts-bridge connect [flags]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--auth-key` | string | Auth key (overrides `TS_AUTHKEY`) — WARNING: visible in process list |
| `--auth-key-file` | string | Read auth key from file (secure alternative to `--auth-key`) |
| `--bootstrap-socks-addr` | string | Loopback SOCKS listener used by SSH bootstrap (default `127.0.0.1:1055`) |
| `--bootstrap-ssh` | string | Reach the custom control plane through OpenSSH `user@host[:port]` |
| `--config` | string | Path to YAML config file |
| `--control-url` | string | Custom control plane URL |
| `--dial-backoff-base` | duration | Dial backoff base (default `1s`) |
| `--dial-backoff-max` | duration | Dial backoff max (default `30s`) |
| `--dial-retries` | int | Dial retry count (default `3`) |
| `--dial-timeout` | duration | Per-dial timeout (default `5s`) |
| `--drain-timeout` | duration | Graceful drain timeout (default `15s`) |
| `--health-addr` | string | Health server address |
| `--hostname` | string | Tailscale hostname |
| `--idle-timeout` | duration | Idle connection timeout (0 = disabled) |
| `--instance` | string | Instance name for auto-mode |
| `--local-addr` | string | Local bind address (default `127.0.0.1:33389`) |
| `--log-format` | string | Log format: `text` or `json` (default `text`) |
| `--manual-mode` | flag | Disable auto-instance mode |
| `--max-conns` | int | Max concurrent connections (default `1000`) |
| `--port-range` | string | Auto port range, e.g. `33389-34388` |
| `--reset` | flag | Reset state (manual mode only) |
| `--state-dir` | string | State directory holding the node identity. Default is a fixed per-user dir (e.g. `~/.local/state/ts-bridge/state` on Linux); a relative value is warned about |
| `--target` | string | Target address HOST:PORT |
| `--timeout` | duration | Connect timeout for tsnet init (default `30s`) |

### Examples

```bash
# Run with .env config
ts-bridge connect

# Secure: read auth key from file (the value never enters the process list)
ts-bridge connect --auth-key-file /run/secrets/authkey

# Run with all flags inline — quick try only: --auth-key is visible in `ps`
ts-bridge connect --target 100.64.0.1:3389 --auth-key tskey-auth-KEYID-SECRET

# Run with YAML config
ts-bridge connect --config ts-bridge.yaml

# Manual mode (persistent hostname)
ts-bridge connect --manual-mode --instance my-laptop --local-addr 127.0.0.1:33389

# Reach a DNS/TLS-filtered Headscale control plane without editing hosts
ts-bridge connect --control-url https://vpn.example.com \
  --bootstrap-ssh deployer@bastion.example.com \
  --auth-key-file /run/secrets/headscale-authkey \
  --target mesh-host:3389
```

## `ts-bridge browser`

Open Microsoft Edge with an isolated profile that reaches allow-listed private
origins through a loopback SOCKS5 proxy over the mesh (ADR-014).

```bash
ts-bridge browser [flags]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--auth-key-file` | string | Read auth key from a local file |
| `--config` | string | Path to YAML config file |
| `--control-url` | string | Custom Headscale control plane URL |
| `--dial-timeout` | duration | Per-dial timeout |
| `--edge-path` | string | Path to the Microsoft Edge executable (auto-detected when omitted) |
| `--hostname` | string | Tailscale hostname |
| `--pac-addr` | string | Loopback address for the local PAC server (default `127.0.0.1:0`) |
| `--profile` | string | Named profile supplying control plane and managed credential |
| `--route` | string (repeatable) | Allow-listed route `SOURCE_HOST:PORT=MESH_HOST:PORT` |
| `--socks5` | string | Loopback SOCKS5 listener address (default `127.0.0.1:1080`) |
| `--state-dir` | string | State directory |
| `--timeout` | duration | Connect timeout for tsnet init |
| `--url` | string | Private HTTPS URL to open |
| `--user-data-dir` | string | Isolated Edge user-data directory (default: a `ts-bridge-browser` directory under the system temp dir) |

### Example

```bash
ts-bridge browser --profile office \
  --route "forge.example.internal:443=apps:443" \
  --url https://forge.example.internal/
```

## `ts-bridge init`

Interactive setup wizard to create a ts-bridge configuration file.

```
ts-bridge init [flags]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--auth-key` | string | Auth key (non-interactive mode) — WARNING: visible in process list |
| `--auth-key-file` | string | Read auth key from file (secure non-interactive mode) |
| `--config` | string | Output config file path (default: `./ts-bridge.yaml` for yaml, `./.env` for env) |
| `--control-url` | string | Control plane URL (Headscale). Written to the config file, or stored with the profile under `--profile` |
| `--format` | string | Output format: `yaml` or `env` (default `env`) |
| `--instance` | string | Instance name for auto-mode |
| `--port-range` | string | Port range for auto mode (e.g. `33389-34388`) |
| `--target` | string | Target address HOST:PORT (non-interactive mode) |

### Examples

```bash
# Interactive wizard (prompts for all values)
ts-bridge init

# Secure non-interactive: .env output (default)
ts-bridge init --auth-key-file /run/secrets/authkey --target 100.64.0.1:3389

# Non-interactive: YAML output
ts-bridge init --auth-key-file /run/secrets/authkey --target 100.64.0.1:3389 --format yaml

# Headscale (hskey-*) requires the control plane in the same configuration
# file, otherwise connect rejects what init wrote
ts-bridge init --auth-key-file /run/secrets/hskey --target 100.64.0.1:3389 \
  --control-url https://headscale.example.com
```

### Security notes

- Auth key input is masked in interactive mode (no echo).
- In YAML mode the auth key is written to `.env`, NOT to the YAML file.
- `--auth-key-file` takes precedence over `--auth-key`.
- An explicitly passed `--auth-key` remains visible in the process list and emits a warning.
- A permission warning is shown if config files are world-readable.

## `ts-bridge status`

Query the bridge's health and metrics endpoints and display a human-readable summary.

```
ts-bridge status [flags]
```

### Flags

| Flag | Type | Description |
|------|------|-------------|
| `--addr` | string | Health server address (default `127.0.0.1:9090`) |
| `--interval` | duration | Polling interval for `--watch` (default `5s`) |
| `--json` | flag | Output raw JSON from `/metrics` |
| `--watch` | flag | Continuously watch and update status |

### Examples

```bash
# One-shot summary
ts-bridge status

# Custom address
ts-bridge status --addr 127.0.0.1:8080

# Continuous watch
ts-bridge status --watch --interval 2s

# Raw JSON metrics
ts-bridge status --json
```

## `ts-bridge host`

Manage host machine configuration for RDP access over Tailscale.

```
ts-bridge host [command]

Available Commands:
  check   Verify host readiness (read-only)
  init    Write the host configuration file that host setup and check read
  serve   Join the tailnet as a persistent node and publish the declared services
  setup   Configure the host for RDP access over Tailscale
```

### `ts-bridge host init`

Write the host's declared state (ADR-017) to the YAML file named by `--config`, which
`host setup`, `host check` and `host serve` then read with the same `--config`. `--config`
is required; no `.env` is written. `--firewall-rule` and `--no-sleep` apply to Windows
only. On macOS it writes nothing yet (#459).

```bash
# Interactive wizard
ts-bridge host init --config /etc/ts-bridge/host.yaml

# Non-interactive (Windows)
ts-bridge host init --config C:\ProgramData\ts-bridge\host.yaml --port 3389 --firewall-rule "MyRule" --no-sleep
```

An existing file is replaced only after confirmation, or with `--force`; a directory or an
unreadable file at the path is refused even with `--force`. The file is written `0644`:
choose a path whose parent directories an unelevated user can traverse, since `host check`
runs unelevated.

### `ts-bridge host setup`

Configure the host machine for RDP access. Requires admin privileges on Windows.

```bash
# Configure host for RDP from the host file
ts-bridge host setup --config /etc/ts-bridge/host.yaml

# Skip sleep mode
ts-bridge host setup --no-sleep

# Output JSON for automation
ts-bridge host setup --json

# Configure, then verify
sudo ts-bridge host setup && ts-bridge host check
```

Exit status: `0` when every step is `ok`, `changed` or `manual` (a manual step prints
the action left to you), `2` when setup ran and at least one step `failed`, `1` when setup
could not run (for example, without root). Use `host check` to confirm the host converged.

### `ts-bridge host check`

Verify host readiness without making changes.

```bash
# Human-readable output
ts-bridge host check

# JSON output for scripts
ts-bridge host check --json
```

Exit status: `0` when every step is `ok`, `2` when the check ran and at least one step
is not `ok` (each one is printed with its fix), `1` when the check could not run.

With `host.mesh.health_addr` in the `--config` file, `--json` adds `published` to each
service: whether `host serve` publishes it. Serve not answering, or a responder that does
not name the file's node, reads as `false`; without `health_addr` the field is absent.

### `ts-bridge host serve`

Join the tailnet as a persistent node and publish exactly the services the host file
declares, each forwarded to `127.0.0.1:<port>` (Windows and Linux). It runs in the
foreground; register it with the OS as a Scheduled Task or systemd unit
([runbook](https://github.com/mlorentedev/ts-bridge/blob/master/docs/runbooks/host-serve.md)).

```bash
ts-bridge host serve --config /etc/ts-bridge/host.yaml
```

The file must declare `host.control_url` and `host.mesh` with `state_dir`
([configuration](configuration.md#host-serve-hostmesh)). Signals:

```text
READY node=lab-pc-01 services=rdp:3389 enrolled=true    # stdout; true = this start used the auth key
ERROR reason=node_not_tagged detail="..."                # stderr; also publish_failed,
                                                         # ssh_bootstrap_failed, control_plane_unreachable
```

After `READY`, a dead SSH bootstrap is restarted with backoff and a lost control session
reads not ready on `/health/ready` (`host.mesh.health_addr`); serve keeps running.

## `ts-bridge version`

Print version information.

```bash
ts-bridge version
```
