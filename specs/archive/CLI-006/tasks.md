---
tags: [spec, tasks, CLI-006]
created: "2026-06-10"
---

# Tasks - CLI-006

## Setup

- [x] Branch from master: feat/cli-status
- [x] Depends on CLI-001 (cmd/ package exists)

## Implementation

- [x] Create cmd/status.go
- [x] Implement health endpoint client (HTTP GET to /health/live, /health/ready, /metrics)
- [x] Print human-readable summary
- [x] Add --json flag for raw metrics output
- [x] Add --watch flag with --interval for continuous refresh
- [x] Graceful error handling when bridge not running

## Testing

- [x] go test ./... green
- [x] golangci-lint run clean

## Closing

- [x] PR #64 merged (diff 222 lines production code)
- [x] PR references issue #53