---
id: lesson-016-headscale-migration-abandoned-tailscale-saas-
type: lesson
status: active
created: "2026-05-28"
owner: manu
tags: [ts-bridge, lesson, headscale, tailscale, corporate-firewall, tls-inspection, decision]
---

# Headscale Migration Abandoned — Tailscale SaaS is the Correct Solution for Corporate Networks

**Context:** Multi-day effort (Feb 25 – Mar 16) to consolidate all VPN nodes under a single Headscale control plane (ADR-013). The code worked (ts-bridge v1.4.0 with `TS_CONTROL_URL`), Traefik TCP passthrough was configured, tsnet was upgraded to v1.80.0, and registration was verified (100.64.0.11, ephemeral cleanup working).

**Problem:** Corporate networks have transparent TLS inspection (inline firewall, not a configured proxy). The firewall MITMs all TLS, decrypts, validates content is HTTP. Headscale serves Noise protocol (binary, not HTTP) after TLS → firewall kills connection. Same VPS IP with standard HTTPS works fine. SSH works (not TLS). Alternate ports also fail. No proxy configured. Diagnostic fingerprint: same IP, HTTPS site works, VPN fails, SSH works.

**Decision:** Abandon Headscale migration for corporate-network devices. Use Tailscale SaaS for all Windows hosts. The 3 PCs (acemagic-office, acemagic-lab-1, acemagic-lab-2) are configured with native Tailscale on the host side + ts-bridge client from both Linux and Windows workstations. All access is via RDP.

**Why this is the right call:**
1. Tailscale SaaS relay IPs are CDN/Cloudflare addresses — corporate firewalls whitelist them.
2. The `TS_CONTROL_URL` feature stays in ts-bridge for non-corporate use cases (homelab, personal network).
3. Headscale consolidation (ADR-013) remains valid for homelab nodes where there's no TLS inspection.
4. Zero operational complexity — Tailscale SaaS handles key rotation, relay infrastructure, and NAT traversal.

**Lesson:** Before planning any self-hosted VPN migration for corporate devices, verify the network allows direct TLS to the control plane. Transparent TLS inspection is invisible (no proxy settings, no certificates to install) but kills any non-HTTP protocol tunneled over TLS. The only reliable diagnostic is: "same IP, HTTPS works, binary protocol doesn't, SSH works."

**Artifacts archived:**
- `90_archive/ts-bridge/guide-headscale-migration.md`
- `90_archive/ts-bridge/guide-windows-host-to-headscale.md`
- `90_archive/ts-bridge/guide-windows-host-headscale-fresh.md`

**Tags:** `#headscale` `#tailscale` `#corporate-firewall` `#tls-inspection` `#decision`
