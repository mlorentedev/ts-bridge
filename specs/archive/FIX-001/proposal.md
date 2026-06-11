---
id: "FIX-001"
type: spec
status: proposed
created: "2026-06-10"
tags: [spec, fix, testing, data-race]
issue: 40
---

# FIX-001: Fix legacy integration tests with stale proxy pattern

## Why

Three integration tests (TestProxyBidirectionalFlow, TestConnectionClosePropagation, TestConcurrentConnections) use a manual proxy pattern with sync.Once that has a known data race (see lessons.md 2026-03-07). They also do not test the actual proxyConnections() function used in production.

## What

Rewrite the three tests to use the real proxyConnections() from internal/proxy/ instead of the manual sync.Once pattern. Extract shared test helpers to eliminate duplication (TECH-001).

### Test changes

1. **Extract helper**: `startProxyPair(t, remoteAddr) (localAddr string, cleanup func())` that sets up a real listener + AcceptLoop + proxyConnections
2. **TestProxyBidirectionalFlow**: use helper + real proxyConnections, verify bidirectional data flow
3. **TestConnectionClosePropagation**: use helper + real proxyConnections, verify close propagation
4. **TestConcurrentConnections**: use helper + real AcceptLoop, reduce from 50 to 10 concurrent connections to avoid flaky CI. Use t.Parallel() + timeout.

### Performance note

The original test used 50 concurrent connections. With real AcceptLoop + proxyConnections, this may be slower. Reduce to 10 connections with a 5s timeout. If the test is still flaky, document the trade-off.

## Dependencies

- No code dependencies (test-only change)

## Acceptance criteria

- [ ] TestProxyBidirectionalFlow uses real proxyConnections()
- [ ] TestConnectionClosePropagation uses real proxyConnections()
- [ ] TestConcurrentConnections uses real AcceptLoop + proxyConnections() (10 conns, not 50)
- [ ] Shared helper extracted (eliminates duplication from TECH-001)
- [ ] go test -race ./... PASS
- [ ] No loss of test coverage vs current tests
- [ ] golangci-lint run clean
- [ ] PR < 200 lines diff

## References

- Issue #40, #43
- lessons.md 2026-03-07 (data race documentation)
- internal/proxy/proxy_test.go (correct test patterns)