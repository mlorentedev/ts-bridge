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
