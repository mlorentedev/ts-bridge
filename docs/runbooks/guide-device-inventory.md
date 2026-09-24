---
id: "guide-device-inventory"
type: runbook
status: active
tags: [operations, inventory, devices, tailscale, rdp]
created: "2026-03-12"
updated: "2026-03-16"
owner: manu
---

# Device Inventory: ts-bridge Target Mapping

> Source of truth for all machines accessed via ts-bridge.
> Update this file whenever a new host is registered or its configuration changes.

---

## Control Plane

All Windows hosts use **Tailscale SaaS** (not Headscale). Corporate networks
have transparent TLS inspection that blocks the Headscale Noise protocol.
Managed machines can trust the corporate inspection CA, but unmanaged hosts on
the same Wi-Fi or an Internet Connection Sharing path may fail Tailscale SaaS
login with `x509: certificate signed by unknown authority`.

See
[lesson-014](../lessons/lesson-014-corporate-tls-inspection-breaks-headscale-tcp.md)
and kubelab ADR-013 Addendum for the full rationale.

| Parameter | Value |
|-----------|-------|
| Control plane | Tailscale SaaS (`controlplane.tailscale.com`) |
| Auth | Personal Tailscale account |
| Access protocol | RDP (all hosts) |

---

## Windows Hosts

| Alias (`TS_INSTANCE_NAME`) | Tailscale IP | Port | OS User | Notes |
|----------------------------|-------------|------|---------|-------|
| `acemagic-office` | `100.82.151.104` | 45000 | `.\manu` | Office workstation, custom RDP port |
| `acemagic-lab-1` | `100.118.157.114` | 3389 | `.\e2v` | Lab PC 1 |
| `acemagic-lab-2` | `100.103.27.100` | 3389 | `.\e2v` | Lab PC 2 |

### .env examples

```bash
# acemagic-office
TS_TARGET=100.82.151.104:45000
TS_INSTANCE_NAME=acemagic-office

# acemagic-lab-1
TS_TARGET=100.118.157.114:3389
TS_INSTANCE_NAME=acemagic-lab-1

# acemagic-lab-2
TS_TARGET=100.103.27.100:3389
TS_INSTANCE_NAME=acemagic-lab-2
```

### Named connection profiles

Import each non-default target once so operators do not need to remember its
hostname or port:

```powershell
.\ts-bridge.exe import office "tsb://acemagic-office:45000?cp=saas"
.\ts-bridge.exe connect --profile office --auth-key-file "$HOME\.ts-bridge\authkey"
```

The profile stores only the target and control-plane selection. It does not
contain the auth key. Secure automatic credential discovery is tracked by
[#355](https://github.com/mlorentedev/ts-bridge/issues/355).

---

## Client Machine

The corporate workstation runs ts-bridge as a **client** (userspace, no admin needed).
It connects to the hosts above via Tailscale SaaS mesh.

No `TS_CONTROL_URL` is set — ts-bridge defaults to Tailscale SaaS.

---

## Adding a New Host

1. Install Tailscale on the Windows PC and authenticate with personal account.
2. Note the Tailscale IP from `tailscale ip -4`.
3. Enable RDP on the host (Settings → System → Remote Desktop).
4. Add a row to the **Windows Hosts** table above.
5. Create a `.env` with `TS_TARGET=<IP>:<PORT>` and `TS_INSTANCE_NAME=<alias>`.

---

## Port Mapping Reference

Auto mode assigns a deterministic local port based on `TS_INSTANCE_NAME`:

```
port = BASE_PORT + (fnv32(instance_name) % RANGE_SIZE)
```

With default range `33389-34388`, the port for a given alias is always the same
on the same client machine. Use `TS_VERBOSE=true` to see the calculated port at startup.

---

## Why Not Headscale

Headscale migration was attempted (Feb-Mar 2026) and blocked by corporate transparent TLS inspection. The firewall MITMs all TLS, validates it is HTTP, and kills the Headscale Noise protocol (binary, not HTTP). Tailscale SaaS works because its relay IPs are CDN/Cloudflare addresses trusted by corporate firewalls.

Archived runbooks (for reference only):
- `90_archive/ts-bridge/guide-headscale-migration.md`
- `90_archive/ts-bridge/guide-windows-host-to-headscale.md`
- `90_archive/ts-bridge/guide-windows-host-headscale-fresh.md`

---

## References

- [guide-deployment-windows.md](guide-deployment-windows.md) — Client deployment on Windows
- [guide-deployment-linux.md](guide-deployment-linux.md) — Client deployment on Linux
