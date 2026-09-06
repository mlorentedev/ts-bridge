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

**Root cause:** `main.go` hardcodes `Ephemeral: true` on `tsnet.Server`, and auto-mode wipes the state dir on shutdown. Result: every bridge startup is a *fresh node registration* that re-consumes the auth key. There is no "already registered, no need to update" path. When the key expires, revokes, or hits its single-use cap, **every client breaks simultaneously** and needs the new key in its local `.env`. The host machines on native Tailscale are unaffected — they use persistent state.

**Diagnostic flow that worked:**
1. Error literal `API key does not exist` → control plane has no record of this key. Three possibilities, all server-side: (a) expired, (b) revoked, (c) single-use already consumed.
2. Check `.env` mtime vs Tailscale auth-key TTL. If `.env` mtime + key TTL < today → expired.
3. Confirm at `https://login.tailscale.com/admin/settings/keys` — if status is *Expired*/*Revoked* or the key is absent, that is the bug.
4. Generate replacement: **Reusable** + **Ephemeral**, max TTL (90d for Tailscale SaaS). Push new value to every client `.env`.

**Rule:** With `Ephemeral: true` (the current ts-bridge default and the design intent), auth-key rotation is not optional — it's a recurring operational task tied to the key TTL. The TTL is a hard ceiling: max 90d on Tailscale SaaS. A scheduled reminder at TTL−7d is the lightweight mitigation; the heavier path is a small automation that monitors the Tailscale API and emits a new key.

**Tags:** `#tailscale` `#ephemeral` `#auth-key` `#operational`
