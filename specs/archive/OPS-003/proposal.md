---
id: "OPS-003"
type: spec
status: proposed
created: "2026-06-10"
tags: [spec, ops, ci, quality]
issue: 56
---

# OPS-003: Add go mod tidy check to CI pipeline

## Why

CI does not verify that go.mod/go.sum are clean. A PR that adds a dependency but forgets go mod tidy passes CI but leaves dirty go.sum.

## What

Add a CI step: run go mod tidy, then git diff --exit-code go.mod go.sum. Fails the build if dirty.

## Acceptance criteria

- [ ] CI step added to .github/workflows/ci.yml
- [ ] Existing go.mod/go.sum pass the check
- [ ] PR < 10 lines diff

## References

- Issue #56
- .github/workflows/ci.yml