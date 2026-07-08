---
id: "BUG-001"
type: spec
status: draft
created: "2026-06-26"
---

# Verification — BUG-001

> Fill in after implementation. Each acceptance criterion maps to at least one command.

## Acceptance criteria checklist

| Criterion | Test / command | Status |
|-----------|----------------|--------|
| Process exits non-zero on terminal tsnet error | `go test ./internal/proxy/... -run TestTerminalDialError` | pending |
| Exit log contains `tsnet session terminal` + reason | `go test ./... -run TestHandleConn_TerminalCancel` | pending |
| `/health/ready` returns 503 with reason before exit | `go test ./internal/health/... -run TestHealthReady_TerminalReason` | pending |
| `TerminalDialError` distinguishable via `errors.As` | `go test ./internal/proxy/... -run TestTerminalDialError_ErrorsAs` | pending |
| Transient errors still retry, no cancel triggered | `go test ./internal/proxy/... -run TestReconnectDialer_TransientNoCancel` | pending |
| Existing retry tests unchanged | `go test ./internal/proxy/... -run TestReconnectDialer` | pending |

## Manual smoke (optional, post-merge)

```sh
# Start connect with a key that will expire, wait for expiry, observe exit code and log
./ts-bridge connect --local-addr 127.0.0.1:2222 --target host:22
# Expected: exits 1, logs "tsnet session terminal", /health/ready returned 503 before exit
```
