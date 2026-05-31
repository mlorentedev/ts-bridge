---
id: task-arch-004-reconnect
type: implementation-task
status: active
created: "2026-05-11"
---

# ARCH-004: Auto-Reconnect & Dial Retry

> **Status:** ✅ Implemented in v1.7.0 (PR #24, subsumes ARCH-005). Historical design record, migrated from the vault.
> **Priority:** P1
> **Depends on:** ARCH-001 (`Dialer` interface — done, PR #12), ARCH-003 (proxy unit tests — done)
> **Branch (suggested):** `feat/reconnect-dialer`
> **Linked:** [adr-006-dialer-interface-extraction](https://github.com/mlorentedev/ts-bridge/blob/master/docs/adr/adr-006-dialer-interface-extraction.md) · [stream-e-architecture](https://github.com/mlorentedev/ts-bridge/blob/master/docs/design/stream-e-architecture.md) (Phase 4)

## Goal

Make ts-bridge resilient to transient failures of the target connection and (where feasible) the tsnet control-plane tunnel, without breaking the single-binary, env-driven design (ADR-002) or the atomic-metrics constraint (ADR-004).

## Non-goals

- Bandwidth shaping / connection rate limiting (already eliminated in backlog).
- Multi-target failover (would require config redesign — eliminated as CFG-001).
- Replacing the `tsnet` library — keep using `*tsnet.Server`.

---

## Open design questions (resolve BEFORE coding)

These must be answered with evidence (code reading or a short empirical test), not guessed:

1. **Does `tsnet.Server` self-heal the control-plane tunnel after a drop?**
   - Read `tailscale.com/tsnet@v1.80.0` source, focus on the `Server` lifecycle and what happens when the magicsock loses its connection to the DERP/control plane.
   - If yes → ARCH-004 reduces to "wrap `Dial()` with retry" (target dial is the only thing we touch).
   - If no → we additionally need a supervisor that calls `server.Close()` + re-`Up()` on persistent failures.

2. **What error does `tsnet.Server.Dial()` return when the control plane is unreachable?**
   - Distinguish: target host is down (permanent for that target) vs. tunnel is broken (transient, retry).
   - Likely candidates to grep for in tsnet: `errNoPeers`, `errNoDERP`, deadline-exceeded.

3. **ARCH-005 collapse confirmation.** The backlog notes ARCH-005 (target retry) "likely collapses into ARCH-004 (same reconnectDialer wrapping pattern)." Validate this is still true after Q1/Q2 are answered. If it holds, mark ARCH-005 done in the same PR.

> **Action:** Spend ≤30 min on Q1 + Q2 first. Capture findings in a short comment on the PR or update this doc before writing code.

---

## Design (assuming tsnet does NOT self-heal — worst case)

Two-tier resilience:

### Tier 1 — `reconnectDialer` (target dial retry)

A `Dialer` decorator. Sits between `main.run()` and `proxy.AcceptLoop`. Retries `Dial()` failures with exponential backoff + jitter. This is the ARCH-005 behavior too.

```go
// internal/proxy/reconnect.go (new file)
type ReconnectDialer struct {
    Inner       Dialer
    MaxRetries  int           // 0 = no retry
    BaseBackoff time.Duration // e.g. 1s
    MaxBackoff  time.Duration // e.g. 30s
    Logger      *slog.Logger
}

func (r *ReconnectDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
    var lastErr error
    for attempt := 0; attempt <= r.MaxRetries; attempt++ {
        conn, err := r.Inner.Dial(ctx, network, addr)
        if err == nil {
            return conn, nil
        }
        lastErr = err
        if isPermanentDialError(err) || ctx.Err() != nil {
            return nil, err
        }
        backoff := exponentialBackoff(attempt, r.BaseBackoff, r.MaxBackoff) // 2^attempt * base, capped
        jitter := time.Duration(rand.Int63n(int64(backoff / 2)))            // rand(base/2)
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(backoff + jitter):
        }
        r.Logger.Warn("dial retry", "attempt", attempt+1, "err", err, "wait", backoff+jitter)
    }
    return nil, fmt.Errorf("dial failed after %d retries: %w", r.MaxRetries, lastErr)
}
```

`isPermanentDialError` — short list, conservative (anything unknown → retry):
- DNS NXDOMAIN
- Address parse errors
- `tsnet` auth failures (if distinguishable)

### Tier 2 — Control-plane supervisor (only if Q1 = "tsnet doesn't self-heal")

Implement in `main.go` (orchestration layer), NOT inside the proxy package. Sketch:

```go
// In run(), wrap initTailscale with a supervisor goroutine that watches a
// "tunnel healthy" signal (e.g. consecutive Dial failures within window).
// On unhealthy → server.Close(), re-call initTailscale, swap the Inner pointer
// of ReconnectDialer atomically.
```

Defer Tier 2 to a separate PR if Q1 confirms tsnet self-heals — keep this PR small.

---

## Configuration (env vars to add)

| Var | Default | Purpose |
|-----|---------|---------|
| `TS_DIAL_RETRIES` | `3` | Max retry attempts for target dial. `0` disables. |
| `TS_DIAL_BACKOFF_BASE` | `1s` | Base backoff (multiplied by `2^attempt`). |
| `TS_DIAL_BACKOFF_MAX` | `30s` | Cap on backoff per attempt. |

Add to `internal/config/config.go` `Config` struct + `Load()` + `.env.example`. Follow the existing `envOr` / `parseDuration` patterns.

---

## File touch list (concrete)

| File | Change |
|------|--------|
| `internal/proxy/reconnect.go` | **new** — `ReconnectDialer`, `isPermanentDialError`, `exponentialBackoff` |
| `internal/proxy/reconnect_test.go` | **new** — TDD-first, see test plan below |
| `internal/config/config.go` | add `DialRetries`, `DialBackoffBase`, `DialBackoffMax` fields + parse |
| `internal/config/config_test.go` | extend table-driven tests for new vars |
| `main.go` | wrap `server` (line 178) with `&proxy.ReconnectDialer{Inner: server, ...}` before passing to `AcceptLoop` |
| `.env.example` | document the 3 new vars (commented optional, not required) |
| `README.md` | add the 3 vars to the env-var reference table |

Current `main.go:178` for reference: `errAccept := proxy.AcceptLoop(listener, server, cfg, &activeConns, logger)` — replace `server` with the wrapped dialer.

---

## Test plan (TDD — write these first, then implement)

In `internal/proxy/reconnect_test.go`, table-driven via `t.Run`:

1. **success on first attempt** — mock returns conn immediately, no retry, no sleep.
2. **success after N transient failures** — mock fails twice with `io.EOF`, succeeds on 3rd. Assert attempt count + total elapsed within expected backoff window (use injectable clock or generous tolerance).
3. **gives up after MaxRetries** — mock always fails. Assert returned error wraps the last failure and includes the attempt count.
4. **permanent error short-circuits** — mock returns `&net.AddrError{}` or DNS error. Assert no retry, immediate return.
5. **context cancellation aborts retry loop** — start with cancellable ctx, cancel during backoff sleep. Assert `ctx.Err()` returned promptly (not after full backoff).
6. **jitter is bounded** — run dial 100x with deterministic seed, assert all sleeps fall in `[base, base + base/2]`.
7. **MaxRetries=0 means no retry** — mock fails once, assert single attempt, no backoff sleep.

For Tier 2 supervisor (if needed): separate test file, exercise via fake `Dialer` whose error rate flips based on a counter.

### Quality gates (must pass before PR)

- `go test -race ./...` — green, no new races
- `go vet ./...` — clean
- `golangci-lint run` — clean (gocyclo < 10 per function)
- `gosec ./...` — no new findings (watch for `crypto/rand` vs `math/rand` complaints — for jitter, `math/rand` is fine; document if gosec flags it)
- All existing tests still pass (no behavior change for the happy path with retries=0)

---

## Risks & edge cases

- **Goroutine leaks during retry**: ensure `ctx.Done()` in the `select` truly aborts the sleep and that no spawned goroutine is left waiting.
- **Metric pollution**: each retry attempt is NOT a new `TotalConnections` — only count the outer call. Make sure the retry happens *inside* `Dial()` and is invisible to `handleConn`'s metric increments.
- **Logging volume**: `Warn` per retry could be noisy. Consider `Debug` on first retry and `Warn` only when escalating, or rate-limit.
- **Backoff math overflow**: `1 << attempt` overflows at attempt=63. Cap `attempt` at a sane number (e.g. 30) before shifting.
- **`math/rand` global state**: don't use the deprecated global; use `rand.New(rand.NewSource(...))` per dialer or `rand/v2`.

---

## Acceptance criteria

- [ ] Q1/Q2/Q3 from the open-questions section answered, findings recorded
- [ ] `ReconnectDialer` implemented in `internal/proxy/reconnect.go`
- [ ] All 7 test cases in the test plan present and passing under `-race`
- [ ] 3 env vars wired through `config` + `.env.example` + README
- [ ] `main.go` wires the wrapper around `server` before `AcceptLoop`
- [ ] No regressions in existing test suite
- [ ] CI green (test, lint, security)
- [ ] PR ≤ 300 lines (excluding tests + docs)
- [ ] If Q1 = "tsnet self-heals" → ARCH-005 marked done in the same PR (collapse confirmed)
- [ ] If Q1 = "tsnet does NOT self-heal" → Tier 2 supervisor split to a follow-up PR (note in the PR description)

---

## How to pick this up cold (first 10 min of next session)

1. `cd ~/Projects/ts-bridge && git checkout master && git pull`
2. `git checkout -b feat/reconnect-dialer`
3. Read this file + [adr-006-dialer-interface-extraction](https://github.com/mlorentedev/ts-bridge/blob/master/docs/adr/adr-006-dialer-interface-extraction.md) (≤5 min total)
4. Open `internal/proxy/proxy.go` and find the existing `Dialer` interface (line 35) and `mockDialer` in `proxy_test.go` (line 18) — both will be reused
5. Answer Q1 by skimming `tailscale.com/tsnet` — `go doc tailscale.com/tsnet.Server` and grep for "reconnect" / "magicsock"
6. Then either: write tests first (TDD), or add a short Q1/Q2 finding note to this file before coding
