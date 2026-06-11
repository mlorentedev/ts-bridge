---
tags: [spec, tasks, CLI-001]
created: "2026-06-10"
---

# Tasks - CLI-001

> TDD order. One task = one focused commit.

## Setup

- [x] Branch from master: feat/cli-scaffold
- [x] Proposal approved (issue #48)
- [x] ADR-008 approved (issue #38)

## Implementation

### Dependency

- [x] go get github.com/spf13/cobra
- [x] go mod tidy
- [x] Verify no unexpected transitive deps added

### Root command

- [x] Create cmd/root.go with root command
- [x] Add global flags: --verbose, --config
- [x] Wire persistent pre-run for logger init
- [x] Write failing test: root help contains expected subcommands

### Version subcommand

- [x] Create cmd/version.go with version command
- [x] Add --short flag
- [x] Wire ldflags (version, commit, date) to Cobra
- [x] Write tests: version output format, --short flag

### main.go wiring

- [x] Update main.go to call cmd.Execute()
- [x] Keep -version flag as deprecated alias
- [x] Remove old flag.Parse() logic (moved to Cobra)
- [x] Verify go build works

### Backward compat

- [x] ts-bridge -version still works (prints version and exits)
- [x] ts-bridge -v still enables verbose (global flag)
- [x] Existing env-var-only workflow unchanged

### Testing

- [x] go test ./... green
- [x] golangci-lint run clean
- [x] Manual: go build && ./ts-bridge --help looks correct

## Closing

- [x] All acceptance criteria met
- [x] PR < 200 lines diff (excluding go.sum)
- [x] PR references issue #48
- [ ] verification.md filled
