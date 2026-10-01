---
id: lesson-018-ephemeral-mode-mandates-auth-key-rotation-on-
type: lesson
status: active
created: "2026-05-28"
owner: manu
tags: [ts-bridge, lesson, tailscale, ephemeral, auth-key, operational]
---

# Ephemeral mode mandates auth-key rotation on every client (operational reality)

**Context:** v1.5.0 client on a corporate Windows machine failed with `tsnet.Up: backend: invalid key: API key does not exist` two months after the `.env` was last edited. Initial misdirection: assumed Headscale misconfiguration (`TS_CONTROL_URL` missing). Vault correction: the 3 acemagic-* PCs are explicitly on Tailscale SaaS (see lesson [2026-03-16]).

**Root cause:** `tsnet.Server` runs ephemeral nodes, and auto-mode wipes the state dir on shutdown. Result: every bridge startup is a *fresh node registration* that reuses the configured auth key. There is no "already registered, no need to update" path. When the key expires, is revoked, or hits its single-use cap, every affected profile fails registration. The host machines on native Tailscale are unaffected because they use persistent state.

**Diagnostic flow that worked:**
1. Error literal `API key does not exist` → control plane has no record of this key. Three possibilities, all server-side: (a) expired, (b) revoked, (c) single-use already consumed.
2. Run `ts-bridge auth status --profile <profile>` to confirm the managed credential binding without printing the value.
3. Confirm at `https://login.tailscale.com/admin/settings/keys` — if status is *Expired*/*Revoked* or the key is absent, that is the bug. A structurally valid `tskey-auth-<id>-<secret>` can still be invalid at the provider.
4. Generate exactly one replacement key: **Reusable** + **Ephemeral**, max TTL (90d for Tailscale SaaS), then store it with `ts-bridge auth set --profile <profile> --force`.

**Rule:** With ephemeral tsnet nodes, auth-key rotation is not optional — it is recurring operational work tied to the key TTL. Keep non-secret connection settings in profiles/YAML and rotate the secret only through the managed credential store; never restore `.env` as the normal secret-distribution path. A scheduled reminder at TTL−7d is the lightweight mitigation; provider-backed create/rotate automation is tracked separately.

**Tags:** `#tailscale` `#ephemeral` `#auth-key` `#operational`
