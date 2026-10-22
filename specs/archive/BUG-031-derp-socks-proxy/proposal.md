---
id: "BUG-031-derp-socks-proxy"
type: spec
status: archived
created: "2026-10-19"
issue: "mlorentedev/ts-bridge#443"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# BUG-031-derp-socks-proxy

## Why

<!-- from issue #443: BUG-031: DERP sends HTTP CONNECT to the SSH SOCKS bootstrap proxy -->

The bootstrap hook advertises OpenSSH's SOCKS endpoint to every SDK consumer for
the control hostname, but the pinned DERP client speaks HTTP CONNECT there.
Consequently a same-host custom relay fails with unexpected EOF on a real
non-admin Windows connection. Repair the protocol boundary in application code,
not through machine-specific workarounds or edits to dependency caches.

## What

Expose a process-owned, ephemeral loopback HTTP proxy to the SDK. It forwards
CONNECT tunnels and ordinary HTTP requests through the existing OpenSSH SOCKS
listener, preserving remote hostname resolution and end-to-end TLS. Limit
requests to the configured control hostname; preserve fallback selection for
unrelated hosts. Tie listeners, outgoing connections and relays to run cancellation.

Use the already-pinned golang.org/x/net SOCKS dialer rather than inventing a
second SOCKS implementation. Production promotes this existing module to
direct without an upgrade. The real SDK server fixture additionally requires
four SDK-pinned indirect modules, used only in tests; this is justified by
verifying an authenticated DERP handshake instead of a protocol mock.
No new CLI flags or stored settings are needed.

## Out of scope

- No system proxy/DNS/hosts/firewall changes, elevation or SSH-agent management.
- No profile/credential migration, auth-key rotation, SDK-cache patch or upgrade.
- No promise that this alone resolves the separate SSH-only Gitea HTTP stall.
- No tunneling unrelated relay hostnames or modifying the active RDP session.

## Risks / open questions

- Relay cancellation must close hijacked sockets as well as the HTTP listener.
- Reject unrelated destinations even if another local process accesses the proxy.
- Preserve buffered bytes after CONNECT and surface upstream failures explicitly.
- Ordinary HTTP control URLs must remain supported, not just HTTPS/DERP.
- Tests must use the actual pinned SDK DERP consumer; URL assertions alone missed
  this defect. Use synthetic loopback SOCKS/TLS/DERP servers, not a live tailnet.

Design choice: a reversible adapter inside the existing ADR-005 boundary.
Rejected: editing the module cache (not reproducible), inferring consumers from
URL paths (fragile), and redirecting all traffic globally (breaks host scope).

## Acceptance criteria

- [x] AC1: Go control HTTPS and the actual pinned SDK DERP client both connect
  through a real synthetic SOCKS server with normal TLS verification and remote DNS.
- [x] AC2: Host scoping/fallback and HTTP control forwarding remain correct;
  disallowed destinations and failed SOCKS dials produce explicit errors.
- [x] AC3: Cancellation closes the proxy listener and active tunnels; startup
  configuration errors clean up without changing the installed hook.
- [x] AC4: Windows package tests/build pass. Live DERP acceptance is not part of
  this change: it is deferred to #441 (real HTTPS acceptance), which stays
  Blocked until it is run and recorded separately from Gitea's
  application-response acceptance.

## References

- #443; related acceptance #441 / #442.
- `docs/adr/adr-005-headscale-compat.md`, ADR-001/006/007/013/014/015.
- `tailscale.com v1.104.1`: `derphttp.Client.dialNodeUsingProxy`.
- Local language/architecture patterns; full 16/16 ADR read completed.
