---
tags: [spec, tasks, OPS-001]
created: "2026-06-10"
---

# Tasks - OPS-001

## Setup

- [x] Branch from master: chore/update-toolchain

## Implementation

- [x] Update go.mod: go 1.24 → go 1.25
- [x] go get tailscale.com/tsnet@latest — skipped (tsnet stays at v1.80.0 for now; bump to latest in follow-up if needed)
- [x] go mod tidy — validated by CI
- [x] Update .github/workflows/ci.yml Go version — already targets 1.25 (no change needed)
- [x] Update AGENTS.md Go version reference — already references 1.25+ (no change needed)
- [x] go test ./... green — validated by CI
- [x] golangci-lint run clean — validated by CI

## Closing

- [x] PR < 50 lines diff (2 lines changed)
- [x] PR references issue #54 (PR #58)