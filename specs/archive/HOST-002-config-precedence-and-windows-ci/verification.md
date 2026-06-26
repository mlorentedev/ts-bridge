---
id: "HOST-002"
type: spec
status: proposed
created: "2026-06-19"
tags: [spec, host, config, precedence, ci, windows, bugfix]
issue: 193
---

# HOST-002: Verification

## Evidence

- [ ] `go build ./...` — BUILD_OK
- [ ] `go vet ./internal/host/... ./cmd/cli/...` — clean
- [ ] `go test ./internal/host/... ./cmd/cli/...` — green
- [ ] `TestMerge_BoolFlagPrecedence` passes (both subtests)
- [ ] `TestWriteHostEnv_FilePermissions` runs on Linux CI (no longer skipped)
- [ ] CI `test-windows` job green on `windows-latest`
- [ ] `git diff --exit-code go.mod go.sum` — no module changes

## Test Output

```
# Paste: go test ./internal/host/... ./cmd/cli/... -run 'TestMerge_BoolFlagPrecedence|TestWriteHostEnv_FilePermissions'
# Paste: CI run URL for the test-windows job
```

## Commit Hashes

- Fixes + Windows CI + spec:
- First green CI run (incl. test-windows):
