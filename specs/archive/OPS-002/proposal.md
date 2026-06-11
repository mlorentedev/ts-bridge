---
id: "OPS-002"
type: spec
status: proposed
created: "2026-06-10"
tags: [spec, ops, cleanup, scripts]
issue: 55
---

# OPS-002: Remove obsolete client scripts after CLI migration

## Why

After all CLI subcommands are implemented, the old shell scripts become dead code. Remove them to reduce maintenance surface.

## What

Remove scripts/client/run.sh, run.ps1, bootstrap.sh, bootstrap.ps1. Keep scripts/host/setup.ps1 and scripts/dev.sh. Update README, CONTRIBUTING.md, AGENTS.md.

## Acceptance criteria

- [ ] All 4 client scripts removed
- [ ] README updated to show CLI usage
- [ ] No remaining references to removed scripts in docs
- [ ] BATS tests for removed scripts archived or removed
- [ ] go test ./... green
- [ ] PR < 100 lines diff

## References

- ADR-008
- Issue #55