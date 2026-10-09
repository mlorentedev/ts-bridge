---
id: "DEPS-312-go-127-tailscale"
type: spec
status: implementing
created: "2026-10-09"
issue: "mlorentedev/ts-bridge#435"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# DEPS-312: Go 1.27 and Tailscale 1.104

## Why

<!-- from issue #435: DEPS-312: Upgrade Go toolchain for Tailscale 1.104 -->

Tailscale v1.104.0 requires Go 1.27.1, but every Go workflow in this repository uses Go 1.26 and its pinned linter was built with Go 1.26. Dependabot PR #431 therefore fails before its tests can run. Upgrade the toolchain and dependency together so CI can actually validate the new version while retaining the fail-closed toolchain policy.

## What

The project builds and tests against Tailscale v1.104.0 with Go 1.27.1 or newer; all CI, release, mutation and hygiene jobs use the Go version required by the module, and lint runs under a compatible pinned version. CI rejects a future toolchain mismatch rather than silently downloading another compiler.

## Out of scope

- No application behavior, protocol, or CLI changes.
- No merge of the separate release PR #409 or Dependabot PR #431.

## Risks / open questions

- A newer linter may expose additional findings; fix only findings caused by this upgrade and track unrelated defects separately.
- A Go test timed out once under the baseline full-suite load on Windows but passed three isolated reruns before any changes. Record any recurrent failure rather than attributing it to the new dependency.

## Acceptance criteria

- [x] `go.mod` requires Go 1.27.1 and Tailscale v1.104.0, and `go mod tidy` produces no diff.
- [x] Every GitHub workflow that installs Go derives its version from `go.mod`; a fixture test rejects a pinned 1.26 workflow and accepts the configured workflows.
- [ ] CI lint uses a pinned version compatible with Go 1.27.1; build, vet, tests and lint pass on the upgraded dependency, with Windows and release jobs covered by CI.
- [x] Operator documentation gives the correct Go minimum and local lint version.

## References

- Bitácora board: the GitHub issue / Project item tracking this spec (see the `issue:` frontmatter field)
- Related PR: #431 (blocked by its Go floor; non-editable Dependabot branch).
