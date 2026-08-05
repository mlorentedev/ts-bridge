---
id: "deps-001-tailscale-client-v2"
type: spec
status: proposed
created: "2026-07-10"
tags: [spec, deps, tailscale, discover, migration, sa1019]
issue: 245
---

# DEPS-001: Verification

## How to verify

| # | Check | Command | Expected |
|---|-------|---------|----------|
| 1 | tailscale bumped | `grep 'tailscale.com v' go.mod` | `tailscale.com v1.100.0` |
| 2 | tidy clean | `go mod tidy && git diff --exit-code go.mod go.sum` | no diff |
| 3 | no v1 client import | `grep -R 'client/tailscale"' internal/` | no matches |
| 4 | builds | `go build ./...` | exit 0 |
| 5 | vet clean | `go vet ./...` | exit 0 |
| 6 | tests green | `go test ./...` | ok (discover conversion tests pass) |
| 7 | SA1019 cleared | CI `lint` job (golangci-lint v2.12.2) | green — no SA1019 |
| 8 | behaviour unchanged | `discover` unit tests | same device fields populated |

Check 7 is the gate that blocks Dependabot #199; it is confirmed by CI because
`golangci-lint` is not reliably runnable on the maintainer's Windows box.

## Results

Verified locally on the migration branch (build/vet/test run detached to avoid
module-cache corruption). Check 7 (SA1019) is CI-only — `golangci-lint` does not
run reliably on the maintainer's Windows box.

- [x] 1 `go.mod` → `tailscale.com v1.100.0` + `tailscale.com/client/tailscale/v2 v2.10.1` (direct)
- [x] 2 `go mod tidy` clean (ran before build; no residual diff)
- [x] 3 `grep -rn 'client/tailscale"' internal/ cmd/` → no matches
- [x] 4 `go build ./...` → ok
- [x] 5 `go vet ./...` → ok
- [x] 6 `go test ./...` → all packages ok (`internal/discover` incl. the new nil-LastSeen + Time round-trip asserts)
- [x] 7 CI `lint` job green on the PR #252 merge commit and on `master` since — SA1019 cleared, which unblocked and superseded Dependabot #199 (closed)
- [x] 8 behaviour unchanged — conversion tests assert the same populated fields; `DeviceID` (from v2 `ID`) and `*Time`→RFC3339 round-trip covered
