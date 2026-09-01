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

Set only these values in `.env` on every client. To keep the key out of the environment, leave
`TS_AUTHKEY` unset and pass `--auth-key-file` to `connect` at launch:

```env
TS_TARGET=<tailscale-ip:port>
TS_AUTHKEY=<tskey-auth-...>       # Or omit this line and use: connect --auth-key-file <file>
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
# Linux / macOS — the auth key is prompted with masked input, so it never lands on the
# command line or in shell history.
./ts-bridge init

# Windows — same masked prompt
.\ts-bridge.exe init
```

### Non-interactive (quick setup)

`init` has no `--auth-key-file` (only `connect` registers it, #306), so unattended provisioning
is the one case where the key must go on the command line:

```bash
# Linux / macOS
./ts-bridge init \
  --auth-key "$(cat ~/.config/ts-bridge/authkey)" \
  --target 100.x.x.x:45000 \
  --instance office-laptop
```

```powershell
# Windows PowerShell
.\ts-bridge.exe init `
  --auth-key (Get-Content "$env:USERPROFILE\.ts-bridge\authkey") `
  --target 100.x.x.x:45000 `
  --instance office-laptop
```

This form is visible in the process table for the length of the call; prefer the interactive
prompt wherever a human is present, and pass the key file to `connect` at launch instead — see
Launch Commands below.

> **Security Note:** `--auth-key` puts the key on the command line, where it is visible in the
> process table (`ps`, Task Manager) to every local user, and `TS_AUTHKEY` is inherited by every
> child process ts-bridge spawns. Prefer the interactive `init` prompt (masked, never echoed),
> and supply the key to `connect` from a `0600` file with `--auth-key-file` rather than from the
> environment.

## Launch Commands

```bash
# Linux / macOS — reads .env with TS_INSTANCE_NAME automatically
./ts-bridge connect

# With explicit instance override
./ts-bridge connect --instance office-laptop

# With verbose logging
./ts-bridge connect -v
```

On Windows, substitute `.\ts-bridge.exe` (PowerShell line continuations are `` ` ``).

Key read from a file, so it never enters the environment or the process table. `--auth-key-file`
beats a `TS_AUTHKEY` that is still set (flags > env), but delete the plaintext line rather than
leaving it as a fallback: it stays readable to every child process while it exists.

```bash
# Linux / macOS — key file at 0600
./ts-bridge connect --auth-key-file ~/.config/ts-bridge/authkey
```

```powershell
# Windows — the 0600 equivalent is an ACL that grants only the owning user
.\ts-bridge.exe connect --auth-key-file "$env:USERPROFILE\.ts-bridge\authkey"
```

On Windows the file's inheritance should be trimmed with `icacls <file> /inheritance:r`
`/grant:r "$env:USERNAME:F"`; `chmod` has no effect on NTFS ACLs.

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
