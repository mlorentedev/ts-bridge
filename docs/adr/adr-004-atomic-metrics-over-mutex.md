---
id: "ADR-004-atomic-metrics-over-mutex"
type: adr
status: accepted
date: "2024-06-01"
tags: [architecture, decision, performance, concurrency]
owner: manu
created: "2026-03-28"
---

# ADR-004: Use sync/atomic for Metrics Instead of sync.Mutex

## Context
ts-bridge tracks 6 operational metrics (active connections, total connections, bytes tx/rx, errors, rejected connections) that are updated from concurrent goroutines (one per connection) and read by the `/metrics` HTTP handler.

## Options Considered
1. **sync.Mutex**
    * *Pros:* Simple, protects complex state, familiar pattern.
    * *Cons:* Lock contention under high connection churn, potential for deadlocks if misused, blocks readers during writes.
2. **sync/atomic**
    * *Pros:* Lock-free, zero contention, optimal for simple counters, no deadlock risk.
    * *Cons:* Only works for individual integer operations, not suitable for compound operations.
3. **sync.RWMutex**
    * *Pros:* Allows concurrent reads, only blocks on writes.
    * *Cons:* Still has overhead vs atomic, unnecessary complexity for simple counters.

## Decision
We chose **sync/atomic** because all 6 metrics are independent `int64` counters with only `Add` and `Load` operations. No compound read-modify-write is needed. This gives zero-contention performance even with thousands of concurrent connections.

## Consequences
- **Positive:** Zero lock contention, optimal performance for counter-only metrics, simple code (`atomic.AddInt64`, `atomic.LoadInt64`).
- **Negative:** If future metrics require compound operations (e.g., histogram buckets), a mutex or dedicated metrics library (Prometheus client) will be needed. This is already planned as backlog item OBS-001.

## References
- https://pkg.go.dev/sync/atomic
- `main.go:73-81` — `Metrics` struct definition
- `main.go:297-307` — Atomic snapshot in `/metrics` handler
