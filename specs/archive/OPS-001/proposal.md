---
id: "OPS-001"
type: spec
status: proposed
created: "2026-06-10"
tags: [spec, ops, toolchain, update]
issue: 54
---

# OPS-001: Update Go toolchain 1.24 to 1.25 and tsnet to latest

## Why

Go 1.25 is the current stable release (since March 2026). tsnet v1.80.0 is from February 2026. Staying current reduces security risk and CI drift.

## What

- Update go.mod: go 1.24 -> go 1.25
- Run `go get tailscale.com/tsnet@latest` to bump tsnet
- Run `go mod tidy` to clean up go.sum
- Run `go test ./...` to verify no regressions
- Run `golangci-lint run` to verify no new lint issues
- Update CI Go version in .github/workflows/ci.yml
- Update AGENTS.md to reflect actual Go version

### Pre-flight check

Before running `go get`, check the tsnet changelog between v1.80.0 and latest for any breaking changes. tsnet API has historically been stable (see lessons.md), but verify:
- No changes to tsnet.Server struct fields
- No changes to Up(), Dial(), Close() signatures
- No changes to Ephemeral behavior

If breaking changes are found, document them and adjust the implementation plan.

## Dependencies

- ADR-009 (approved)

## Acceptance criteria

- [ ] go.mod updated to go 1.25
- [ ] tsnet at latest version (verified no breaking changes)
- [ ] go mod tidy produces clean go.sum
- [ ] go test ./... green
- [ ] golangci-lint run clean
- [ ] CI uses Go 1.25
- [ ] AGENTS.md corrected
- [ ] PR < 50 lines diff (mostly go.sum)

## References

- ADR-009
- Issue #54
- lessons.md 2026-02-27 (tsnet API stability)