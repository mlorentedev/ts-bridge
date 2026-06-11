# ts-bridge — Project Memory

> Strategic context, roadmap, and session continuity. Build/operate docs live in `docs/` (repo-bound).

## Session Handoff

> Updated: 2026-06-11
**Last task:** CLI-002 — `ts-bridge connect` subcommand + YAML config support. PR #62 merged (650d252).
**Decisions:** YAML config merged into CLI-002 (subsumed CLI-005); auth key rejected in YAML; `--auth-key-file` for secure loading; `goconst` excluded from `_test.go` in `.golangci.yml`.
**Open threads:** OPS-003 PR #61 still open (go mod tidy CI check); OPS-001/ADR-009 Go 1.25 upgrade pending; TECH-001 test refactoring pending.
**Next action:** Merge OPS-003 PR #61, then pick next CLI subcommand (CLI-003 `init` wizard or CLI-004 `host`).

## Architecture

- **ADR-008:** CLI Architecture — Cobra subcommands + YAML config (docs/adr/adr-008-cli-architecture.md)
- **ADR-009:** Go toolchain 1.24→1.25 + tsnet upgrade (docs/adr/adr-009-toolchain-update.md)
- **ADR-002:** Single binary, env-var driven
- **ADR-004:** Atomic metrics
- **ADR-006:** Dialer interface for testability
- **ADR-007:** Multi-package split under `internal/`

## Current Status

### Merged PRs
- #62 — CLI-002: `ts-bridge connect` + YAML config (2026-06-11)
- #61 — OPS-003: go mod tidy CI check (2026-06-11)
- #60 — Release v1.8.0
- #48 — CLI-001: Cobra scaffold + version subcommand

### Open Issues (bitácora)
- #57 DOCS-001: Update Starlight docs site
- #56 OPS-003: go mod tidy check (PR #61 merged, issue still open)
- #55 OPS-002: Remove obsolete client scripts
- #54 OPS-001: Update Go toolchain 1.24→1.25
- #53 ENH-006: CLI-006 `status` subcommand
- #51 ENH-004: CLI-004 `host` subcommand
- #50 ENH-003: CLI-003 `init` wizard
- #47 TECH-005: Auth key in YAML security
- #46 TECH-004: parseBoolEnv truthy values
- #45 TECH-003: CLAUDE.md vs go.mod version mismatch
- #44 TECH-002: ADR-006 status 'accepted'
- #43 TECH-001: Duplicate proxy pattern in integration tests
- #42 FIX-003: Env var leak in tests
- #41 FIX-002: Health endpoint tests use raw TCP
- #40 FIX-001: Legacy integration tests data race
- #39 ADR-009: Go toolchain update
- #38 ADR-008: CLI Architecture

## Lessons

(See `docs/lessons.md` for project-specific lessons)
