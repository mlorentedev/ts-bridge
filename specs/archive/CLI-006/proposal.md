---
id: "CLI-006"
type: spec
status: archived
created: "2026-06-10"
tags: [spec, cli, status, health]
issue: 53
merged: bb6553e
pr: 64
---

# CLI-006: Implement ts-bridge status subcommand

## Why

Users need a quick way to check bridge health without curling the /metrics endpoint. ts-bridge status provides a human-readable summary of the running bridge.

## What

Implement ts-bridge status that queries the bridge health endpoint and prints a summary.

### CLI Flags

```
--addr ADDR      Health server address (default: from config/env)
--watch          Continuously watch and update status (like top)
--interval D     Polling interval for --watch (default: 5s)
--json           Output raw JSON from /metrics
```

### Behaviour

- Connects to the bridge health endpoint (/health/live, /health/ready, /metrics)
- Prints a human-readable summary:
  - Bridge status (running / not running)
  - Active connections / max
  - Total connections served
  - Bytes transferred (tx/rx)
  - Errors
  - Uptime (if available)
- --json outputs the raw metrics JSON
- --watch refreshes every N seconds
- **Signal handling**: SIGINT/SIGTERM in --watch mode restores terminal state and exits gracefully
- **Graceful error**: if bridge is not running, print "Bridge not running at ADDR" (exit code 0, not an error)

## Out of scope

- Starting/stopping the bridge (that is connect job)
- Remote status queries (only local health endpoint)

## Dependencies

- Depends on CLI-001 (Cobra scaffold)

## Acceptance criteria

- [ ] ts-bridge status prints human-readable summary
- [ ] ts-bridge status --json prints raw JSON from /metrics
- [ ] ts-bridge status --watch refreshes periodically
- [ ] Ctrl+C in --watch mode exits gracefully (no terminal corruption)
- [ ] Graceful message if bridge is not running (not an error)
- [ ] go test ./... green
- [ ] golangci-lint run clean
- [ ] PR < 150 lines diff (excluding tests)

## References

- ADR-008
- Issue #53
- internal/health/health.go
- internal/telemetry/metrics.go