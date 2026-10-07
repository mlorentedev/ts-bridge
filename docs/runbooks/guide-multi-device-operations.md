---
id: "ts-bridge-multi-device-ops"
type: runbook
status: active
tags: [operations, multi-device, windows, linux, aliases]
created: "2026-02-24"
updated: "2026-10-05"
owner: manu
---

# Runbook: Multi-Device Stateless Operations

## Goal

Operate ts-bridge from multiple client devices with named profiles, managed
credentials, and repeatable live validation.

## Source of Truth

Each client owns its local `profiles.yaml` and managed credential store. Profiles
hold non-secret connection settings; credentials are referenced by name and are
never copied into this runbook, YAML, command arguments, or environment variables.

## Configuration Baseline

Create the non-secret profile, then bind a managed credential through masked
input:

```bash
./ts-bridge init --profile office --target acemagic-office:45000
./ts-bridge auth set --profile office
```

```powershell
.\ts-bridge.exe init --profile office --target acemagic-office:45000
.\ts-bridge.exe auth set --profile office
```

Use `--force` with `init` or `auth set` only when intentionally replacing an
existing profile or rotated key. `.env` and `--auth-key-file` remain compatibility
paths, not the preferred multi-device operating model. If a Windows client still
uses `--auth-key-file`, trim the file's inheritance with
`icacls <file> /inheritance:r /grant:r "$env:USERNAME:F"`; `chmod` has no effect
on NTFS ACLs.

When migrating an existing client, remove `TS_AUTHKEY` from its `.env` file and
process environment before relying on the managed credential. Environment
variables have higher precedence and would otherwise keep the legacy key active
after `auth set` stores or rotates the profile credential.

Remove `TS_TARGET` the same way, together with any `target:` key in the YAML
config file. Both rank above the profile, so either one silently replaces the
profile's target and `connect --profile office` reaches the wrong endpoint.

## Launch Commands

```bash
./ts-bridge connect --profile office
```

```powershell
.\ts-bridge.exe connect --profile office
```

## Validation Workflow

1. Start `connect --profile <name>` and wait for the structured `READY` line.
2. Verify `/health/live`, `/health/ready`, and `/metrics`.
3. Send an application-level request through the local listener and verify a
   valid response from the remote service.
4. Interrupt the active network path, confirm a request fails, restore the
   path, and measure recovery without restarting ts-bridge.
5. Exercise a transient target failure and verify the configured retry count.
6. Exercise a terminal resolution failure and verify it does not retry.
7. Run sustained connection load while sampling latency, errors, memory,
   handles, threads, and `active_connections`.

## QA-013 Windows Evidence (2026-10-05)

The `office` profile connected this Windows workstation to
`acemagic-office:45000` over Tailscale SaaS.

| Check | Result |
|------|--------|
| Readiness | `READY`; live and ready endpoints returned `ok` |
| Bidirectional application probe | Valid 19-byte RDP X.224 Connection Confirm |
| Network interruption | 73.76-second Wi-Fi outage; probe failed during outage |
| Recovery | Same process recovered in 3.5 seconds after Wi-Fi returned |
| Transient retries | Closed remote port produced 4 total attempts with backoff |
| Terminal failure | NXDOMAIN stopped after one attempt |
| Sustained load | 1,000/1,000 RDP negotiations; 0 failures; 29.88 connections/s |
| Latency | p50 26.39 ms; p95 38.35 ms; p99 85.27 ms |
| Interactive RDP payload | 8,096,109 bytes over 487.41 seconds; 0 errors; operator confirmed responsive use |
| Resource stability | Working set 72.68-73.71 MiB; handles 1040-1042; threads 74-75 |
| Final state | `active_connections: 0`; readiness remained `ok` |

The interactive session averaged 0.133 Mbps across active and idle periods.
Byte totals became visible only after disconnect because live sessions are not
yet reflected in `total_bytes_tx` / `total_bytes_rx`; #415 tracks that
observability defect.

## Windows Runtime Notes

- `wsarecv: ... forcibly closed by the remote host` is treated as an expected close path (usually remote-side session termination).
- If RDP closes unexpectedly during multi-client tests, verify destination host session policy (many desktop editions allow only one interactive session).
- Ephemeral state cleanup now retries briefly on shutdown to reduce transient temp-directory race warnings.

## Hostname Strategy

- Auto mode generates a unique hostname each run for collision safety.
- Use the profile name (`office` above) as the stable operational identity. `--instance` / `TS_INSTANCE_NAME` only seeds the derived local port and hostname when neither is set explicitly; it is not an identity record.
- If you need a stable hostname for admin visibility, set `TS_HOSTNAME` explicitly and treat it as managed configuration.

## Linux Compatibility Checklist (`.env` / `--instance`)

- [ ] `./ts-bridge connect --instance <alias>` works with `.env` auto mode settings.
- [ ] Reboot test keeps deterministic local port for same alias.
- [ ] Concurrent instances produce distinct local ports.
- [ ] RDP/SSH client can connect to `127.0.0.1:<local-port>`.
