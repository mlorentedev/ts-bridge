---
id: lesson-024-half-close-idleconn-embedding-type-assertions
type: lesson
status: active
created: "2026-05-28"
owner: manu
tags: [ts-bridge, lesson, go, interfaces, embedding, net-conn]
---

# Half-close + idleConn embedding: type assertions need unwrap

**Context:** Adding `CloseWrite()` support to `proxyConnections` (ARCH-004-follow-on) hit a subtle Go gotcha. `idleConn struct{ net.Conn; ... }` embeds the `net.Conn` *interface*, not a concrete type. Embedded interfaces do NOT promote concrete methods from whatever implementation is held — so even if the underlying `*net.TCPConn` has `CloseWrite()`, the wrapper does not satisfy `interface{ CloseWrite() error }`.

**Finding:** The type assertion `dst.(halfCloser)` against the wrapped conn returns `(_, false)`. Two cleanups possible:
1. Add `CloseWrite()` method to `idleConn` that delegates — but then the wrapper "claims" the capability even for inner conns that lack it, and the test path silently no-ops.
2. Unwrap before asserting: `if ic, ok := c.(*idleConn); ok { c = ic.Conn }; if cw, ok := c.(halfCloser); ok { ... }`. Correct, honest. Costs 2 lines.

Chose unwrap. Production code reflects truth; test path falls back to full `Close()` (which is what `net.Pipe` requires anyway).

**Rule:** When wrapping `net.Conn` via interface embedding, any optional interface method (`CloseWrite`, `SetDeadline` family that the inner may or may not support) needs an explicit unwrap-before-assert at the call site. Embedding promotes interface methods (those on `net.Conn` itself) but not extra ones on concrete implementations. The Go FAQ and stdlib `net/http` source both follow this pattern.

**Tags:** `#go` `#interfaces` `#embedding` `#net-conn`
