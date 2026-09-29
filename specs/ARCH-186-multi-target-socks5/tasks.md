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

- [x] [AC4] Amend ADR-014 and the multi-target runbook to define browser
  profile routing, loopback-only binding, and the no-system-mutation rule.
- [x] [AC4] Write failing unit tests for RFC 1928 handshake, address parsing,
  loopback binding, and dial failure behavior.
- [x] [AC4] Implement the SOCKS5 proxy server in `internal/proxy` using
  `tsnet.Server.Dial`.
- [x] [AC4] Add `--socks5` and `TS_SOCKS5_ADDR` configuration support in
  `cmd/cli/connect.go` and `internal/config`.
- [x] [AC5] Write failing tests for origin-to-mesh route validation and
  byte-for-byte forwarding that preserves the client TLS stream.
- [x] [AC5] Add an allow-listed browser-route profile model that maps a
  canonical origin to a tailnet `host:port` target without resolving the
  origin through the operating system.
- [x] [AC6] Write failing tests for PAC generation and browser launch
  arguments using an isolated user-data directory.
- [x] [AC6] Implement a browser-profile command that exposes the loopback PAC
  endpoint and launches Edge with only the configured origins proxied.
- [x] [AC7] Add tests proving an unlisted start URL returns a structured error
  and later unlisted navigation is not forwarded through the mesh; confirm no
  route enables a global proxy or hosts-file mutation.
- [x] [AC4] [AC5] [AC6] [AC7] Run deterministic Go tests and an isolated
  local TLS integration test before the live Apps/Gitea smoke test.

## Phase 3: Live Evidence (Manual, No Secrets)

- [ ] [AC6] With a fresh key stored in an auth-key file, use the approved
  Apps descriptor to verify the canonical Gitea URL in the isolated browser
  profile. Record only the command result and URL, never the key, endpoint
  credential, or Gitea content.
