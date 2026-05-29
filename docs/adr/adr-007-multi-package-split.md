---
id: "adr-007"
type: adr
status: accepted
date: "2026-03-07"
tags: [architecture, refactor, packaging, testability]
owner: manu
created: "2026-05-18"
---

# ADR-007: Split monolithic main.go into internal packages

> Retroactive ADR. The refactor itself landed in PR #14 (`feat: graceful drain and multi-package refactoring`) on 2026-03-07; this document captures the decision rationale that was lived through commits + a `90-lessons.md` entry but never crystallized as an architectural record. Created during 2026-05-18 maintenance-mode cleanup.

## Context

Before this refactor, `main.go` was ~600+ lines containing:
- Config parsing (env vars + validation)
- `tsnet.Server` lifecycle
- Local listener + accept loop
- Per-connection handling (`handleConn`)
- Bidirectional copy proxy (`proxyConnections`)
- Health/metrics HTTP server
- Graceful shutdown + drain logic
- Telemetry counters (atomic)

Symptoms reaching ADR threshold:

- `golangci-lint` `gocyclo` failed: the `run()` function reached cyclomatic complexity 19, over the 15 threshold.
- Cross-cutting changes (e.g. adding graceful drain in REL-001) required touching disparate code regions in the same file, increasing review friction.
- Unit testing the proxy logic required either (a) a real Tailscale network, or (b) heavy stubs around `*tsnet.Server` — both prohibitively expensive for a contained piece of business logic.
- The `Dialer` interface extraction (see [adr-006-dialer-interface-extraction.md](adr-006-dialer-interface-extraction.md)) decoupled `tsnet.Server` from `handleConn`, but the consuming code still lived in `main.go`, defeating most of the testability win.

## Decision

Split `main.go` into thin orchestrator + four `internal/` packages:

```text
ts-bridge/
├── main.go                  # ~270 lines — flags, logger, signals, glue
└── internal/
    ├── config/              # env parsing + Config struct + validation
    │   ├── config.go
    │   └── config_test.go
    ├── proxy/               # Dialer interface, AcceptLoop, handleConn,
    │   ├── proxy.go         #   proxyConnections, IsExpectedCloseError;
    │   └── proxy_test.go    #   later REL-003 idleConn + ARCH-004 ReconnectDialer
    ├── health/              # /health/live, /health/ready, /metrics HTTP server
    │   └── health.go
    └── telemetry/           # atomic counters + read accessors (no business logic)
        └── metrics.go
```

`main.go` becomes a thin orchestrator: parse flags → load config → init logger → init tsnet → start listener → start health server → enter accept loop → handle drain on signal → close. The four packages have no circular dependencies (verified: `health` and `proxy` both depend on `telemetry`; `config` depends on nothing project-local).

## Options considered

1. **Status quo: single-file `main.go`** — *Rejected*. Already at 600 lines with rising complexity; gocyclo failures becoming routine.
2. **Two packages: `main` + `internal/core`** — *Rejected*. Single bucket doesn't address the cyclic-dependency risk between proxy and health (both want to read metrics), and doesn't enable independent test compilation.
3. **Five+ packages** — *Rejected*. Over-engineering for a 1000-line codebase. Four boundaries map cleanly to the four observable concerns; finer splits would invent abstractions.
4. **External package (`pkg/...`)** — *Rejected*. Nothing in this repo is meant to be consumed by external Go programs; `internal/` enforces "this is application code, not a library."
5. **Chosen: four-package split under `internal/`** — see above.

## Consequences

### Positive

- **Unit-testable proxy.** With `Dialer` interface (ADR-006) + `internal/proxy` package, `handleConn` and `proxyConnections` can be exercised by `net.Pipe()` mocks. Coverage of proxy logic went from ~24% to >40% after this refactor (also enabled REL-003 and ARCH-004 to land with high test density).
- **Cyclomatic complexity dropped from 19 to 7** in `run()` (per [lessons.md](../lessons.md) entry 2026-03-07). Each helper (`initTailscale`, `handleShutdown`, `drainActiveConnections`) sits below the lint threshold by construction.
- **Reviewer cognitive load reduced.** A PR that touches the proxy reads only `internal/proxy/`; a PR that touches health-server reads only `internal/health/`. `main.go` rarely changes.
- **Telemetry isolated** as `internal/telemetry`, breaking the cyclic temptation where both `proxy` and `health` might define their own counters.

### Negative / trade-offs

- **`main.go` glue grows over time.** Each new component (idleConn wrapping, ReconnectDialer wrapping, future ARCH-* additions) adds a few lines of glue. Acceptable — it's still O(packages), not O(lines-of-business-logic).
- **Cross-package types** (e.g. `config.Config` passed to `proxy.AcceptLoop`) create a small public surface. Mitigation: `Config` is a value type, no pointer aliasing risk; only proxy reads what it needs and the rest is ignored.
- **`internal/` packages are not consumable externally.** Anyone wanting to embed ts-bridge as a library would need to fork or extract. Acceptable — the project's stated goal is a single binary, not a library (ADR-002 §"single binary").

### Neutral

- The four-package boundary is **stable**: nothing since the original refactor has wanted to cross a boundary or merge two packages. The slicing matches the natural seams of the code.

## Validation

- `go test ./...` after refactor: passing, with 35+ unit tests across the four packages (was ~15 in monolithic main).
- `golangci-lint run` clean post-refactor (no `gocyclo` failures observed since 2026-03-07).
- Two subsequent features (REL-003 idle-timeout in v1.6.0, ARCH-004 ReconnectDialer in v1.7.0) landed entirely inside `internal/proxy/` with zero changes to `main.go` glue except a one-line wiring — supporting the "stable boundary" claim.

## Related

- [adr-002-single-binary-no-config.md](adr-002-single-binary-no-config.md) — single-binary, env-var-only design that this split preserves.
- [adr-004-atomic-metrics-over-mutex.md](adr-004-atomic-metrics-over-mutex.md) — telemetry isolation made this trivially correct.
- [adr-006-dialer-interface-extraction.md](adr-006-dialer-interface-extraction.md) — interface that enabled the proxy package to be testable.
- [lessons.md](../lessons.md) entry 2026-03-07 — "Managing Cyclomatic Complexity in Go projects" lesson that triggered this work.
- Repo: PR #14 (the refactor itself).
