---
id: "ts-bridge-error-auth-failure"
type: troubleshooting
status: active
tags: [authentication, tailscale, auth-key]
created: "2026-02-23"
owner: manu
---

# Error: Authentication Failure

## Symptom
```
tailscale init failed (control=https://controlplane.tailscale.com (default)): tsnet.Up: backend: invalid key: API key does not exist
```

The literal string `API key does not exist` (or `invalid key` / `key expired` in older tsnet versions) means the control plane has *no record* of the key — it was never registered, has been revoked, or has expired.

In ts-bridge v1.5.1+ a structured WARN with `remediation` is emitted alongside the error.

## Causes (in order of likelihood)
1. **Key expired** — Tailscale SaaS reusable+ephemeral keys have a max TTL of 90 days. The `.env` mtime + TTL gives an estimate.
2. **Key revoked / deleted** from `https://login.tailscale.com/admin/settings/keys`.
3. **Key was single-use and already consumed** — ts-bridge runs with `Ephemeral: true` and a wiped state dir, so every startup re-consumes the key. A single-use key works *exactly once*.
4. **Key sent to the wrong control plane** — `TS_CONTROL_URL` empty defaults to Tailscale SaaS; a Headscale-minted key sent there would be rejected. The v1.5.1+ error message prints the effective control URL.
5. **Network blocking the control plane** — much less common with Tailscale SaaS (CDN/Cloudflare IPs are usually allowlisted); see [lessons.md](../lessons.md) (2026-03-13 entry) for the Headscale corporate-firewall edge case.

## Fix

1. Confirm at https://login.tailscale.com/admin/settings/keys — check the key's status (Expired / Revoked / Active) and the *Reusable* + *Ephemeral* flags.
2. If expired/revoked/missing → generate a replacement: **Reusable** ✅, **Ephemeral** ✅, expires in 90 days (max).
3. Update `TS_AUTHKEY` in `.env` on **every client machine** running ts-bridge — see operational note below.
4. Re-run `./ts-bridge`.

## Operational note: rotation cadence

Because `main.go` hardcodes `Ephemeral: true` and auto-mode wipes the state dir on shutdown, **every bridge startup re-consumes the auth key**. There is no "node is already registered" cache that survives a restart. Consequences:

- When the key expires/rotates, **every client breaks at once** — they must all be updated.
- Host machines on native Tailscale (e.g., `acemagic-office`, `acemagic-lab-1`, `acemagic-lab-2`) are *not* affected — they use persistent node state.
- Schedule a calendar reminder at `key_creation_date + 83 days` (a week before the 90-day max TTL) to generate the replacement and push to all `.env` files before users hit the expired error.

See lesson [2026-05-18] *"Ephemeral mode mandates auth-key rotation on every client"*.

## v1.5.0 cleanup quirk (Windows, fixed in v1.5.1)

Before v1.5.1, an auth failure on Windows produced a confusing companion warning:

```
level=WARN msg="failed to cleanup ephemeral state directory" path=... error="...tailscaled.log1.txt: The process cannot access the file because it is being used by another process." attempts=5
```

Cause: `initTailscale` did not call `server.Close()` on the error path, so the partially-started tsnet workers kept the log file locked. PR #18 (v1.5.1) calls `server.Close()` before the cleanup defer fires. If you still see this warning, you're on v1.5.0 or earlier — upgrade.

## Notes

- Ephemeral keys have max 90-day TTL on Tailscale SaaS.
- Use **reusable** keys for ts-bridge so one key serves all clients.
- Certificate warnings with RDP: use `/cert:ignore` with `xfreerdp` or accept in `mstsc`.

## Related

- [adr-005-headscale-compat.md](../adr/adr-005-headscale-compat.md) — `TS_CONTROL_URL` semantics
- [lessons.md](../lessons.md) (2026-05-18) — full diagnostic flow and the tsnet partial-start lifecycle rule
- [error-state-permissions.md](error-state-permissions.md) — adjacent state-dir issues

## Quick Reference

| Issue | Solution |
|-------|----------|
| `tailscale init failed` | Invalid auth key or network blocking |
| Certificate warnings | Use `/cert:ignore` with xfreerdp |
