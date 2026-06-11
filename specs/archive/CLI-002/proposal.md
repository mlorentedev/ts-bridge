---
id: "CLI-002"
type: spec
status: archived
merged: "2026-06-11"
pr: 62
created: "2026-06-10"
tags: [spec, cli, connect, config]
issue: 49
---

# CLI-002: Implement ts-bridge connect subcommand + YAML config support

## Why

Replace run.sh/run.ps1 with a native Go subcommand. Users run ts-bridge connect --target HOST:PORT instead of sourcing .env and running a shell script. Additionally, add optional YAML config file support for persistent, structured configuration.

## What

Implement ts-bridge connect with all flags that map to existing TS_* env vars. The subcommand loads config from the following precedence chain (high to low):

1. CLI flags
2. Env vars (TS_*)
3. YAML config file (--config)
4. Built-in defaults

### Config merge logic

The merge logic lives in `internal/config/` as a shared utility. The same precedence chain is used by all subcommands.

Steps:
1. Start with defaults from Config struct
2. Load YAML file if --config is provided (optional, not an error if missing)
3. Overlay env vars on top
4. Overlay CLI flags on top (highest precedence)

### CLI Flags for connect

```
--target HOST:PORT         Target address (overrides TS_TARGET)
--auth-key KEY             Auth key (overrides TS_AUTHKEY) — WARNING: visible in process list
--auth-key-file PATH       Read auth key from file (secure alternative to --auth-key)
--instance NAME            Instance name for auto-mode
--local-addr ADDR          Local bind address
--hostname NAME            Tailscale hostname
--state-dir PATH           State directory
--control-url URL          Custom control plane URL
--timeout DURATION         Connect timeout for tsnet init
--dial-timeout DURATION    Per-dial timeout
--idle-timeout DURATION    Idle connection timeout
--drain-timeout DURATION   Graceful drain timeout
--max-conns N              Max concurrent connections
--dial-retries N           Dial retry count
--dial-backoff-base D      Dial backoff base
--dial-backoff-max D       Dial backoff max
--health-addr ADDR         Health server address
--log-format FORMAT        Log format (text|json)
--manual-mode              Disable auto-instance mode
--port-range RANGE         Auto port range
--reset                    Reset state dir before starting (no-op in auto mode, state is already ephemeral)
--config PATH              Path to YAML config file (default: none)
```

### YAML Config Schema

The YAML config file is optional. When present, it provides non-sensitive settings. The auth key MUST NOT be stored in YAML.

```yaml
# ts-bridge configuration
# Auth key is NOT stored here — use TS_AUTHKEY env var or --auth-key-file
version: 1

target: "100.64.0.1:3389"
hostname: "my-bridge"
local_addr: "127.0.0.1:33389"
control_url: ""  # Empty = Tailscale SaaS

# Timeouts
timeout: 30s
dial_timeout: 5s
idle_timeout: 0     # 0 = disabled
drain_timeout: 15s

# Dial retry
dial_retries: 3
dial_backoff_base: 1s
dial_backoff_max: 30s

# Limits
max_connections: 1000

# Health
health_addr: "127.0.0.1:9090"

# Logging
log_format: "text"   # text or json

# Multi-target support (future)
# instances:
#   - name: office
#     target: "100.64.0.1:3389"
#     local_addr: "127.0.0.1:33389"
```

### Security

- `--auth-key` flag is available but logs a WARNING that the key is visible in the process list
- `--auth-key-file PATH` is the recommended way to pass the auth key (reads from file, file should have 0600 permissions)
- `TS_AUTHKEY` env var remains the primary secure path
- Auth key in YAML is explicitly rejected with an error message directing to use TS_AUTHKEY or --auth-key-file
- YAML file with world-readable permissions (>0600 on Unix) produces a warning

### YAML Implementation

- New file `internal/config/yaml.go` for YAML struct + loader
- `LoadYAMLConfig(path string) (Config, error)` returns a partial Config
- Merge logic in `internal/config/merge.go` applies: flags > env > yaml > defaults
- `yaml.v3` dependency
- Validation: reject unknown fields, validate types, reject auth key in YAML
- Schema includes `version: 1` field for future evolution

## Out of scope

- Multi-target instances (future enhancement)
- TOML/JSON formats (YAML only)
- Removing old scripts (OPS-002)

## Dependencies

- This spec subsumes CLI-005 (YAML config). CLI-005 is closed as duplicate.
- Depends on CLI-001 (Cobra scaffold + cmd/ package)

## Acceptance criteria

### Connect subcommand

- [ ] ts-bridge connect --target 100.64.0.1:3389 works without any env vars
- [ ] ts-bridge connect --auth-key-file /path/to/key works (reads key from file)
- [ ] ts-bridge connect with TS_TARGET+TS_AUTHKEY env vars works (backward compat)
- [ ] All existing TS_* env vars still work when flags are not provided
- [ ] Flag precedence: CLI flag > env var > YAML > default
- [ ] --reset is no-op in auto mode (ephemeral state)
- [ ] --auth-key logs a WARNING about process list visibility

### YAML config

- [ ] --config ts-bridge.yaml loads and applies settings
- [ ] CLI flags override YAML values
- [ ] Env vars override YAML values
- [ ] Missing YAML file is not an error (config is optional)
- [ ] Unknown YAML fields produce a warning
- [ ] Invalid YAML values produce a clear error
- [ ] Auth key in YAML is explicitly rejected with message to use TS_AUTHKEY or --auth-key-file
- [ ] `version: 1` in YAML is accepted; missing version uses defaults
- [ ] YAML with world-readable perms (>0600 on Unix) produces a warning

### General

- [ ] Config merge logic is in internal/config/, not in cmd/
- [ ] go test ./... green
- [ ] golangci-lint run clean
- [ ] Binary size increase < 2MB vs current build
- [ ] PR < 400 lines diff (excluding tests, go.sum)

## References

- ADR-008 (CLI Architecture)
- Issue #49
- scripts/client/run.sh, run.ps1
- TECH-005 (auth key security)
- Internal note: CLI-005 subsumed into this spec