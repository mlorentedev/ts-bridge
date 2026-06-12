---
id: "ts-bridge-multi-device-ops"
type: runbook
status: active
tags: [operations, multi-device, windows, linux, aliases]
created: "2026-02-24"
updated: "2026-06-12"
owner: manu
---

# Runbook: Multi-Device Stateless Operations

## Goal

Operate ts-bridge from multiple client devices with minimal friction, using alias-based launches and stateless sessions.

## Source of Truth (Vault)

Keep one inventory table in this runbook for all managed client aliases.

| Alias | Target (`TS_TARGET`) | Expected Local Port | Client OS | Last Verified | Notes |
|------|-----------------------|---------------------|-----------|---------------|-------|
| office-laptop | 100.x.x.x:45000 | 33685 | Windows | 2026-02-24 | Primary workstation |
| lab-pc | 100.x.x.x:45000 | 33627 | Windows | 2026-02-24 | Secondary instance |
| laptop-2 | 100.x.x.x:45000 | 34291 | Windows | 2026-02-24 | Additional client alias validated |

## Configuration Baseline

Set only these values in `.env` on every client:

```env
TS_TARGET=<tailscale-ip:port>
TS_AUTHKEY=<tskey-auth-...>
TS_INSTANCE_NAME=<device-alias>
```

Optional:

```env
TS_PORT_RANGE=33389-34388
```

Do not set `TS_LOCAL_ADDR`, `TS_HOSTNAME`, or `TS_STATE_DIR` unless you intentionally want manual overrides.

## Bootstrap Commands

### Interactive setup

```bash
# Linux / macOS
./ts-bridge init

# Windows
.\ts-bridge.exe init
```

### Non-interactive (quick setup)

```bash
# Linux / macOS
./ts-bridge init \
  --auth-key tskey-auth-... \
  --target 100.x.x.x:45000 \
  --instance office-laptop

# Windows
.\ts-bridge.exe init ^
  --auth-key tskey-auth-... ^
  --target 100.x.x.x:45000 ^
  --instance office-laptop
```

## Launch Commands

```bash
# All platforms — reads .env with TS_INSTANCE_NAME automatically
./ts-bridge connect

# With explicit instance override
./ts-bridge connect --instance office-laptop

# With verbose logging
./ts-bridge connect -v
```

## Validation Workflow

1. Start once and note `Local:` from banner.
2. Restart and verify the same `Local:` port (if still free).
3. Start a second instance in parallel (different `.env` or `--instance`) and verify a different `Local:` port.
4. Update `Last Verified` in the inventory table.

## Windows Runtime Notes

- `wsarecv: ... forcibly closed by the remote host` is treated as an expected close path (usually remote-side session termination).
- If RDP closes unexpectedly during multi-client tests, verify destination host session policy (many desktop editions allow only one interactive session).
- Ephemeral state cleanup now retries briefly on shutdown to reduce transient temp-directory race warnings.

## Hostname Strategy

- Auto mode generates a unique hostname each run for collision safety.
- Use alias (TS_INSTANCE_NAME) as your stable operational identity in Vault.
- If you need a stable hostname for admin visibility, set `TS_HOSTNAME` explicitly and treat it as managed configuration.

## Linux Parity Checklist

- [ ] `./ts-bridge connect --instance <alias>` works with `.env` auto mode settings.
- [ ] Reboot test keeps deterministic local port for same alias.
- [ ] Concurrent instances produce distinct local ports.
- [ ] RDP/SSH client can connect to `127.0.0.1:<local-port>`.
