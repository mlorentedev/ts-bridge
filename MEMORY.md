# ts-bridge — Project Memory

> Strategic context, roadmap, and session continuity. Build/operate docs live in `docs/` (repo-bound).

## Session Handoff

> Updated: 2026-06-11
**Last task:** Backlog cleanup — TECH-002 (ADR-006 status), TECH-003 (obsolete), TECH-005 (already implemented), TECH-001 (fixed by PR #68). Created ADR-008, ADR-009, TECH-004, DOCS-001, QA-001 as new GitHub issues.
**Decisions:**
- ADR-006 status: `proposed` → `accepted` (it's implemented and in production)
- TECH-003: closed as obsolete (go.mod and AGENTS.md already aligned at 1.25)
- TECH-005: closed as already-implemented (auth key only via env var, never YAML)
- TECH-001: closed as fixed by PR #68 (stale proxy tests removed)
- TECH-004: keep as-is (yes/on truthy values match shell UX)
- ADR-008/ADR-009: created as documentation ADRs for CLI architecture and toolchain
- DOCS-001: Starlight docs update — must happen last, after all other changes
- QA-001: multi-device e2e validation + independent audit — final gate before production-ready
- Build-matrix CI: keep for compilation validation (cross-compile 6 combos), complemented by QA-001 runtime validation
- All legacy feature branches (feat/cli-*) and chore branches cleaned up; work preserved in PRs #79, #80, #81
**Open threads:** PR #79 (ADR-006 status), PR #80 (obsolete scripts), PR #81 (session artifacts) — all waiting CI.
**Next action:** Merge PRs #79, #80, #81 once CI passes, then close TECH-002/TECH-003/TECH-005/TECH-001.

## Architecture

- **ADR-002:** Single binary, env-var driven
- **ADR-004:** Atomic metrics (no mutexes)
- **ADR-006:** Dialer interface for testability (accepted)
- **ADR-007:** Multi-package split under `internal/`
- **ADR-008:** CLI Architecture — Cobra subcommands + YAML config (new issue #74)
- **ADR-009:** Go toolchain 1.24→1.25 + tsnet upgrade (new issue #75)

## Current Status

### Merged PRs
- #62 — CLI-002: `ts-bridge connect` + YAML config
- #61 — OPS-003: go mod tidy CI check
- #60 — Release v1.8.0
- #48 — CLI-001: Cobra scaffold + version subcommand
- #73 — OPS-002: Remove obsolete client scripts

### Open PRs (awaiting CI)
- #79 — `chore/adr-006-status`: ADR-006 status `proposed` → `accepted` (1 line change)
- #80 — `chore/remove-obsolete-scripts`: Remove obsolete scripts + archive specs (closes #55)
- #81 — `chore/preserve-session-artifacts`: Archive CLI-002 spec + MEMORY.md (preserves session handoff artifacts)

### Open Issues (bitácora)
- #78 QA-001: End-to-end validation across multiple devices + independent audit
- #77 DOCS-001: Update Starlight docs site for new CLI commands
- #76 TECH-004: parseBoolEnv accepts non-standard truthy values (yes/on) — keep as-is
- #75 ADR-009: Update Go toolchain 1.24 → 1.25 and tsnet to latest
- #74 ADR-008: CLI Architecture — Cobra subcommands + YAML config
- #57 DOCS-001 (duplicate): Update Starlight docs site
- #46 TECH-004 (duplicate): parseBoolEnv uses non-standard truthy values
- #39 ADR-009 (duplicate): Go toolchain update
- #38 ADR-008 (duplicate): CLI Architecture

## Lessons

(See `docs/lessons.md` for project-specific lessons)
