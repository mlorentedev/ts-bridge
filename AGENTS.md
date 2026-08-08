# ts-bridge

Portable TCP bridge over Tailscale/Headscale mesh networks using tsnet.

## Tech Stack

- **Language:** Go 1.26+
- **Key dependency:** `tailscale.com/tsnet` v1.102.1
- **Architecture:** Single binary, multi-package — `cmd/ts-bridge/main.go` (~40 lines, thin entry point) delegates to the `cmd/cli` Cobra tree (ADR-010) over eight `internal/` packages (ADR-007).
- **Config:** Flags, environment variables, and YAML config files, in that precedence order, plus named profiles (ADR-011/012). `.env.example` documents the env-var surface; `init` writes either format.
- **Logging:** `log/slog` (structured, text or JSON)
- **Metrics:** `sync/atomic` counters, JSON endpoint at `/metrics`

## Key Paths

| Path | Purpose |
|------|---------|
| `cmd/ts-bridge/main.go` | Thin entry point — wires flags/logger/signals and delegates to `cmd/cli` (ADR-010). No business logic. |
| `cmd/cli/` | Cobra command tree (`root`, `connect`, `init`, `status`, `host`, `discover`, `import`, `version`) + `args`/`run`/`signal` helpers, with per-command `*_test.go`. |
| `internal/config/` | Env-var parsing + `Config` struct + validation |
| `internal/proxy/` | `Dialer` interface, `AcceptLoop`, `handleConn`, `proxyConnections`, `idleConn`, `ReconnectDialer` |
| `internal/health/` | `/health/live`, `/health/ready`, `/metrics` HTTP server |
| `internal/telemetry/` | Atomic counters + read accessors |
| `internal/discover/` | Tailnet device discovery via Tailscale and Headscale APIs |
| `internal/host/` | Platform-specific host setup/check (firewall, RDP/xrdp, service) — `*_linux.go` / `*_windows.go` / `*_darwin.go` |
| `internal/logging/` | Structured slog logging: text to console + JSON to a rotating log file |
| `internal/profile/` | Shareable connection descriptor (`tsb://`) + profile store behind `connect --profile` (ADR-011/012) |
| `specs/` (and `specs/archive/`) | Per-feature SDD folders (proposal + tasks + verification) — see §Workflow Rules |
| `.env.example` | Configuration reference (2 required vars + commented optionals) |
| `scripts/host/` | Host setup (`setup.ps1`, `ts-bridge.service`) |
| `scripts/tests/` | CLI smoke tests (`smoke.bats`, BATS) exercising the built binary. Cross-platform CLI coverage belongs in Go tests under `cmd/cli/`, which the `test-windows` job already runs on Windows. |
| `Makefile` | Dev task runner — mutation-testing targets (gremlins, install-only; see QA-014) |
| `.github/workflows/ci.yml` | CI jobs: `test`, `test-windows`, `smoke` (bats), `build-matrix`, `lint`, `security` (gosec) |
| `.github/workflows/release.yml` | Automated releases via release-please (PAT-driven) |

## Commands

```sh
# Build
go build -o ts-bridge ./cmd/ts-bridge/

# Test (CI runs -race on Linux; the Windows job omits it — it needs a C toolchain)
go test -race -v ./...

# Lint (CI pins this version via golangci-lint-action@v9 — match it to avoid drift)
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
golangci-lint run

# Security scan
gosec ./...

# Run in dev mode
./scripts/dev.sh
```

## Architecture Decisions

- **ADR-002:** Single binary, no config files, env-var driven — **deprecated**; config files and profiles arrived with ADR-011/012. Kept for the portability rationale, not as current guidance.
- **ADR-004:** Atomic metrics, no mutexes
- **ADR-006:** `Dialer` interface for testability
- **ADR-007:** Multi-package split under `internal/` (this is the current layout — see Key Paths)
- **ADR-010:** `cmd/cli` Cobra package layout — `main.go` stays a thin entry point
- **ADR-012:** Named profile model for multi-tailnet configuration (current config model)
- **ADR-013:** CLI coverage lives in Go tests under `cmd/cli/`, not a second native suite — `smoke.bats` is frozen as the Linux wiring check
- Full ADR index: [`docs/adr/`](docs/adr/) (project-bound knowledge, docs-as-code)

## Documentation

Project-bound knowledge lives in this repo under [`docs/`](docs/) (docs-as-code):

- [`docs/adr/`](docs/adr/) — Architecture Decision Records (the *why* behind decisions)
- [`docs/runbooks/`](docs/runbooks/) — operational procedures (deploy, RDP host setup, multi-device ops)
- [`docs/troubleshooting/`](docs/troubleshooting/) — known errors, security audit, release issues
- [`docs/lessons.md`](docs/lessons.md) — accumulated gotchas and post-mortems

Strategic context, roadmap, and session memory live in the maintainer's cross-project knowledge store and are intentionally not committed here.

## Workflow Rules (read before first tool call)

This repo opts in to the global behaviour rules in `~/Projects/dotfiles/AGENTS.md`.
Read it once at session start and apply its §Spec-Driven Development +
§Standing Orders + §Operational Rules. Specifically:

- **SDD by default** — every PR-sized change (~50-300 lines, public-contract
  touch, new dep, or multi-PR sequence) gets a transient `specs/<feature-id>/`
  with `proposal.md` + `tasks.md` + `verification.md`. Spec templates live in
  the maintainer's cross-project knowledge store (not committed here). Archived to
  `specs/archive/<feature-id>/` on merge.
  Skip SDD only for: typos, comment-only edits, mechanical refactors,
  bug fixes <20 lines with obvious cause, doc-only changes.
- **TDD inside the spec** — failing test first, then implementation. Already
  the project standard; SDD wraps it, does not replace it.
- **Atomic PRs** — one logical change per PR, ~300 line hard cap (tests, lockfiles, generated files excluded).

## Conventions

- Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`)
- GitHub Flow — all work via feature branches + PRs against `master`
- **PR body must include `Closes #N`** for each issue the PR resolves. One per line: `Closes #1, Closes #2`. Without this, issues don't auto-close on merge.
- TDD — write failing tests first
- Table-driven tests with `t.Run` subtests
- Functions < 40 lines, cyclomatic complexity < 10
- Error wrapping: `fmt.Errorf("context: %w", err)`
- No new dependencies without strong justification (zero-dep design goal)
