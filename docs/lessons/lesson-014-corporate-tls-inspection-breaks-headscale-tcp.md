---
id: lesson-014-corporate-tls-inspection-breaks-headscale-tcp
type: lesson
status: active
created: "2026-03-13"
owner: manu
tags: [ts-bridge, lesson, headscale, tailscale, corporate-firewall, tls-inspection, networking, traefik, tcp-passthrough]
---

# Corporate TLS Inspection Breaks Headscale TCP Passthrough

**Context:** Migrating a corporate Windows workstation from Tailscale SaaS to self-hosted Headscale (vpn.kubelab.live). The VPS uses Traefik TCP passthrough with SNI routing so Headscale handles TLS termination and the Noise protocol directly.
**Problem:** tailscale up --login-server=https://vpn.kubelab.live hung indefinitely. Health check showed: fetch control key: wsarecv: connection forcibly closed. Diagnosis took multiple steps: (1) Headscale container running and healthy, (2) TLS certs valid (expire May 2026), (3) Traefik TCP passthrough config correct, (4) curl from VPS through full Traefik path worked (TLS 1.3 OK), (5) curl from Windows failed with schannel: failed to receive handshake, (6) mlorente.dev (same VPS IP, Traefik HTTP routing) worked, (7) direct IP also failed, (8) alternate port 8443 also failed, (9) no proxy configured (netsh/env/registry all empty), (10) SSH on port 22 worked fine. Root cause: corporate network has transparent TLS inspection (inline firewall, not a configured proxy). The firewall MITMs all TLS connections, decrypts traffic, and validates it is HTTP. For mlorente.dev, Traefik terminates TLS and serves standard HTTP — firewall allows it. For vpn.kubelab.live, Traefik does TCP passthrough to Headscale, which serves the Tailscale Noise protocol (binary, not HTTP) after TLS — firewall detects non-HTTP content and kills the connection. SSH works because it uses its own encryption protocol that the firewall cannot MITM.
**Solution:** From corporate networks with transparent TLS inspection, Headscale TCP passthrough is fundamentally incompatible. Options: (1) Use Tailscale SaaS from corporate networks (their relays use Cloudflare/Akamai IPs that firewalls whitelist), (2) SSH tunnel workaround (ssh -N -L 9443:localhost:8443 deployer@VPS_IP + hosts file entry + TS_CONTROL_URL with tunnel port), (3) Test Headscale migration from non-corporate networks (home workstation msi is already on the mesh at 100.64.0.1), (4) Request IT exception for the VPS IP on port 443. Rule: Before planning Headscale migration for a device, verify the network allows direct TLS to the control plane. Corporate networks with DPI/TLS inspection will block the Noise protocol even though standard HTTPS to the same IP works. The giveaway is: same IP, mlorente.dev works, vpn.kubelab.live doesn't, no proxy configured, SSH works.
**Tags:** `#headscale` `#tailscale` `#corporate-firewall` `#tls-inspection` `#networking` `#traefik` `#tcp-passthrough`
