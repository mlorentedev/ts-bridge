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

Scope of the four items below: **documentation deliverables only**. They are ticked because the ADR
and the runbook exist, not because the proxy exists — `ts-bridge` has no SOCKS5 implementation
(`connect --socks5` is rejected by the binary, measured 2026-09-21 on `master`), so issue #186 stays
open and nothing here may be read as a delivered feature. See `docs/lessons/lesson-030-2026-08-31.md`
on ticking a criterion against the artifact rather than the document that describes it.

- [x] ADR-014 authored in `docs/adr/adr-014-socks5-dynamic-mesh-proxy.md`.
- [x] SOCKS5 runbook with SSH and kubectl recipes documented in `docs/runbooks/guide-multi-target-socks5.md`.
- [x] Headscale ACL contract documented.
- [x] `AGENTS.md` updated with ADR-014 entry.
