---
id: "ARCH-186-multi-target-socks5"
type: spec
status: draft
created: "2026-08-14"
issue: "ts-bridge#186"
tags: [spec, tasks, architecture, socks5]
template_version: "1.0"
---

# Tasks - ARCH-186: Multi-Target SOCKS5 Dynamic Proxy

## Setup

- [x] Spec created in `specs/ARCH-186-multi-target-socks5/`
- [x] Gating issue verified: #186

## Phase 1: Architecture & Documentation (Complete)

- [x] [AC1] Author ADR-014 (`docs/adr/adr-014-socks5-dynamic-mesh-proxy.md`).
- [x] [AC2] Author operational runbook (`docs/runbooks/guide-multi-target-socks5.md`) with SSH and kubectl recipes.
- [x] [AC3] Document Headscale control plane ACL policy contract.

## Phase 2: Implementation (Follow-up PR)

- [ ] Implement SOCKS5 proxy server in `internal/proxy` using `tsnet.Server.Dial`.
- [ ] Add `--socks5` flag and `TS_SOCKS5_ADDR` configuration support in `cmd/cli/connect.go` and `internal/config`.
- [ ] Add unit tests for SOCKS5 handshake, address parsing, and dial error handling.
