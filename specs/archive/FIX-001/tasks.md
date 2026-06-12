---
tags: [spec, tasks, FIX-001]
created: "2026-06-10"
---

# Tasks - FIX-001

## Setup

- [ ] Branch from master: fix/legacy-integration-tests

## Implementation

- [ ] Extract shared test helper startProxyPair(t, remoteAddr) (localAddr, cleanup)
- [ ] Rewrite TestProxyBidirectionalFlow to use real proxyConnections()
- [ ] Rewrite TestConnectionClosePropagation to use real proxyConnections()
- [ ] Rewrite TestConcurrentConnections to use real AcceptLoop + proxyConnections()
- [ ] Verify race detector passes: go test -race ./...

## Testing

- [ ] go test -race ./... green
- [ ] golangci-lint run clean

## Closing

- [ ] PR < 200 lines diff
- [ ] PR references issues #40, #43