# ts-bridge

Portable TCP bridge over Tailscale/Headscale mesh networks using tsnet.

## Tech Stack

- **Language:** Go 1.25+
- **Key dependency:** `tailscale.com/tsnet` v1.80.0
- **Architecture:** Single-binary, single-file (`main.go` ~785 lines)
- **Config:** Environment variables only (no config files) — see `.env.example`
- **Logging:** `log/slog` (structured, text or JSON)
- **Metrics:** `sync/atomic` counters, JSON endpoint at `/metrics`

## Key Paths

| Path | Purpose |
|------|---------|
| `main.go` | All application code |
| `main_test.go` | Unit tests |
| `main_integration_test.go` | Integration tests (loopback, no tsnet) |
| `.env.example` | Configuration reference (2 required vars) |
| `scripts/client/` | Client launchers (run.sh, run.ps1, bootstrap) |
| `scripts/host/` | Host setup (setup.ps1, ts-bridge.service) |
| `.github/workflows/ci.yml` | CI pipeline (test, lint, security, shellcheck, build-matrix) |
| `.github/workflows/release.yml` | Automated releases via release-please |

## Commands

```sh
# Build
go build -o ts-bridge .

# Test (always use race detector)
go test -race -v ./...

# Lint
golangci-lint run

# Security scan
gosec ./...

# Run in dev mode
./scripts/dev.sh
```

## Architecture Decisions

- **ADR-002:** Single binary, no config files, env-var driven
- **ADR-004:** Atomic metrics, no mutexes
- Full ADR index in vault: `knowledge/10_projects/ts-bridge/30-architecture/`

## Conventions

- Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`)
- GitHub Flow — all work via feature branches + PRs against `master`
- TDD — write failing tests first
- Table-driven tests with `t.Run` subtests
- Functions < 40 lines, cyclomatic complexity < 10
- Error wrapping: `fmt.Errorf("context: %w", err)`
- No new dependencies without strong justification (zero-dep design goal)
