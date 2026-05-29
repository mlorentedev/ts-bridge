---
id: "ADR-001-tsnet-userspace"
type: adr
status: accepted
date: "2024-01-01"
tags: [architecture, decision, networking, tailscale]
owner: manu
created: "2026-03-28"
---

# ADR-001: Use tsnet (Userspace Networking) Instead of Native Tailscale

## Context
ts-bridge needs to create Tailscale connections from machines where the user has no administrator privileges. Native Tailscale requires kernel-level network interface creation (TUN device), which demands admin/root access and leaves system-level traces.

The primary use case is corporate environments where users cannot install software but need to tunnel RDP/SSH connections through restrictive firewalls.

## Options Considered
1. **Native Tailscale client**
    * *Pros:* Full Tailscale feature set, direct WireGuard performance, official support.
    * *Cons:* Requires admin/root for TUN device, needs installation, leaves system traces, defeats the project's purpose.
2. **tsnet (userspace networking library)**
    * *Pros:* No admin required, no kernel modules, embeddable in Go binary, ephemeral nodes, portable.
    * *Cons:* Limited to TCP (no UDP/ICMP), slightly higher latency, less documented API.
3. **Manual WireGuard userspace implementation**
    * *Pros:* No Tailscale dependency.
    * *Cons:* Massive effort, no mesh coordination, no DERP relay, no NAT traversal.

## Decision
We chose **tsnet** because it is the only option that satisfies the core requirement: Tailscale connectivity without administrator privileges. It provides WireGuard encryption, DERP relay fallback, and ephemeral node behavior — all in a single embeddable Go library.

## Consequences
- **Positive:** Zero-install portable binary, no admin footprint, automatic DERP relay through HTTPS when UDP is blocked, ephemeral nodes auto-cleanup.
- **Negative:** TCP-only (no UDP protocols like VoIP), slightly higher latency (~50-200ms via DERP), smaller community for tsnet-specific issues.

## References
- https://pkg.go.dev/tailscale.com/tsnet
- https://tailscale.com/blog/tsnet-virtual-private-services
