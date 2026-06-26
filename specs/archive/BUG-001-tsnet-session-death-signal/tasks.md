---
id: "BUG-001"
type: spec
status: draft
created: "2026-06-26"
---

# Tasks — BUG-001

> TDD order. One task = one focused commit.

## Setup

- [ ] Branch created: `fix/BUG-001-tsnet-session-death-signal`
- [ ] `proposal.md` acceptance criteria all testable — done

## Implementation

### 1. TerminalDialError + isPermanentDialError coverage

- [ ] Test: `isPermanentDialError("tsnet: backend in state NeedsMachineAuth")` → true
- [ ] Test: `isPermanentDialError("tsnet: backend in state Stopped")` → true
- [ ] Test: transient error (e.g. `"connect: connection refused"`) → false, retries, does NOT wrap as terminal
- [ ] Implement `TerminalDialError` (typed wrapper); `ReconnectDialer.Dial` returns it when `isPermanentDialError` fires instead of plain `err`
- [ ] Test: `errors.As(err, &TerminalDialError{})` distinguishes terminal from retries-exhausted

### 2. Health state with reason

- [ ] Test: `TunnelStatus.Load()` returns `{ready:true, reason:""}` initially; after `Store(false, "tsnet: backend in state Stopped")` returns `{ready:false, reason:"tsnet: backend in state Stopped"}`
- [ ] Implement `TunnelStatus` struct with atomic `Load`/`Store` (ready + reason)
- [ ] Test: `/health/ready` handler returns 200 `{"status":"ok"}` when ready, 503 `{"status":"not_ready","reason":"..."}` when not — table-driven
- [ ] Update `health.StartServer` to accept `*TunnelStatus` instead of `*atomic.Bool`; update `main.go` call site

### 3. Cancel propagation

- [ ] Test: when `AcceptLoop`'s `handleConn` receives a `TerminalDialError`, the provided `cancel` func is called — mock Dialer returning terminal error, assert cancel called within 1s
- [ ] Implement: `handleConn` detects `TerminalDialError`, calls `cancel(terminalErr)`, logs `slog.Error("tsnet session terminal", "reason", ...)`, sets `TunnelStatus` to not-ready with reason

### 4. Regression

- [ ] Test: existing transient-retry behavior unchanged — 3 transient errors then success still returns conn, no cancel called
- [ ] Test: `MaxRetries` exhausted on transient (not terminal) exits with retries-exhausted error, no cancel

## Closing

- [ ] All acceptance criteria from `proposal.md` covered by at least one test
- [ ] `go build ./...`, `go vet ./...` clean
- [ ] Lint passes (no new goconst / staticcheck violations)
- [ ] No scope creep — diff touches only `internal/proxy/reconnect.go`, `internal/health/health.go`, `main.go`, and their test files
- [ ] `verification.md` filled in
- [ ] PR opened with `Closes #208`
