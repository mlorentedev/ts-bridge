---
id: "BUG-001"
type: spec
status: draft
created: "2026-06-26"
issue: "ts-bridge#208"
tags: [spec, bug, reliability, proxy, health]
template_version: "1.0"
---

# BUG-001: Exit non-zero and surface dead state when tsnet session dies at runtime

## Why

When the upstream tsnet session dies at runtime (auth-key expiry/revocation, `NeedsMachineAuth`,
`Stopped`), `ts-bridge connect` keeps its local TCP listener accepting. Every forwarded connection
immediately resets at the remote handshake (`kex_exchange_identification: Connection reset by peer`)
with no indication that the bridge is functionally dead. Port-based liveness checks return false
"healthy" and automation that waits for the listener re-enters a permanent failure loop.

`isPermanentDialError()` in `internal/proxy/reconnect.go` already classifies `"tsnet: backend in
state "` as a permanent error — the detection building block exists. What is missing is the
**propagation path**: the error stays inside `ReconnectDialer.Dial()`, never surfaces to the
accept loop or the health server.

## Design decision (session 2026-06-26)

**B + C:**
- **B** — Cancel the root context when a terminal tsnet error is detected; process exits non-zero
  after graceful drain (same shutdown path as SIGTERM).
- **C** — `/health/ready` returns 503 with the terminal reason in the JSON body before exit.

Option A (stop accepting without exiting) is implicit: cancelling the root context already stops the
`AcceptLoop` (it selects on `ctx.Done()`). Explicit listener close is not needed.

**Why not retry forever:** tsnet already self-heals DERP/magicsock transients via `awaitRunning`
(see `docs/lessons.md` 2026-05-18). By the time an error reaches `isPermanentDialError`, tsnet has
already decided the state is terminal. Retrying at our layer is futile.

## What

1. **`TerminalDialError` sentinel** — a typed error wrapping a permanent tsnet error, distinct from
   "ran out of retries on a transient". `ReconnectDialer.Dial()` returns it when
   `isPermanentDialError` fires; the caller can `errors.As` to distinguish.
2. **Cancel propagation** — the accept loop (or caller of `Dial`) detects `TerminalDialError` and
   calls the root context's `CancelCauseFunc` with the terminal reason. This stops the accept loop,
   triggers graceful drain, and exits the process non-zero via the existing shutdown path.
3. **Health state with reason** — extend the health server from `atomic.Bool` to a small
   `TunnelStatus` (ready bool + reason string), set to `{ready: false, reason: "<tsnet state>"}` on
   terminal error. `/health/ready` returns 503 with `{"status":"not_ready","reason":"..."}`.
4. **Structured log** — emit `slog.Error("tsnet session terminal", "reason", ...)` with the same
   reason string before cancellation.

## Out of scope

- #203 (READY line at startup) and #204 (structured exit on startup failure) — both cover startup,
  not runtime. This spec covers runtime death only.
- Automatic reconnect / restart of the tsnet session — that is a separate architectural decision
  (ARCH-004 level). This spec makes the death visible and clean; restart is the operator's job.
- Detection of transient death that later recovers: tsnet's `awaitRunning` handles this internally;
  we only act on errors that escape it.

## Acceptance criteria

- [ ] After tsnet session dies (simulated via a mock Dialer returning a terminal error), `connect`
      exits with a non-zero code within one graceful-drain timeout.
- [ ] The exit log contains `slog.Error("tsnet session terminal")` with a `reason` field matching
      the tsnet state string.
- [ ] `/health/ready` returns 503 with `{"status":"not_ready","reason":"..."}` after the terminal
      event and before process exit.
- [ ] A `TerminalDialError` is distinguishable via `errors.As` from a plain "retries exhausted"
      error — unit-tested.
- [ ] A transient error (non-permanent) still retries up to `MaxRetries` and does NOT trigger
      context cancellation — regression test.
- [ ] Existing `ReconnectDialer` retry tests still pass (no behaviour change for transient path).
- [ ] `isPermanentDialError` coverage: `"tsnet: backend in state NeedsMachineAuth"` and
      `"tsnet: backend in state Stopped"` both classified as terminal.

## Risks / open questions

- **[RESOLVED] Shutdown path reuse.** The existing graceful drain (SIGTERM path) already cancels
  the root context and calls `drainActiveConnections`. Reusing it means zero new shutdown code —
  just trigger the same cancellation from the proxy layer.
- **[RESOLVED] Health server signature change.** `StartServer` currently takes `*atomic.Bool`.
  Changing to `*TunnelStatus` (a struct with `Load`/`Store` methods) is a breaking change to the
  call site in `main.go` — one-liner fix, no other callers.
- **[OPEN] Key-expired error string.** `isPermanentDialError` matches `"tsnet: backend in state "`.
  Auth-key expiry may surface a different string (e.g., from the config-layer rejection added in
  #209). Verify the actual error path in integration and extend the matcher if needed.

## References

- Issue: ts-bridge#208
- Related: ts-bridge#203 (READY line, startup), ts-bridge#204 (structured exit, startup)
- Lesson: `docs/lessons.md` 2026-05-18 — `tsnet.Server.Up()` partial-start, `awaitRunning` self-heal
- Touch points: `internal/proxy/reconnect.go`, `internal/health/health.go`, `main.go`
