---
tags: [spec, tasks, ARCH-004]
created: "2026-05-18"
---

# Tasks - ARCH-004

> TDD order. One task = one focused commit. Frozen now that spec is `implementing`.

## Setup

- [x] Branch from master: `feat/reconnect-dialer`
- [x] Proposal complete + Q1/Q2/Q3 resolved with code evidence
- [x] Vault canonical doc reviewed

## Implementation

### Config layer

- [x] Add `DialRetries int`, `DialBackoffBase`, `DialBackoffMax` to `Config`.
- [x] Write failing config tests: defaults, valid values, invalid/negative rejected, max<base rejected.
- [x] Wire parsing in `LoadConfig` (new `parseDialRetries` for retries — int 0 allowed; existing `parseDurationEnv` for backoffs).
- [x] Update test cleanup list with the three new env vars.
- [x] `go test ./internal/config/...` green.

### Proxy layer — ReconnectDialer

- [x] Write failing tests in `internal/proxy/reconnect_test.go` covering the seven canonical scenarios plus three extras (overflow, error wrapping, permanent classifier).
- [x] Implement `ReconnectDialer`, `isPermanentDialError`, `computeBackoff` in `internal/proxy/reconnect.go`.
- [x] Use `math/rand/v2` (Go 1.22+) — avoids deprecated `math/rand` global lock.
- [x] `go test ./internal/proxy/...` green (10 new tests, all PASS).

### Wiring

- [x] In `main.go`, wrap `server` with `&proxy.ReconnectDialer{...}` and pass to `AcceptLoop`.
- [x] Existing integration + main tests still pass (default `DialRetries=3` is transparent for successful dialers).

### Documentation

- [x] Update `.env.example` with the three commented vars.
- [x] (No README env-var table exists — confirmed via grep, N/A.)

### Closing

- [x] Every acceptance criterion covered by ≥1 test (see verification.md).
- [x] `go test ./...` green (race deferred to CI).
- [x] `go vet ./...` clean.
- [x] `verification.md` filled.
- [ ] PR opened referencing `specs/ARCH-004/`.
- [ ] PR description marks ARCH-005 subsumed; vault `11-tasks.md` updated post-merge.
