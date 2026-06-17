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

Move all CLI files from `cmd/` to `cmd/ts-bridge/`, making `cmd/ts-bridge/main.go`
the entry point. The `cmd/` directory becomes a namespace for CLI binaries;
`cmd/ts-bridge/` contains the single binary.

```text
ts-bridge/
├── cmd/
│   ├── cli/             ← package cmd (Cobra CLI code)
│   │   ├── root.go      ← Execute(), initCmdWiring()
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
`cmd/ts-bridge/root.go` (or a dedicated `main.go` inside the package), directly
importing `internal/*` packages without the intermediate `cmd` layer.

### Why not keep `main.go` at root?

1. **Idiomatic Go.** `go list ./...` shows `ts-bridge/cmd/ts-bridge` as the
   main package, matching `go build ./cmd/ts-bridge/`.
2. **No intermediate layer.** `main.go` importing `cmd` adds a package boundary
   with zero value — `cmd` only exists to be called from `main()`.
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
- **Cleaner import graph.** `cmd/ts-bridge` imports `internal/*` directly — no
  intermediate `cmd` package boundary.
- **Testable.** All existing tests pass under `ts-bridge/cmd/ts-bridge`.
- **Extensible.** Adding a second binary (agent, daemon, etc.) fits naturally.

### Negative

- **Import path change.** `ts-bridge/cmd` → `ts-bridge/cmd/cli`.
  Only affects `main.go` (one import). All other packages unchanged.
- **`go build .` no longer works.** Must use `go build ./cmd/ts-bridge/` or
  `go build .` from within `cmd/ts-bridge/`.
- **CI build commands need update.** Any script or CI job that does `go build .`
  must change to `go build ./cmd/ts-bridge/`.

### Neutral

- Package names inside `cmd/ts-bridge/` remain `package cmd` (not `package main`)
  for all non-entry-point files. Only `root.go` (or a dedicated file) becomes
  `package main`.

## Migration

1. Move `cmd/*.go` → `cmd/ts-bridge/`
2. Update import in `main.go`: `"ts-bridge/cmd"` → `"ts-bridge/cmd/ts-bridge"`
3. Remove root `main.go`
4. Verify: `go build ./...` and `go test ./...`

## Related

- [adr-007-multi-package-split.md](adr-007-multi-package-split.md) — internal package split
- [adr-008-cli-architecture.md](adr-008-cli-architecture.md) — Cobra adoption
