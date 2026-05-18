---
tags: [spec, tasks, REL-003]
created: "2026-05-18"
---

# Tasks - REL-003

> TDD order. One task = one focused commit. Frozen now that spec is `implementing`.

## Setup

- [x] Branch created from master: `feat/idle-timeout`
- [x] `proposal.md` complete, acceptance criteria testable
- [x] No open questions remaining in proposal

## Implementation

### Config layer

- [x] Write failing tests for `TS_IDLE_TIMEOUT` parsing in `internal/config/config_test.go`:
  default disabled, valid `5m`, invalid garbage, negative rejected.
- [x] Add `IdleTimeout time.Duration` to `Config` struct in `internal/config/config.go`.
- [x] Parse `TS_IDLE_TIMEOUT` via existing `parseDurationEnv` helper with explicit
  negative-value rejection.
- [x] Add `TS_IDLE_TIMEOUT` to the test cleanup list at the bottom of the table-driven test.
- [x] `go test ./internal/config/... -v` green.

### Proxy layer

- [x] Write failing tests in `internal/proxy/proxy_test.go` for the idle layer
  (`TestWithIdleTimeout{Disabled,Enabled}`, `TestIdleConnRead{TimesOutWithoutActivity,SucceedsWithTraffic}`, `TestIsIdleTimeoutErr`).
- [x] Implement `idleConn` wrapper in `internal/proxy/proxy.go`
  (embeds `net.Conn`, overrides `Read`); plus `withIdleTimeout` constructor.
- [x] Wire `withIdleTimeout` into `handleConn` for both `client` and `remote`
  (no-op when `cfg.IdleTimeout == 0`, so existing behavior is preserved).
- [x] Add idle-timeout classification via `isIdleTimeoutErr`: distinguish
  `net.Error.Timeout()` from other errors; emit `info` log "connection closed
  (idle timeout)" instead of `warn` "copy error". Does not increment error metric.
- [x] `go test ./internal/proxy/... -v` green.

### Documentation

- [x] Update `.env.example` with commented `TS_IDLE_TIMEOUT` entry.
- [x] `ts-bridge/CLAUDE.md` env-var table: verified — file has no env-var table; nothing to update there.

### Closing

- [x] Every acceptance criterion in `proposal.md` covered by at least one test.
- [x] `go test ./...` green project-wide (race detector deferred to CI; cgo not on Windows dev box).
- [x] `go vet ./...` green.
- [x] `verification.md` filled.
- [ ] PR opened referencing `specs/REL-003/`.

## Machine-readable features

Not generating `features.json` for this spec — the project does not yet use the
harness-gated `features.json` lifecycle. Adopting it is a separate decision
that should land via `_meta` first. Acceptance criteria are tracked manually
in `verification.md`.
