---
title: CLI Reference
description: Complete reference for all ts-bridge subcommands, flags, and usage patterns.
---

## Overview

```
ts-bridge [command]

Available Commands:
  connect     Start the TCP bridge
  host        Host setup and verification commands
  init        Interactive setup wizard
  status      Show bridge health and metrics summary
  version     Print version information

Flags:
  -v, --verbose   Enable verbose logging
      --config string   Config file path (reserved for future use)
```

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
| `--state-dir` | string | State directory (default `./ts-state`) |
| `--target` | string | Target address HOST:PORT |
| `--timeout` | duration | Connect timeout for tsnet init (default `30s`) |

### Examples

```bash
# Run with .env config
ts-bridge connect

# Run with all flags inline
ts-bridge connect --target 100.64.0.1:3389 --auth-key tskey-auth-xxxxx

# Run with YAML config
ts-bridge connect --config ts-bridge.yaml

# Secure: read auth key from file
ts-bridge connect --auth-key-file /run/secrets/authkey

# Manual mode (persistent hostname)
ts-bridge connect --manual-mode --instance my-laptop --local-addr 127.0.0.1:33389
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
| `--config` | string | Output config file path (default: `./ts-bridge.yaml` for yaml, `./.env` for env) |
| `--format` | string | Output format: `yaml` or `env` (default `env`) |
| `--instance` | string | Instance name for auto-mode |
| `--port-range` | string | Port range for auto mode (e.g. `33389-34388`) |
| `--target` | string | Target address HOST:PORT (non-interactive mode) |

### Examples

```bash
# Interactive wizard (prompts for all values)
ts-bridge init

# Non-interactive: .env output (default)
ts-bridge init --auth-key tskey-auth-xxxxx --target 100.64.0.1:3389

# Non-interactive: YAML output
ts-bridge init --auth-key tskey-auth-xxxxx --target 100.64.0.1:3389 --format yaml
```

### Security notes

- Auth key input is masked in interactive mode (no echo).
- In YAML mode the auth key is written to `.env`, NOT to the YAML file.
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
  setup   Configure the host for RDP access over Tailscale
```

### `ts-bridge host setup`

Configure the host machine for RDP access. Requires admin privileges on Windows.

```bash
# Configure host for RDP
ts-bridge host setup

# Skip sleep prompts
ts-bridge host setup --no-sleep
```

### `ts-bridge host check`

Verify host readiness without making changes.

```bash
# Human-readable output
ts-bridge host check

# JSON output for scripts
ts-bridge host check --json
```

## `ts-bridge version`

Print version information.

```bash
ts-bridge version
```
