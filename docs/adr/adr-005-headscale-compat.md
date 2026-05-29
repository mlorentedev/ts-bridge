---
id: "ADR-005-headscale-compat"
type: adr
status: accepted
date: "2026-02-25"
tags: [architecture, decision, networking, headscale, vpn]
owner: manu
created: "2026-03-28"
---

# ADR-005: Add Headscale Control Plane Compatibility

## Context

ts-bridge currently uses tsnet which defaults to Tailscale's SaaS coordination server (`controlplane.tailscale.com`). The kubelab infrastructure runs a self-hosted Headscale instance (`vpn.kubelab.live`) as the VPN control plane for all homelab nodes (see kubelab ADR-010).

This creates a split-brain situation: homelab devices authenticate to Headscale, but ts-bridge instances authenticate to Tailscale SaaS. The two tailnets are invisible to each other — no direct mesh connectivity, no shared ACLs, no unified node management.

### The gap

tsnet's `Server` struct exposes a `ControlURL` field that overrides the default coordination server. ts-bridge currently initializes `tsnet.Server` without setting this field:

```go
// current code (main.go)
srv := &tsnet.Server{
    Hostname:  hostname,
    AuthKey:   cfg.AuthKey,
    Dir:       stateDir,
    Ephemeral: true,
}
```

Adding `ControlURL: os.Getenv("TS_CONTROL_URL")` would allow ts-bridge to connect to any Tailscale-compatible control plane, including Headscale.

### Use cases unlocked

1. **Homelab mesh unification** — Windows PCs running ts-bridge join the same Headscale tailnet as homelab nodes. Single ACL policy, single admin UI.
2. **Contractor access** — External contractors use ts-bridge (no native Tailscale install) with Headscale-issued pre-auth keys. Scoped ACLs restrict access to specific services.
3. **Zero vendor lock-in** — ts-bridge works with Headscale, Tailscale SaaS, or any future compatible control plane.

## Options Considered

1. **Environment variable (`TS_CONTROL_URL`)**
   * *Pros:* Consistent with tsnet conventions, zero config file needed, backward-compatible (empty = Tailscale SaaS).
   * *Cons:* None significant.
2. **CLI flag (`--control-url`)**
   * *Pros:* Explicit.
   * *Cons:* ts-bridge uses env vars exclusively (ADR-002). Adding CLI flags breaks the pattern.
3. **Config file**
   * *Cons:* Eliminated in ADR-002. Env vars are the correct approach for a portable binary.

## Decision

Add `TS_CONTROL_URL` environment variable support. When set, ts-bridge passes the value to `tsnet.Server.ControlURL`. When empty or unset, tsnet defaults to Tailscale SaaS (backward-compatible).

### Implementation

One-line change in `main.go`:

```go
srv := &tsnet.Server{
    Hostname:   hostname,
    AuthKey:    cfg.AuthKey,
    Dir:        stateDir,
    Ephemeral:  true,
    ControlURL: os.Getenv("TS_CONTROL_URL"),
}
```

Update `.env.example`:

```bash
# Control plane URL (optional, defaults to Tailscale SaaS)
# For Headscale: TS_CONTROL_URL=https://vpn.kubelab.live
TS_CONTROL_URL=
```

Update bootstrap scripts to include the variable.

## Consequences

- **Positive:** Enables VPN consolidation under a single Headscale control plane. Zero breaking changes for existing users — empty value preserves current behavior.
- **Positive:** ts-bridge becomes the standard way to connect non-admin devices (Windows PCs, contractor machines) to the Headscale mesh.
- **Negative:** Headscale pre-auth keys have different semantics than Tailscale auth keys (shorter expiry, reusable flag). Users must understand their control plane's key management.
- **Testing:** Requires manual testing against Headscale instance. Unit tests can verify the env var is read but cannot test actual Headscale connectivity.

## Related

- [adr-001-tsnet-userspace.md](adr-001-tsnet-userspace.md) — tsnet is the foundation that makes this possible
- [adr-002-single-binary-no-config.md](adr-002-single-binary-no-config.md) — Env var pattern (no config files)
- kubelab ADR-010 (Headscale as control plane) and ADR-013 (full VPN consolidation plan) — decided in a separate project; not linked here to preserve repo independence.
