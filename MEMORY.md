# ts-bridge — Session Memory

> Updated: 2026-06-15

## Session Handoff

**Last task:** Batch bug fixes — BUG-003 (.env auto-load), BUG-017 (banner width), CI fix (dependabot add-to-project). PR #134 (`fix/init-bugs`) with 5 commits, CI green.

**Decisions:**
- `.env` auto-load via new `internal/config/envfile` package — minimal loader, 22 tests, precedent: docker-compose/webpack/rails
- Banner width dynamic with `...` truncation — no hardcoded `%-14s`
- Dependabot PRs excluded from `add-to-project` workflow (token lacks `read:project`)
- BUG-001/002/004 already fixed in prior releases (v1.12.x)

**Open threads:**
- 19 issues still open (BUG-005, 009, 010, TECH-006..013, REFACTOR-001..004, QA-001..003, ADR-008) — board disconnected from repo (items exist in repo but not mapped to GitHub Project #1)
- Bitácora (Project #1) has 110+ items but none from ts-bridge are mapped with fields (Status, Priority, Repository)

**Next action:** Merge PR #134, then tackle BUG-005 (validation order) or BUG-010 (banner concurrent log) — whichever you prefer.
