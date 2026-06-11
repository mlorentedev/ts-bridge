---
id: "CLI-001"
type: spec
status: proposed
created: "2026-06-10"
tags: [spec, cli, cobra, scaffold]
issue: 48
---

# CLI-001: Scaffold Cobra CLI structure with version subcommand

## Why

ts-bridge needs a professional CLI to replace the current shell-script-based launcher pattern. This is the foundation: add Cobra, scaffold the command tree, and implement the simplest subcommand (`version`) to validate the pattern.

## What

- Add `github.com/spf13/cobra` dependency
- Create `cmd/` package with root command + version subcommand
- Update `main.go` to be a thin entry point calling `cmd.Execute()`
- Keep backward compat: `-version` flag still works (deprecation path)
- Inject version/commit via ldflags (already done, just wire to Cobra)

## Out of scope

- Other subcommands (connect, init, host, status) — separate specs
- YAML config parsing — CLI-005
- Removing old scripts — OPS-002

## Acceptance criteria

- [x] `go get github.com/spf13/cobra` — dependency added
- [x] `ts-bridge --help` shows available subcommands
- [x] `ts-bridge version` prints version + commit + Go version
- [x] `ts-bridge version --short` prints just semver
- [x] `-v` / `--verbose` global flag works
- [x] `--config` flag accepted (parsing deferred to CLI-005)
- [x] Existing `-version` flag still works (deprecation path)
- [x] `go test ./...` green
- [x] `golangci-lint run` clean
- [x] PR < 200 lines diff (excluding go.sum)

## References

- ADR-008 (CLI Architecture)
- Issue #48
- https://github.com/spf13/cobra
