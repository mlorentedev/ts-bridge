---
id: "ADR-011-shareable-connection-profile"
type: adr
status: accepted
date: "2026-06-23"
tags: [architecture, decision, configuration, discovery, headscale, ux]
owner: manu
issue: "mlorentedev/ts-bridge#213"
created: "2026-06-23"
---

# ADR-011: Shareable Connection Profile for Host→Client Port Propagation

## Context

ts-bridge has no channel to propagate a host's real service port to clients. The
host already knows its actual RDP port — `host setup` reads it from the registry
(`HKLM\...\WinStations\RDP-Tcp\PortNumber`, e.g. `45000`) — but the client side
has no way to learn it:

- `connect` reads `TS_TARGET=host:port` from `.env`, where the port is hand-typed.
- `discover` defaults its `--port` flag to `3389` and never queries the host's
  real port.

When a host's RDP port differs from 3389 (moving RDP off the default port is a
common hardening step), `connect` dials the wrong port and fails with
`connection was refused`. The operator must reverse-engineer the port by hand —
a real friction incident that motivated this ADR.

The information asymmetry is the core problem: **the host knows the port; the
client guesses it; nothing carries the value across.** Closing the gap is
inherently a host+client feature.

### Hard constraints

- **C3 — control-plane agnostic** (ADR-005): must work identically on Tailscale
  SaaS and self-hosted Headscale (kubelab).
- **C4 — daemonless host** (ADR-002): `host setup` configures the machine and
  exits; there is no persistent ts-bridge process on the host to serve metadata.
- **C1 — locked-down hosts** (ADR-002): the host may lack admin rights and may
  be unable to change its RDP port.
- **C5 — backward compatible**: existing `TS_TARGET=host:port` must keep working;
  the feature is additive/opt-in.
- **C6 — security**: do not undo deliberate hardening (e.g. force RDP back onto
  the most-scanned port 3389) and avoid requiring broad API credentials.
- **C7 — compose with #185** (CFG-001 profiles model), not a parallel mechanism.

## Options Considered

1. **A — Publish the port via control-plane device metadata.** `host setup` writes
   the detected port as a Tailscale custom posture attribute
   (`POST /api/v2/device/{id}/attributes/custom:tsbridge_rdp_port`); the client's
   `discover` reads it (`GET .../attributes`). Zero manual transfer.
   - *Pros:* truly zero-touch on the client.
   - *Cons:* **Headscale does not support device posture / custom attributes**, so
     this breaks ADR-005 parity (C3). Also requires API access tokens (write on
     the host, read on the client) beyond the auth keys already in play (C6), and
     couples the design to Tailscale SaaS.

2. **B — Shareable connection profile (CHOSEN).** `host setup` emits a
   self-describing connection descriptor that carries the *detected* port
   (`{hostname-or-IP, port, control-plane}`). The client imports it once into a
   named profile; `connect --profile <name>` consumes it. The port travels inside
   a ts-bridge artifact, not the control plane.
   - *Pros:* control-plane agnostic (C3) — identical on Tailscale and Headscale;
     daemonless (C4); no extra API credentials (C6); the descriptor *is* a
     profile, so it composes directly with #185 (C7); backward compatible (C5).
   - *Cons:* one host→client transfer of the descriptor per host (see Open
     Questions for how to make that transfer low-friction).

3. **C — Standardize/force a fixed RDP port.** `host setup` rewrites the host's
   RDP port to a fixed value; the client assumes it.
   - *Pros:* trivial; no propagation needed.
   - *Cons:* requires host admin (C1) — not guaranteed on locked-down machines;
     re-exposes the most-scanned port and undoes deliberate hardening (C6);
     ts-bridge cannot always change a host's RDP port.

### Options vs constraints

| Option | C1 admin-free | C3 agnostic | C4 daemonless | C5 compat | C6 security | C7 #185 |
|---|---|---|---|---|---|---|
| A metadata | ok | **gap (Headscale)** | ok | ok | gap (API tokens) | partial |
| **B profile** | ok | **ok** | ok | ok | ok | **ok** |
| C force port | **gap (admin)** | ok | ok | ok | **gap (hardening)** | partial |

## Decision

Adopt **Option B**: a shareable connection profile as the host→client port
propagation mechanism. The detected port travels inside a ts-bridge-defined
descriptor that the host emits and the client imports, composing with the named
profiles model in #185 (CFG-001).

`host setup` (and `host check`) emit the descriptor including the detected port;
the client gains an import path that materializes a named profile, consumed by
`connect --profile <name>`. The existing `TS_TARGET=host:port` path remains valid.

### Transfer channel (default + spike)

The descriptor must move host→client. Decided default:

- **Floor (T0 + T1), no feasibility risk:** `host setup` *surfaces* the detected
  port prominently (and the exact `discover --port <n>` / `TS_TARGET` line) **and**
  emits a copy-paste token (`tsb://...`) the client can `import`. Universal —
  works on Tailscale and Headscale, today, with no extra credentials. Note that
  since `discover` already resolves the host name/IP via the control plane, the
  payload that truly must travel is *just the port*, keeping the token tiny.
- **Spike (T2) — investigated 2026-06-23, demoted.** Source review of tsnet
  v1.80.0 shows Taildrop *receive* is mechanically wired (tsnet `SetVarRoot`
  → `fileRootLocked` resolves a real dir → `taildrop.Manager` exists in the
  peerAPI), **but receiving is gated on the control-plane capability**
  `CapabilityFileSharing` (`handlePeerPut` → `hasCapFileSharing()` ←
  `nm.HasCap(tailcfg.CapabilityFileSharing)`, local.go:5774). That makes T2
  Tailscale-strong / **Headscale-fragile — the same ADR-005 parity failure that
  rejected option A** — plus a chicken-and-egg (the client must already be a
  running, authenticated tsnet node to receive the bootstrap descriptor). T2 is
  therefore **not the universal path**; at most an optional Tailscale-only
  convenience, carrying option A's reopen caveat. **T0+T1 stands as the
  mechanism.**

### Rationale

C3 (Headscale parity, ADR-005) is decisive and evidence-backed: Tailscale supports
custom posture attributes but Headscale explicitly does not, eliminating A as a
sole mechanism. B keeps the port in a ts-bridge artifact, so it is indifferent to
the control plane — the only option satisfying every hard constraint. It also
folds into the profiles work already planned in #185 rather than adding a parallel
channel, and removes no existing capability.

## Consequences

- **Positive:** zero port-guessing — the client uses the host's *verified* port.
  Works on Tailscale SaaS and Headscale alike. No new host daemon, no extra API
  credentials. The descriptor doubles as the shareable unit for #185 profiles.
- **Negative:** one host→client transfer of the descriptor per host. The transfer
  channel is an open sub-decision (see below); the universal floor is a
  copy-paste token, with tailnet-native transfer as an optional enhancement.
- **Neutral:** introduces a ts-bridge-owned descriptor format that must be
  versioned and parsed on both ends.

### Rejected with reopen trigger

- **Option A** is rejected *for now*, not forever. **Reopen trigger:** if Headscale
  adds device posture / custom-attribute support, A becomes a viable opportunistic
  layer on Tailscale-and-compatible control planes (zero-touch where available,
  B as the universal fallback).

## Open Questions (to resolve in the spec)

- **T2 Taildrop spike — RESOLVED 2026-06-23 (see Transfer channel above).** tsnet
  can receive Taildrop, but it is gated on the control-plane `CapabilityFileSharing`
  → Headscale-fragile (same parity failure as option A) + chicken-and-egg. Demoted
  to an optional Tailscale-only convenience; **T0+T1 is the mechanism**. No further
  spike needed for the floor.
- **Descriptor format & versioning** (token grammar vs file schema).
- **Exact import surface** (`import` subcommand vs `discover --import` vs
  `connect --profile`), pending the #185 profiles model.

## References

- Issue: mlorentedev/ts-bridge#213 (CFG-002)
- [adr-002-single-binary-no-config.md](adr-002-single-binary-no-config.md) — env-var config; C1/C2/C4 origin
- [adr-005-headscale-compat.md](adr-005-headscale-compat.md) — C3 (Headscale parity), the decisive constraint
- #185 (CFG-001) — structured config & named profiles; this ADR composes with it
- Tailscale device posture attributes API: https://tailscale.com/kb/1288/device-posture
- Headscale lacks device posture support: https://headscale.net/stable/ref/policy/
