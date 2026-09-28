---
id: "ARCH-186-multi-target-socks5"
type: spec
status: draft
created: "2026-08-14"
issue: "ts-bridge#186"
tags: [spec, architecture, networking, socks5, multi-target, kubelab]
template_version: "1.0"
---

# ARCH-186: Multi-Target Connectivity via SOCKS5 Dynamic Proxy

<!-- from issue #186: Multi-target connectivity: reach a whole tailnet from one bridge (SSH + kubectl) -->

## Why

Currently, `ts-bridge` forwards a single TCP target per instance (`connect --target host:port`). Reaching an entire mesh network (such as a Headscale kubelab cluster) requires connecting to multiple hosts and ports (SSH `:22`, kubectl `:6443`, HTTP services) from a non-admin client machine without OS virtual network adapters (TUN/TAP).

## What

1. **Architecture Decision (ADR-014)**:
   - Establish the SOCKS5 dynamic proxy model using `tsnet.Server.Dial("tcp", target)`.
   - Reject static port ranges and OS TUN/TAP adapters.
2. **Operational Runbook**:
   - Provide concrete configuration recipes for OpenSSH (`ProxyCommand`), Kubernetes (`proxy-url` / `HTTPS_PROXY`), and curl.
   - Document Headscale/Tailscale ACL policy contracts for tag permissions.
3. **Implementation Plan**:
   - Add SOCKS5 listener support in `internal/proxy` or `cmd/cli/connect.go` (`--socks5` flag / `TS_SOCKS5_ADDR`).
   - Forward inbound SOCKS5 CONNECT requests dynamically to mesh destinations via tsnet.
4. **Browser-access profile**:
   - Add an explicit profile route from a canonical HTTPS origin to a permitted
     mesh destination, for example
     `forge.example.internal -> apps:443`.
   - Preserve the browser's original TLS SNI and HTTP Host while ts-bridge
     forwards raw TCP to the mapped mesh destination.
   - Provide a browser launch/profile command that uses a local PAC endpoint
     and an isolated browser profile. Only configured origins use the
     loopback SOCKS5 proxy; all other browser traffic remains direct.
   - Reject an origin that is absent from the profile allow-list. Do not
     mutate the Windows hosts file, system DNS, system proxy, or browser's
     primary user profile.

## Acceptance Criteria

- [AC1] ADR-014 written and accepted in `docs/adr/adr-014-socks5-dynamic-mesh-proxy.md`.
- [AC2] Operational recipes for SSH and kubectl documented in `docs/runbooks/guide-multi-target-socks5.md`.
- [AC3] Headscale / Tailscale ACL contract documented.
- [AC4] A loopback-only SOCKS5 listener accepts RFC 1928 CONNECT requests and
  dials permitted mesh targets through tsnet.
- [AC5] A browser profile maps an allow-listed HTTPS origin to its declared
  mesh target while preserving the original TLS SNI and HTTP Host.
- [AC6] A dedicated browser profile reaches a configured private-forge URL
  through its local PAC/SOCKS configuration without changing Windows hosts,
  DNS, system proxy settings, or the user's normal browser profile.
- [AC7] An unlisted browser origin is not forwarded through the mesh and
  produces an actionable denial.
