# ts-bridge

Portable TCP bridge over Tailscale/Headscale mesh networks using tsnet.

## Tech Stack

- **Language:** Go 1.25+
- **Key dependency:** `tailscale.com/tsnet` v1.80.0
- **Architecture:** Single binary, multi-package — `main.go` (~270 lines orchestrator) + four `internal/` packages (see ADR-007).
- **Config:** Environment variables only (no config files) — see `.env.example`
- **Logging:** `log/slog` (structured, text or JSON)
- **Metrics:** `sync/atomic` counters, JSON endpoint at `/metrics`

## Key Paths

| Path | Purpose |
|------|---------|
| `main.go` | Orchestrator: flags, logger, signals, init, drain. Thin glue, no business logic. |
| `main_test.go` | Unit tests for main-package helpers (error diagnosis, etc.) |
| `main_integration_test.go` | Integration tests with mock Dialer (loopback, no tsnet) |
| `internal/config/` | Env-var parsing + `Config` struct + validation |
| `internal/proxy/` | `Dialer` interface, `AcceptLoop`, `handleConn`, `proxyConnections`, `idleConn`, `ReconnectDialer` |
| `internal/health/` | `/health/live`, `/health/ready`, `/metrics` HTTP server |
| `internal/telemetry/` | Atomic counters + read accessors |
| `specs/` (and `specs/archive/`) | Per-feature SDD folders (proposal + tasks + verification) — see §Workflow Rules |
| `.env.example` | Configuration reference (2 required vars + commented optionals) |
| `scripts/client/` | Client launchers (`run.sh`, `run.ps1`, `bootstrap.{sh,ps1}`) |
| `scripts/host/` | Host setup (`setup.ps1`, `ts-bridge.service`) |
| `scripts/tests/` | BATS tests for the launchers |
| `.github/workflows/ci.yml` | CI: test, lint, security (gosec), shellcheck, bats, build-matrix |
| `.github/workflows/release.yml` | Automated releases via release-please (PAT-driven) |

## Commands

```sh
# Build
go build -o ts-bridge .

# Test (always use race detector)
go test -race -v ./...

# Lint (CI pins this version — run the same locally to avoid drift)
go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.62.2
golangci-lint run

# Security scan
gosec ./...

# Run in dev mode
./scripts/dev.sh
```

## Architecture Decisions

- **ADR-002:** Single binary, no config files, env-var driven
- **ADR-004:** Atomic metrics, no mutexes
- **ADR-006:** `Dialer` interface for testability
- **ADR-007:** Multi-package split under `internal/` (this is the current layout — see Key Paths)
- Full ADR index in vault: `knowledge/10_projects/ts-bridge/30-architecture/`

## Workflow Rules (read before first tool call)

This repo opts in to the global behaviour rules in `~/Projects/dotfiles/AGENTS.md`.
Read it once at session start and apply its §Spec-Driven Development +
§Standing Orders + §Operational Rules. Specifically:

- **SDD by default** — every PR-sized change (~50-300 lines, public-contract
  touch, new dep, or multi-PR sequence) gets a transient `specs/<feature-id>/`
  with `proposal.md` + `tasks.md` + `verification.md`. Templates in vault at
  `00_meta/templates/spec-{proposal,tasks,verification}.md`. Archived to
  `specs/archive/<feature-id>/` on merge.
  Skip SDD only for: typos, comment-only edits, mechanical refactors,
  bug fixes <20 lines with obvious cause, doc-only changes.
- **TDD inside the spec** — failing test first, then implementation. Already
  the project standard; SDD wraps it, does not replace it.
- **Atomic PRs** — one logical change per PR, ~300 line hard cap (tests, lockfiles, generated files excluded).

## Conventions

- Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`)
- GitHub Flow — all work via feature branches + PRs against `master`
- TDD — write failing tests first
- Table-driven tests with `t.Run` subtests
- Functions < 40 lines, cyclomatic complexity < 10
- Error wrapping: `fmt.Errorf("context: %w", err)`
- No new dependencies without strong justification (zero-dep design goal)
