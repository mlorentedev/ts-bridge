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

Set only these values in `.env` on every client. If you keep the key out of the environment,
`init` writes the rest and `connect` takes `--auth-key-file` at launch:

```env
TS_TARGET=<tailscale-ip:port>
TS_AUTHKEY=<tskey-auth-...>       # Or omit this line and pass --auth-key-file /path/to/keyfile
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
```bash
# Linux / macOS — interactive: the auth key is prompted with masked input, so it never
# lands on the command line or in shell history.
./ts-bridge init \
  --target 100.x.x.x:45000 \
  --instance office-laptop

# Windows — same, masked prompt (drop the ^ line continuations if you type it on one line)
.\ts-bridge.exe init ^
  --target 100.x.x.x:45000 ^
  --instance office-laptop
```

`init` has no `--auth-key-file`: `--auth-key` exists only for non-interactive provisioning, and
`connect` is where the key file is read from — see Launch Commands below.

> **Security Note:** `--auth-key` puts the key on the command line, where it is visible in the
> process table (`ps`, Task Manager) to every local user, and `TS_AUTHKEY` is inherited by every
> child process ts-bridge spawns. Prefer the interactive `init` prompt (masked, never echoed),
> and supply the key to `connect` from a `0600` file with `--auth-key-file` rather than from the
> environment.

## Launch Commands

```bash
# All platforms — reads .env with TS_INSTANCE_NAME automatically
./ts-bridge connect

# Key read from a 0600 file, so it never enters the environment or the process table
./ts-bridge connect --auth-key-file ~/.config/ts-bridge/authkey

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
