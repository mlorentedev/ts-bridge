---
id: "ARCH-004"
type: spec
status: archived
created: "2026-05-18"
archived: "2026-05-18"
merged_pr: 24
released_in: v1.7.0
subsumes: [ARCH-005]
tags: [spec, proposal, reliability, reconnect, dialer]
template_version: "1.0"
---

# ARCH-004: Auto-retry dial with exponential backoff

> Companion spec to the vault design doc [[task-arch-004-reconnect]] (canonical).
> This file captures the proposal-shaped subset and the in-flight decisions.

## Why

Today, when `tsnet.Server.Dial(target)` fails for a transient reason (DERP relay flap, target host briefly unreachable, brief target-side socket exhaustion), the client connection is closed immediately and the error is counted as a transport fault. The user sees the bridge drop their RDP session over what is effectively network noise. Vault backlog [[11-tasks]] **ARCH-004** (P1) tracks this; **ARCH-005** (P2) was the sibling for target-dial retries, expected to collapse — confirmed below.

## What

Add a `ReconnectDialer` decorator that wraps any `proxy.Dialer` and retries `Dial()` with exponential backoff + jitter on transient errors. Wire it around the `*tsnet.Server` in `main.go` before passing to `AcceptLoop`. Expose three opt-in env vars:

- `TS_DIAL_RETRIES` (default `3`, `0` disables)
- `TS_DIAL_BACKOFF_BASE` (default `1s`)
- `TS_DIAL_BACKOFF_MAX` (default `30s`)

Existing behavior preserved when `TS_DIAL_RETRIES=0`.

## Out of scope

- **Tier 2 control-plane supervisor.** `tsnet.Server.awaitRunning()` blocks until the backend returns to `Running`, which covers DERP/magicsock transients. Only terminal states (`Stopped`, `NeedsMachineAuth`) need external supervision, and those are rare enough to defer. If they appear in practice, escalate to a follow-up spec.
- Multi-target failover, bandwidth shaping, target health checks — eliminated in vault backlog.
- ARCH-005 (separate spec for target retry) — **subsumed into this PR**, the same `ReconnectDialer` covers both control-plane and target failures from the bridge's perspective.

## Design questions resolved

Per the canonical vault doc's "Open design questions" section, validated by reading `tailscale.com/tsnet@v1.80.0/tsnet.go`:

- **Q1: Does tsnet self-heal?** Mostly yes. `Dial` calls `awaitRunning`, which watches `ipn.Notify` state changes and blocks until backend is `Running`. DERP/magicsock layer has independent reconnect logic. → No supervisor needed for transients.
- **Q2: What errors come from `Dial`?** Three classes: `ctx.Err()` during context cancellation, `"tsnet: backend in state %v"` on terminal states (permanent — no retry), and stdlib `net.OpError` from `UserDial` (transient — retry).
- **Q3: ARCH-005 collapse?** Confirmed. Single retry layer at the Dialer interface covers both motivations.

## Risks / open questions

- **`math/rand` global state.** Use `rand.New(rand.NewSource(...))` per dialer instance to avoid global lock contention and make tests deterministic when needed. Resolved by design.
- **Backoff overflow.** `1 << attempt` overflows at 63. Cap `attempt` at 30 before shifting. Resolved.
- **Metric pollution.** Retries happen inside `Dial()` and are invisible to `handleConn`'s `AddTotalConnection()`. Verified — `handleConn` calls `dialer.Dial` once. Resolved.
- **Logging volume.** First retry attempt at `debug`, subsequent at `warn`. Resolved.

## Acceptance criteria

- [ ] `internal/proxy/reconnect.go` implements `ReconnectDialer`, `isPermanentDialError`, `computeBackoff`.
- [ ] `internal/proxy/reconnect_test.go` covers the seven test cases from the canonical doc's test plan (success on first attempt, success after N failures, give-up after max, permanent error short-circuit, context cancellation aborts loop, jitter bounded, MaxRetries=0 disables).
- [ ] `internal/config/config.go` parses `TS_DIAL_RETRIES`, `TS_DIAL_BACKOFF_BASE`, `TS_DIAL_BACKOFF_MAX` with defaults `3` / `1s` / `30s` and rejects invalid/negative.
- [ ] `main.go` wraps the `*tsnet.Server` with `&proxy.ReconnectDialer{...}` before `AcceptLoop`.
- [ ] `.env.example` documents the three new vars (commented optional).
- [ ] `go test ./...` green with no regressions. `go vet` clean.
- [ ] PR <300 lines diff excluding tests.
- [ ] In the PR description: mark ARCH-005 as subsumed and update vault `11-tasks.md` accordingly.

## References

- Canonical design: vault `10_projects/ts-bridge/30-architecture/task-arch-004-reconnect.md`
- Vault backlog: `11-tasks.md` ARCH-004 (P1), ARCH-005 (P2 — collapses here)
- Related ADR: vault `30-architecture/adr-006-dialer-interface-extraction.md` (the `Dialer` interface this builds on)
- Source-of-truth on tsnet: `tailscale.com/tsnet@v1.80.0/tsnet.go` lines 168-206 (`Dial`/`awaitRunning`)
