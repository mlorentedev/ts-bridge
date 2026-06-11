---
id: "adr-006"
type: adr
status: accepted
created: "2026-03-07"
tags: [architecture, testability, refactor]
owner: manu
---

# ADR-006: Extract Dialer Interface for Testability

## Context

`handleConn()` and `acceptLoop()` take `*tsnet.Server` directly, coupling the core proxy logic to tsnet. This makes unit testing impossible without a real Tailscale network. The wormhole project demonstrated that a clean transport interface (4 methods) enabled full mock-based testing of its core relay logic.

ts-bridge only needs one method from tsnet: `Dial(ctx, network, addr) (net.Conn, error)`.

## Decision

Extract a `Dialer` interface:

```go
// Dialer abstracts the remote connection mechanism.
// tsnet.Server satisfies this interface without an adapter.
type Dialer interface {
    Dial(ctx context.Context, network, addr string) (net.Conn, error)
}
```

`tsnet.Server` already has this exact signature, so **no adapter code is needed** — it satisfies the interface implicitly. Change `acceptLoop` and `handleConn` signatures from `*tsnet.Server` to `Dialer`.

## Consequences

- `handleConn` and `proxyConnections` become testable with a mock `Dialer` that returns `net.Pipe()` connections
- Opens the door for ARCH-004 (reconnection logic) by wrapping the dialer with retry behavior
- Zero runtime overhead — Go interfaces are checked at compile time, dispatched via vtable
- Maintains ADR-002 (single-binary, no config files) — this is an internal refactor only

## Alternatives Considered

- **Full transport interface** (Connect/Send/Receive/Close like wormhole): overkill — ts-bridge works at TCP level, not message protocol level. A single `Dial` method is sufficient.
- **Constructor injection via struct**: adds unnecessary ceremony for a single method.
