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
**Solution:** From corporate networks with transparent TLS inspection, Headscale TCP passthrough is fundamentally incompatible. Options: (1) Use the supported `ts-bridge connect --bootstrap-ssh deployer@VPS_IP` mode, which creates an OpenSSH dynamic SOCKS path for the configured control-plane hostname without editing `hosts`; (2) use Tailscale SaaS from networks where its control path is allowed; (3) test Headscale migration from a non-corporate network; (4) request an IT exception for the control-plane endpoint. The older manual `ssh -N -L ...` plus hosts-file workaround is retained only as a diagnostic fallback. Rule: Before planning Headscale migration for a device, verify the network allows direct TLS to the control plane. Corporate networks with DPI/TLS inspection will block the Noise protocol even though standard HTTPS to the same IP works. The giveaway is: same IP, mlorente.dev works, vpn.kubelab.live doesn't, no proxy configured, SSH works.

**2026-09-24 correction:** Tailscale SaaS is not universally exempt from the
inspection boundary. A managed corporate workstation can trust the enterprise
inspection CA while an unmanaged Windows host using the same corporate Wi-Fi
or an Internet Connection Sharing gateway cannot. The unmanaged host then
retains its old `100.x` address but reports itself offline with
`fetch control key ... x509: certificate signed by unknown authority`.
Internet access, a running Windows service, and a saved Tailscale IP are not
proof of a live tailnet session. Check `tailscale status`; use a network without
TLS interception, or have IT install the authorized enterprise CA in the
Local Machine trust store. Do not bypass certificate validation.

**2026-09-28 addendum:** A second, earlier-stage symptom of the same corporate
boundary, found while diagnosing a `ts-bridge connect` failure against
`vpn.kubelab.live` from a "Teledyne-Guest" Wi-Fi network. This time the
connection never got far enough to hit the TLS handshake: `nslookup`/
`Resolve-DnsName` for `vpn.kubelab.live` (and even the bare `kubelab.live`
apex) returned a `sinkhole.paloaltonetworks.com` CNAME -- from every resolver
tried, including public ones explicitly queried by IP (`1.1.1.1`, `8.8.8.8`).
That looked like a DNS record problem on the domain itself until the same
network sinkholed `google.com`'s and `tailscale.com`'s DNS-over-HTTPS
endpoints too (empty response, not even a TLS alert) while their plain-DNS
answers came back correctly -- proof the filtering happens transparently at
the network layer (any query for `1.1.1.1:443`/`dns.google:443` from this
Wi-Fi is intercepted or dropped), not by poisoning one specific domain.
Palo Alto's DNS Security subscription commonly auto-sinkholes newly
registered or "uncategorized" domains by policy default, which a personal
`.live` apex is likely to be classified as. **Diagnostic fingerprint:** a
domain resolves to `sinkhole.paloaltonetworks.com` from every DNS path
including DoH, while well-known domains on the same network resolve fine.
**Rule:** before assuming a Headscale domain's DNS records are broken, repeat
the resolution from a network without corporate DNS filtering (mobile
hotspot, home Wi-Fi) -- a network-level sinkhole and a real DNS misconfiguration
produce an identical-looking CNAME answer. Regenerating the Tailscale/Headscale
auth key does nothing for this failure mode; the request never reaches the
control plane.

**Tags:** `#headscale` `#tailscale` `#corporate-firewall` `#tls-inspection` `#networking` `#traefik` `#tcp-passthrough`
