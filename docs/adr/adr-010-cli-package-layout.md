---
id: "adr-010"
type: adr
status: accepted
date: "2026-06-16"
tags: [architecture, packaging, cli]
owner: manu
---

# ADR-010: CLI Package Layout — `cmd/ts-bridge/main.go`

## Context

ts-bridge adopted Cobra for CLI in ADR-008, placing all CLI code under `cmd/`.
However, `main.go` remained at the repository root with `package main`,
importing `cmd` as an external dependency (`ts-bridge/cmd`).

This created an awkward layout:

```text
ts-bridge/
├── main.go              ← package main, imports cmd/*
├── cmd/                 ← package cmd (Cobra CLI)
│   ├── root.go
│   ├── connect.go
│   ├── init.go
│   └── ...
└── internal/            ← package internal/*
```

`main.go` became a thin glue layer importing `cmd` — not idiomatic Go.
The standard Go convention (used by Cobra, kubectl, Docker, GitHub CLI)
places the `main()` function inside `cmd/<binary-name>/`.

## Decision

Move the process entry point to `cmd/ts-bridge/main.go` and keep the Cobra command
tree in the importable `cmd/cli` package. The entry point owns process-level
concerns (legacy argument normalization, build variables, dependency wiring, and
exit status); `cmd/cli` owns commands and bridge orchestration.

As amended by issue #278 on 2026-08-10, `cmd/cli` exposes:

```go
func NewRootCmd() *cobra.Command // constructs a complete, fresh command tree
func Execute() error            // executes NewRootCmd()
```

Every command is built by a `newXCmd()` constructor. Package `init()` functions
and package-level command singletons are prohibited because they make Cobra flag
state persist between invocations and force tests to replicate production wiring.

```text
ts-bridge/
├── cmd/
│   ├── cli/             ← package cmd (Cobra CLI code)
│   │   ├── root.go      ← NewRootCmd(), Execute()
│   │   ├── run.go       ← Runner, LoggerInit, bridge main loop
│   │   ├── connect.go
│   │   ├── init.go
│   │   ├── host.go
│   │   ├── status.go
│   │   ├── version.go
│   │   ├── confirm_overwrite.go
│   │   └── *_test.go    ← package cmd + package cmd_test (same-dir)
│   └── ts-bridge/       ← package main (entry point)
│       └── main.go
├── internal/            ← package internal/* (business logic)
│   ├── config/
│   ├── proxy/
│   ├── health/
│   ├── telemetry/
│   └── host/
└── go.mod
```

Test placement follows Go conventions:
- `package cmd` tests in `*_test.go` alongside source (access to internals)
- `package cmd_test` external tests in the **same directory** (not a sibling dir)
- `internal/*` tests in `*_test.go` alongside source

`main.go` at the repository root is removed. The `main()` function lives in
`cmd/ts-bridge/main.go` and imports `cmd/cli`; business logic remains under
`internal/*`.

### Why not keep `main.go` at root?

1. **Idiomatic Go.** `go list ./...` shows `ts-bridge/cmd/ts-bridge` as the
   main package, matching `go build ./cmd/ts-bridge/`.
2. **Explicit binary boundary.** `cmd/ts-bridge` contains only process wiring;
   the importable `cmd/cli` package lets tests construct the production tree
   without executing `main()` or reproducing command definitions.
3. **Future-proof.** If a second binary is ever needed (e.g. `ts-bridge-agent`),
   `cmd/` naturally extends to `cmd/ts-bridge/`, `cmd/ts-bridge-agent/`.
4. **Standard convention.** Cobra documentation, `go run`, and IDE tooling
   expect `cmd/<name>/main.go`.

### Why not merge into `internal/`?

ADR-007 already split `main.go` into `internal/` packages. Merging CLI into
`internal/` would violate the principle that `internal/` contains application
logic, not entry points. The `cmd/` namespace signals "this is the binary
boundary."

## Consequences

### Positive

- **Standard Go layout.** Matches conventions used by the entire Go ecosystem.
- **Cleaner import graph.** `cmd/ts-bridge` imports `cmd/cli`; `cmd/cli` imports
  the required `internal/*` packages.
- **Testable.** External tests can call `cmd/cli.NewRootCmd()` and exercise the
  complete production tree. Each call receives independent Cobra flag state.
- **Extensible.** Adding a second binary (agent, daemon, etc.) fits naturally.

### Negative

- **Import path change.** `ts-bridge/cmd` → `ts-bridge/cmd/cli`.
  Only affects `main.go` (one import). All other packages unchanged.
- **`go build .` no longer works.** Must use `go build ./cmd/ts-bridge/` or
  `go build .` from within `cmd/ts-bridge/`.
- **CI build commands need update.** Any script or CI job that does `go build .`
  must change to `go build ./cmd/ts-bridge/`.

### Neutral

- Files under `cmd/cli/` use `package cmd`; the entry point under
  `cmd/ts-bridge/` uses `package main`.

## Migration

1. Move Cobra files from `cmd/` to `cmd/cli/`
2. Move the root entry point to `cmd/ts-bridge/main.go` and import
   `"ts-bridge/cmd/cli"`
3. Remove root `main.go`
4. Replace command singletons and registration side effects with constructors
5. Verify: `go build ./...` and `go test ./...`

## Related

- [adr-007-multi-package-split.md](adr-007-multi-package-split.md) — internal package split
- [adr-008-cli-architecture.md](adr-008-cli-architecture.md) — Cobra adoption
- [adr-013-cli-tests-in-go.md](adr-013-cli-tests-in-go.md) — production CLI tests in Go
- Issue #278 — command-tree constructor amendment
