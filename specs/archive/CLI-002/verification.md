---
tags: [spec, verification, CLI-002]
created: "2026-06-10"
---

# Verification - CLI-002

## Evidence

### Connect subcommand

- [x] ts-bridge connect --target 100.64.0.1:3389 starts bridge without env vars
- [x] ts-bridge connect --auth-key-file /path/to/key reads key from file
- [x] ts-bridge connect with TS_TARGET+TS_AUTHKEY env vars works (backward compat)
- [x] All TS_* env vars mapped to flags
- [x] Flag --target overrides TS_TARGET env var
- [x] --reset is no-op in auto mode
- [x] --auth-key logs WARNING about process list

### YAML config

- [x] --config ts-bridge.yaml loads and applies settings
- [x] CLI flag overrides YAML value
- [x] Env var overrides YAML value
- [x] Missing YAML file is not an error
- [x] Unknown YAML fields produce warning
- [x] Auth key in YAML rejected with clear error
- [x] version: 1 accepted; missing version uses defaults
- [x] World-readable YAML perms produce warning (Unix)

### General

- [x] Config merge logic in internal/config/, not cmd/
- [x] go test ./... PASS
- [x] golangci-lint run clean
- [x] Binary size increase < 2MB (negligible)
- [x] PR diff < 400 lines (excluding tests, go.sum)

## Archive checklist

- [x] proposal.md frontmatter set to status: archived
- [x] Folder moved: specs/CLI-002/ -> specs/archive/CLI-002/
- [x] Issue #49 closed with PR link (#62)
- [x] Issue #52 (CLI-005) closed as duplicate
- [x] PR #62 merged (650d252)
