---
id: "ARCH-186-multi-target-socks5"
type: spec
status: draft
created: "2026-08-14"
issue: "ts-bridge#186"
tags: [spec, verification, architecture, socks5]
template_version: "1.0"
---

# Verification - ARCH-186: Multi-Target SOCKS5 Dynamic Proxy

## Evidence Checklist

The implementation evidence below is local to the feature worktree until the
change is reviewed and merged. Browser access is an explicit extension of the
SOCKS5 implementation, not a manual hosts-file workaround. See
`docs/lessons/lesson-030-2026-08-31.md` on ticking a criterion against the
artifact rather than the document that describes it.

- [x] ADR-014 authored in `docs/adr/adr-014-socks5-dynamic-mesh-proxy.md`.
- [x] SOCKS5 runbook with SSH and kubectl recipes documented in `docs/runbooks/guide-multi-target-socks5.md`.
- [x] Headscale ACL contract documented.
- [x] `AGENTS.md` updated with ADR-014 entry.
- [x] SOCKS5 handshake, parsing, loopback binding, and dial-error tests pass.
- [x] The proxy hands an allow-listed mesh target to the tsnet-compatible
  `Dialer` abstraction.
- [x] Browser-route validation maps the configured forge origin to its mesh
  target without an operating-system DNS lookup and preserves a TLS client
  stream.
- [x] A local TLS integration test confirms the remote endpoint receives the
  original canonical hostname in ClientHello SNI after mesh-target mapping.
- [x] PAC generation directs only allow-listed origins through the loopback
  SOCKS proxy. An unlisted start URL returns an actionable allow-list error;
  later unlisted navigation remains direct and cannot use the mesh route.
- [x] Edge launch arguments use an isolated profile, local PAC, and resolver
  rules without changing Windows hosts, DNS, or system proxy.
- [x] A live Apps/private-forge smoke runs using an auth-key file and records
  no credentials or confidential forge content.

## Live KubeLab Browser Smoke

Date: 2026-09-28

- Control plane: `https://vpn.kubelab.live`
- Browser origin: `gitea.kubelab.live:443`
- Mesh target: `kubelab-vps:443`
- Credential handling: a freshly generated Headscale key was supplied only
  through a local auth-key file; its value was never read or printed.
- Browser result: the isolated Edge profile launched and the operator confirmed
  the private forge was reachable.
- Bridge result:
  `READY local=127.0.0.1:1080 target=dynamic-socks5`.
- Independent route probe:
  `curl --socks5-hostname 127.0.0.1:1080 https://gitea.kubelab.live/api/healthz`
  returned HTTP `200` with exit code `0`; response content was discarded.
- Independent adversarial review: waived by the operator on 2026-09-28 because
  no approved reviewer was operationally available.
