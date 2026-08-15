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

## Acceptance Criteria

- [AC1] ADR-014 written and accepted in `docs/adr/adr-014-socks5-dynamic-mesh-proxy.md`.
- [AC2] Operational recipes for SSH and kubectl documented in `docs/runbooks/guide-multi-target-socks5.md`.
- [AC3] Headscale / Tailscale ACL contract documented.
