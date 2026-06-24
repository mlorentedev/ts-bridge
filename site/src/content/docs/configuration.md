---
title: Configuration
description: Full configuration reference — environment variables, YAML config, and precedence rules.
---

## Config precedence

Configuration is resolved in this order (highest to lowest):

1. **CLI flags** — explicit flags on the command line
2. **Environment variables** — `TS_*` prefix
3. **YAML config file** — specified with `--config`
4. **Built-in defaults** — hardcoded fallbacks

## Environment variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `TS_AUTHKEY` | Auth key. Tailscale: `tskey-*`. Headscale: `hskey-*` | `tskey-auth-kXXXXXX` |
| `TS_TARGET` | Host address on the mesh network | `100.82.151.104:3389` |

### Optional

| Variable | Default | Description |
|----------|---------|-------------|
| `TS_LOCAL_ADDR` | `127.0.0.1:33389` | Local bind address. Auto-derived in auto mode when unset. |
| `TS_CONTROL_URL` | _(Tailscale default)_ | Custom control plane URL for self-hosted Headscale. |
| `TS_HOSTNAME` | `ts-bridge` | Node name in the admin console. Auto-generated per run in auto mode. |
| `TS_STATE_DIR` | _(per-user)_ | Directory for node state (holds the private node identity). Default is a fixed per-user dir: Windows `%LOCALAPPDATA%\ts-bridge\state`, macOS `~/Library/Application Support/ts-bridge/state`, Linux `$XDG_STATE_HOME/ts-bridge/state` (→ `~/.local/state/ts-bridge/state`). Created with `0700` permissions. Ephemeral temp dir in auto mode with an instance. A relative override is warned about. |
| `TS_AUTO_INSTANCE` | `true` | Auto mode toggle. Set `false` to disable auto behavior. |
| `TS_MANUAL_MODE` | `false` | Force legacy persistent mode. Takes precedence over `TS_AUTO_INSTANCE`. |
| `TS_INSTANCE_NAME` | _(empty)_ | Stable instance alias for deterministic local port selection. |
| `TS_PORT_RANGE` | `33389-34388` | Port range for auto mode (`START-END`). |
| `TS_TIMEOUT` | `30s` | Timeout for tsnet initialization (control-plane handshake). Go duration format. |
| `TS_DIAL_TIMEOUT` | `5s` | Per-connection target dial timeout. Keeps stuck dials from holding a slot across retries. Go duration format. _(v1.8.0+)_ |
| `TS_DRAIN_TIMEOUT` | `15s` | Timeout for graceful drain of active connections on shutdown. Go duration format. |
| `TS_MAX_CONNECTIONS` | `1000` | Maximum concurrent connections before rejecting new ones. |
| `TS_IDLE_TIMEOUT` | _(disabled)_ | Close connections after this period of no traffic. Go duration format (e.g. `30m`). Default `0` disables. _(v1.6.0+)_ |
| `TS_DIAL_RETRIES` | `3` | Maximum retries for transient target dial failures. `0` disables. _(v1.7.0+)_ |
| `TS_DIAL_BACKOFF_BASE` | `1s` | Base backoff for dial retries (multiplied by `2^attempt`, plus jitter). Go duration format. _(v1.7.0+)_ |
| `TS_DIAL_BACKOFF_MAX` | `30s` | Cap on backoff per retry attempt. Must be ≥ `TS_DIAL_BACKOFF_BASE`. _(v1.7.0+)_ |
| `TS_HEALTH_ADDR` | _(disabled)_ | Address for health/metrics HTTP server. |
| `TS_VERBOSE` | `false` | Enable debug logging. Also available as `-v` flag. |
| `TS_LOG_FORMAT` | `text` | Log output format (`text` or `json`). |

### Boolean parsing

`TS_VERBOSE`, `TS_AUTO_INSTANCE`, `TS_MANUAL_MODE` accept: `1`, `true`, `yes`, `on` (case-insensitive). All other values are treated as `false`.

## YAML config file

YAML config is optional and supports non-sensitive settings only. The auth key **must not** be stored in YAML files.

### Example

```yaml
# ts-bridge.yaml
target: "100.82.151.104:3389"
localAddr: "127.0.0.1:33389"
instanceName: "office-laptop"
portRange: "33389-34388"
healthAddr: "127.0.0.1:9090"
maxConnections: 1000
idleTimeout: "30m"
logFormat: "json"
```

### Loading

```bash
# Load from YAML, auth key from env
ts-bridge connect --config ts-bridge.yaml

# Auth key from file (secure)
ts-bridge connect --config ts-bridge.yaml --auth-key-file /run/secrets/authkey
```

## Headscale configuration

For self-hosted Headscale, set `TS_CONTROL_URL` and use `hskey-*` auth keys:

```bash
TS_AUTHKEY=hskey-auth-xxxxx
TS_TARGET=100.64.0.5:3389
TS_CONTROL_URL=https://vpn.example.com
```

The `init` wizard supports both Tailscale and Headscale — it detects the key prefix and configures accordingly.
