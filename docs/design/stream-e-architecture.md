---
id: "plan-stream-e"
type: implementation-plan
status: active
created: "2026-03-07"
tags: [architecture, testability, refactor, stream-e]
origin: "Comparative analysis with github.com/MuhammadHananAsghar/wormhole"
owner: manu
---

# Stream E: Architecture & Testability — Implementation Plan

> **Goal:** Improve testability of core proxy logic and prepare for reconnection resilience. Maintain single-binary, zero-config design (ADR-002).

## Phase 1: Dialer Interface (ARCH-001) — Foundation

**Branch:** `refactor/dialer-interface`
**Estimated scope:** ~30 lines changed in `main.go`, ~0 new files

### Tasks

1. **Define `Dialer` interface** in `main.go` (above `handleConn`)
   ```go
   type Dialer interface {
       Dial(ctx context.Context, network, addr string) (net.Conn, error)
   }
   ```
   `tsnet.Server` already satisfies this — no adapter needed.

2. **Update `acceptLoop` signature**
   - Before: `func acceptLoop(listener net.Listener, server *tsnet.Server, cfg Config) error`
   - After: `func acceptLoop(listener net.Listener, dialer Dialer, cfg Config) error`

3. **Update `handleConn` signature**
   - Before: `func handleConn(client net.Conn, server *tsnet.Server, cfg Config)`
   - After: `func handleConn(client net.Conn, dialer Dialer, cfg Config)`

4. **Update call in `handleConn`**
   - Before: `remote, err := server.Dial(ctx, "tcp", cfg.Target)`
   - After: `remote, err := dialer.Dial(ctx, "tcp", cfg.Target)` (no change needed — same method)

5. **Update call site in `run`**
   - Before: `return acceptLoop(listener, server, cfg)`
   - After: `return acceptLoop(listener, server, cfg)` (no change — `*tsnet.Server` satisfies `Dialer`)

6. **Verify:** `go build`, `go vet`, `go test -race ./...` — all must pass unchanged.

### Acceptance criteria
- [ ] `Dialer` interface defined
- [ ] `acceptLoop` and `handleConn` accept `Dialer` instead of `*tsnet.Server`
- [ ] All existing tests pass (no behavior change)
- [ ] `go vet` clean

---

## Phase 2: Core Proxy Tests (ARCH-003) — High-Value Testing

**Branch:** `test/proxy-core`
**Depends on:** ARCH-001
**Estimated scope:** ~150-200 lines in `main_test.go`

### Tasks

1. **Create `mockDialer` test helper**
   ```go
   type mockDialer struct {
       dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)
   }
   func (m *mockDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
       return m.dialFunc(ctx, network, addr)
   }
   ```

2. **Test `handleConn` — successful proxy** (table-driven)
   - Mock dialer returns `net.Pipe()` server end
   - Client writes data, assert it arrives at remote end
   - Remote writes data, assert it arrives at client end
   - Verify metrics: `ActiveConnections`, `TotalConnections`, `TotalBytesTx`, `TotalBytesRx`

3. **Test `handleConn` — dial failure**
   - Mock dialer returns error
   - Assert client connection is closed
   - Verify `TotalErrors` incremented

4. **Test `handleConn` — keepalive set on TCP connections**
   - Pass a real `*net.TCPConn` (from `net.Listen` + `net.Dial`)
   - Verify no error on keepalive setup

5. **Test `proxyConnections` — bidirectional transfer**
   - Use `net.Pipe()` for both client and remote
   - Write known data in both directions
   - Assert byte counts match

6. **Test `proxyConnections` — one side closes early**
   - Close remote, assert client also closes
   - Close client, assert remote also closes

7. **Test `acceptLoop` — connection limit enforcement** (verify existing integration test covers this, add unit test if not)

### Acceptance criteria
- [ ] `handleConn` has ≥4 test cases
- [ ] `proxyConnections` has ≥3 test cases
- [ ] All tests use mock `Dialer` — no tsnet dependency
- [ ] Coverage of `handleConn` + `proxyConnections` > 80%
- [ ] `go test -race` clean

---

## Phase 3: Multi-Package Split (ARCH-002) — Structural

**Branch:** `refactor/multi-package`
**Independent of** ARCH-001/003 (can be done in parallel or after)
**Estimated scope:** ~785 lines redistributed, 0 new logic

### Target structure

```
main.go                    (~80 lines — thin orchestrator)
internal/
  config/
    config.go              (~200 lines — Config, loadConfig, parse*, auto-instance)
    config_test.go         (existing config tests, moved)
  proxy/
    proxy.go               (~120 lines — Dialer, handleConn, proxyConnections, acceptLoop)
    proxy_test.go          (new tests from ARCH-003 + existing integration tests)
    errors.go              (~35 lines — isExpectedCloseError, isRetryableCleanupError)
  health/
    health.go              (~60 lines — startHealthServer, Metrics)
    health_test.go         (existing health tests, moved)
```

### Tasks

1. **Create `internal/config/` package**
   - Move: `Config`, `loadConfig`, `parseTarget`, `parseAuthKey`, `envOr`, `applyAutoInstanceConfig`, `shouldEnableAutoInstance`, `parseBoolEnv`, `deriveAutoLocalAddr`, `deriveAutoHostname`, `sanitizeHostnameLabel`, `parsePortRange`, `selectAvailablePort`
   - Export: `Config`, `Load` (rename from `loadConfig`)
   - Move constants: `defaultTimeout`, `defaultMaxConnections`, `defaultLocalAddr`, `defaultHostname`, `defaultStateDir`, `defaultAutoPortRange`
   - Move corresponding tests from `main_test.go`

2. **Create `internal/proxy/` package**
   - Move: `Dialer` interface, `handleConn`, `proxyConnections`, `acceptLoop`, `bufferPool`, `isExpectedCloseError`
   - Export: `Dialer`, `AcceptLoop`, `HandleConn`
   - Move: `Metrics` struct here (or to `internal/health/`)
   - Move constants: `bufferSize`, `keepAliveInterval`, `backoffMin`, `backoffMax`
   - Decision: `Metrics` stays as package-level var within `proxy` (acceptable for single-binary)

3. **Create `internal/health/` package**
   - Move: `startHealthServer`
   - Export: `StartServer`
   - Takes `*atomic.Bool` (ready flag) and metrics pointer

4. **Keep in `main.go`**
   - `main()`, `run()`, `initLogger()`, `printBanner()`, `ensureStateDir()`, `cleanupEphemeralStateDir()`, `isRetryableCleanupError()`
   - Version/commit vars, signal handling

5. **Update imports** across all files

6. **Verify:** full CI pipeline (`go build`, `go vet`, `go test -race`, `golangci-lint`, `gosec`)

### Acceptance criteria
- [ ] No file > 250 lines
- [ ] `main.go` < 100 lines (orchestration only)
- [ ] All tests pass in new locations
- [ ] No circular imports
- [ ] CI green

---

## Phase 4: Reconnection Logic (ARCH-004) — Resilience

**Branch:** `feat/reconnect`
**Depends on:** ARCH-001
**Estimated scope:** ~80-100 new lines

### Design

Wrap the `Dialer` with reconnection behavior. When `Dial()` fails with a non-permanent error, retry with exponential backoff + jitter before returning the error to the caller.

```go
type reconnectDialer struct {
    inner      Dialer
    maxRetries int
    maxBackoff time.Duration
    logger     *slog.Logger
}

func (r *reconnectDialer) Dial(ctx context.Context, network, addr string) (net.Conn, error) {
    var lastErr error
    for attempt := 0; attempt <= r.maxRetries; attempt++ {
        conn, err := r.inner.Dial(ctx, network, addr)
        if err == nil {
            return conn, nil
        }
        lastErr = err
        if isPermanentDialError(err) {
            return nil, err
        }
        backoff := min(time.Duration(1<<attempt)*time.Second, r.maxBackoff)
        jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-time.After(backoff + jitter):
        }
        r.logger.Warn("dial retry", "attempt", attempt+1, "error", err, "backoff", backoff+jitter)
    }
    return nil, fmt.Errorf("dial failed after %d retries: %w", r.maxRetries, lastErr)
}
```

### Tasks

1. **Define `reconnectDialer`** struct wrapping `Dialer`
2. **Implement `isPermanentDialError`** — auth failures, invalid address → don't retry
3. **Wire into `run()`** — wrap `server` (tsnet) with `reconnectDialer` before passing to `acceptLoop`
4. **Add env var `TS_DIAL_RETRIES`** (default 3) to `Config`
5. **Test with mock dialer** — verify retry count, backoff timing, permanent error short-circuit, context cancellation

### Acceptance criteria
- [ ] Transient dial failures retry up to N times with exponential backoff + jitter
- [ ] Permanent errors (auth, address) fail immediately
- [ ] Context cancellation stops retries
- [ ] ≥5 test cases for retry behavior
- [ ] Existing tests unaffected

---

## Phase 5: Target Retry (ARCH-005) — Polish

**Branch:** `feat/target-retry`
**Depends on:** ARCH-004 (reuses reconnectDialer pattern)
**Estimated scope:** ~20 lines (ARCH-004's reconnectDialer already handles this)

If ARCH-004's `reconnectDialer` is in place, ARCH-005 is **already solved** — the retry logic wraps `Dial()` which is the target dial. No additional work needed unless we want different retry behavior for initial connection vs. per-request dials.

### Decision point
- If same retry policy works for both control plane reconnection and target dial: **ARCH-005 = done with ARCH-004**
- If different policies needed: add a second `reconnectDialer` with different params

---

## Execution Order

```
Phase 1 (ARCH-001) ──→ Phase 2 (ARCH-003) ──→ Phase 4 (ARCH-004)
                                                        │
                                                        ▼
                                                Phase 5 (ARCH-005)

Phase 3 (ARCH-002) ──→ (independent, can run in parallel with 1→2→4)
```

**Recommended sequence:** 1 → 2 → 3 → 4 → 5

Phase 1 is tiny (30 lines) and unblocks Phase 2 (the highest-value change for test coverage). Phase 3 is the largest but mechanical — pure file reorganization. Phase 4 adds new behavior and should come after the codebase is well-tested.

## Constraints

- Maintain ADR-002: single binary, no config files, env-var driven
- Maintain ADR-004: atomic metrics, no mutexes
- TDD: write failing tests first, then implement (per global CLAUDE.md)
- Each phase = one PR with CI green before merge
