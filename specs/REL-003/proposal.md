---
id: "REL-003"
type: spec
status: implementing
created: "2026-05-18"
tags: [spec, proposal, idle-timeout, reliability]
template_version: "1.0"
---

# REL-003: Opt-in connection idle timeout

## Why

Abandoned RDP sessions (user closes the RDP client without logging out, laptop sleeps mid-session, network blip leaves a half-open TCP that survives TCP keepalive) consume a slot of `TS_MAX_CONNECTIONS` and a tsnet handle indefinitely. The existing 3-minute TCP keepalive detects *dead peers*, but not *live peers with no human activity* — both ends are healthy at TCP level, just no user data flows. Vault backlog entry [[11-tasks]] REL-003 (P1).

## What

Introduce `TS_IDLE_TIMEOUT` env var (`time.Duration`, default `0` = disabled). When set and > 0, ts-bridge enforces a per-direction read deadline: if either direction stays silent for the configured duration, both ends of the bridged connection are closed and the slot is released. The close is logged at `info` level (not `warn`/error) and not counted as a transport error.

## Out of scope

- ARCH-005 (transient dial retries) — separate task.
- Bidirectional-shared idle semantics (close only when *both* directions silent simultaneously). Per-direction is enforced here; if false positives are reported in practice, a follow-up can escalate to shared state.
- Hard maximum connection duration (TS_MAX_CONN_DURATION) — outside this spec.
- Idle behavior for the local listener (`AcceptLoop`) — only applies to accepted client/remote pairs.

## Risks / open questions

- **False positives on legitimate-but-quiet sessions.** Mitigation: opt-in (default 0), value selected by operator (30 min recommended starting point), per-direction deadline is documented so users understand semantics. Resolved.
- **`net.Conn.SetReadDeadline` on `tsnet` connections.** `tsnet.Server.Dial()` returns a `net.Conn` that supports `SetReadDeadline` (verified in tsnet source). No special-case needed. Resolved.
- **`io.CopyBuffer` interaction with timeout errors.** A timeout error from `Read` propagates out of `io.CopyBuffer` as the returned error. We classify it as an expected close (idle reached) and avoid the `warn` path. Resolved.
- **Interaction with `keepAliveInterval = 3min` on the TCP socket.** Independent mechanisms — TCP keepalive probes the L4 socket health; idle timeout monitors L7 traffic. Both can coexist. Resolved.

## Acceptance criteria

- [ ] `TS_IDLE_TIMEOUT` unset → `Config.IdleTimeout == 0` (disabled), no behavior change vs current.
- [ ] `TS_IDLE_TIMEOUT="5m"` → `Config.IdleTimeout == 5 * time.Minute`.
- [ ] `TS_IDLE_TIMEOUT="garbage"` → `LoadConfig` returns a non-nil error mentioning `TS_IDLE_TIMEOUT`.
- [ ] `TS_IDLE_TIMEOUT="-1m"` → `LoadConfig` returns a non-nil error (negative rejected).
- [ ] When `IdleTimeout > 0`, a paired `net.Pipe` simulation that sends one byte and then sleeps past the timeout sees the conn closed by the bridge within `idleTimeout + grace`.
- [ ] When `IdleTimeout > 0` and traffic flows continuously, the conn is NOT closed prematurely.
- [ ] The idle-close path emits a single `info` log entry containing `idle_timeout`, not a `warn` "copy error".
- [ ] Existing test suite passes unchanged (no regressions in `internal/config`, `internal/proxy`, integration tests, main pkg).

## References

- Vault: `10_projects/ts-bridge/11-tasks.md` REL-003
- Related pattern: `00_meta/patterns/pattern-spec-driven-development.md`
- Related code: `internal/proxy/proxy.go` (`handleConn`, `proxyConnections`)
- Related code: `internal/config/config.go` (`LoadConfig`)
