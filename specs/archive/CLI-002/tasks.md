---
tags: [spec, tasks, CLI-002]
created: "2026-06-10"
---

# Tasks - CLI-002

> TDD order. This spec subsumes CLI-005 (YAML config).

## Setup

- [ ] Branch from master: feat/cli-connect
- [ ] Depends on CLI-001 (cmd/ package exists)
- [ ] CLI-005 marked as duplicate (subsumed here)

## Implementation

### Config merge logic (internal/config/)

- [ ] Write failing test: flag overrides env var
- [ ] Write failing test: env var overrides YAML value
- [ ] Write failing test: YAML value overrides default
- [ ] Write failing test: full precedence chain (flag > env > yaml > default)
- [ ] Implement Config.Merge(flags, env, yaml) in internal/config/merge.go

### YAML config loader (internal/config/yaml.go)

- [ ] go get gopkg.in/yaml.v3
- [ ] Define YAML struct with version: 1 field
- [ ] Implement LoadYAMLConfig(path string) (partial Config, error)
- [ ] Reject auth key in YAML with clear error
- [ ] Warn on unknown YAML fields
- [ ] Warn on world-readable permissions (>0600 on Unix)
- [ ] Validate all YAML values (types, ranges)

### Connect command (cmd/connect.go)

- [ ] Create cmd/connect.go
- [ ] Register all flags (target, auth-key, auth-key-file, instance, local-addr, hostname, state-dir, control-url, timeouts, retries, health-addr, log-format, manual-mode, port-range, reset, config)
- [ ] Wire --auth-key-file: read key from file, validate perms
- [ ] Wire --auth-key: log WARNING about process list visibility
- [ ] Wire config merge: flags > env > yaml > defaults
- [ ] Wire to existing run(cfg) function
- [ ] Handle --reset flag (no-op in auto mode)

### Backward compat

- [ ] All TS_* env vars still work when flags omitted
- [ ] Auto-instance mode is default (same as current)
- [ ] --manual-mode restores legacy behaviour

### Binary size check

- [ ] Build before: `go build -o /dev/null .` record size
- [ ] Build after: verify increase < 2MB

### Testing

- [ ] go test ./... green
- [ ] golangci-lint run clean
- [ ] Manual: ts-bridge connect --help shows all flags
- [ ] Manual: ts-bridge connect --config missing.yaml (not an error)
- [ ] Manual: ts-bridge connect --config bad.yaml (clear error)

## Closing

- [ ] All acceptance criteria met
- [ ] PR < 400 lines diff (excluding tests, go.sum)
- [ ] PR references issue #49
- [ ] PR description notes CLI-005 subsumed
- [ ] verification.md filled