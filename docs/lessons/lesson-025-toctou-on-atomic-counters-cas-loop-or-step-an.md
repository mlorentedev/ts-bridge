---
id: lesson-025-toctou-on-atomic-counters-cas-loop-or-step-an
type: lesson
status: active
created: "2026-05-28"
owner: manu
tags: [ts-bridge, lesson, go, atomics, concurrency, toctou, race-conditions]
---

# TOCTOU on atomic counters: CAS loop or step-and-rollback

**Context:** `AcceptLoop` previously did `GetActiveConnections() >= max` check, then `AddActiveConnection(1)` inside `handleConn` — two separate atomic ops. Under accept-storm load, multiple goroutines could observe `cur < max` simultaneously and all increment past the cap.

**Finding:** Two correct patterns to fix this in Go's `sync/atomic`:
1. **CAS loop** (chosen): `for { cur := Load; if cur >= max { return false }; if CompareAndSwap(cur, cur+1) { return true } }`. Spins until either succeeds or limit hits. Bounded retries since the value can only grow.
2. **Step-and-rollback**: `n := Add(1); if n > max { Add(-1); return false }; return true`. Always increments first; rolls back on overflow. Simpler but the brief over-increment can be visible to other readers (`GetActiveConnections` returns `max+1` for a moment).

CAS loop preserves the invariant `ActiveConnections <= max` always. For a metric people inspect via HTTP (`/metrics`), this matters — a step-and-rollback can show `cap+1` to scrapers mid-rollback.

**Rule:** When the counter is observable (exposed metric, used for back-pressure decisions, etc.), prefer CAS loop. When it's purely internal (just want eventual correctness), step-and-rollback is fine and slightly cheaper. Either way, the original `check; then; act` pattern is wrong under any concurrency.

**Tags:** `#go` `#atomics` `#concurrency` `#toctou` `#race-conditions`
