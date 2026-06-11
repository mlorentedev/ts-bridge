---
tags: [spec, tasks, CLI-003]
created: "2026-06-10"
---

# Tasks - CLI-003

## Setup

- [x] Branch from master: feat/cli-init
- [x] Depends on CLI-001 (cmd/ package exists)

## Implementation

- [x] Create cmd/init.go
- [x] Implement interactive mode: prompt for auth key (masked), target, instance, format
- [x] Implement non-interactive mode: --auth-key + --target flags
- [x] Write YAML output with comments
- [x] Write .env output for legacy compat
- [x] Validate all inputs before writing
- [x] Extract collectInteractiveInputs() to reduce cyclomatic complexity
- [x] Add golang.org/x/term (already indirect dep via tailscale.com)

## Testing

- [x] go test ./... green
- [x] golangci-lint run clean

## Closing

- [ ] PR < 250 lines diff (excluding tests)
- [ ] PR references issue #50