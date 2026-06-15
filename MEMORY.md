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

---

# Session 2026-06-15b — Bugs batch close + branch protection

## Continuity Block

**Fecha:** 2026-06-15 (segunda sesión del día)

**Última tarea:** Cierre de los 6 bugs pendientes (BUG-001, 002, 004, 009, 010, 016) + documentación de branch protection enforcement.

**Estado de bugs:**
| Bug | Estado | PR |
|-----|--------|-----|
| BUG-016 (#111) | ✅ Merged | #141 `cdf6959` |
| BUG-010 (#101) | ✅ Cerrado | #139 `d8186df` |
| BUG-009 (#100) | ✅ Cerrado | #139 `d8186df` |
| BUG-004 (#95) | ✅ Cerrado | code fix en master |
| BUG-002 (#93) | ✅ Cerrado | code fix en master |
| BUG-001 (#92) | ✅ Cerrado | code fix en master |

**Problema crítico encontrado:** `enforce_admins` está `false` en branch protection de master. Esto permite bypass de las reglas con push directo (PAT con scope `repo`). Los commits directos a master fueron hechos 2 veces antes de darse cuenta.

**Solución aplicada:**
- Se documentó la regla en AGENTS.md: nueva sección `### Branch Protection Enforcement (NON-NEGOTIABLE)` en `## Operational Rules`
- Se creó PR #141 correctamente (fix/bug-016-structured-logger → master)
- Los reverts `86be381`/`ed41a7e` fueron reaplicados para restaurar los fixes

**Acción requerida del usuario (CRÍTICO):**
1. Ir a: https://github.com/mlorentedev/ts-bridge/settings/branches
2. Click en `master` (branch protection rule)
3. Scroll a "General" → habilitar **"Enforce administered restrictions on matching branches"**
4. Esto establece `enforce_admins: true` y evita cualquier bypass futuro
5. Sin esto, cualquier commit directo a master (incluyendo de bots/PATs) bypassa las reglas

**Tareas pendientes (orden recomendado):**

1. **REFACTOR-004** (#126) — Reemplazar fmt.Fprintf → slog en merge.go (applyEnvDuration, applyEnvInt, applyEnvInt64) — TRIVIAL, ya hicimos el patrón con BUG-016
2. **REFACTOR-001** (#123) — Split `setupImpl` en helpers — BAJA
3. **REFACTOR-002** (#124) — Split `writeYAMLConfig`/`writeEnvConfig` — BAJA
4. **REFACTOR-003** (#125) — Split `proxyConnections` — MEDIA
5. **TECH-009** (#110) — `//nolint:unused` misleading — TRIVIAL
6. **TECH-007** (#105) — CI bats referencia script archivado — TRIVIAL
7. **TECH-006** (#104) — `sanitizeHostnameLabel` duplicado — BAJA
8. **TECH-008** (#114) — Watch mode test sin assertions — MEDIA
9. **TECH-010/011/012/013** (#117/118/115/116) — Audit minors — evaluar cada hallazgo
10. **QA-001/002/003** (#78/90/91) — E2E tests — requiere entorno real
11. **ADR-008** (#74) — Actualizar estado del ADR — TRIVIAL

**Ruta de trabajo:**
- Todo va por PR → feature branch → PR → review → merge
- master está protegido (PR required, 1 approval, 3 status checks: test, lint, security)
- PERO `enforce_admins: false` — necesita habilitarse para que sea efectivo

**Notas técnicas:**
- Branches viejos `fix/banner-hostname-clean` y `fix/init-bugs` eliminados
- `fix/bug-016-structured-logger` también eliminado (ya merged)
- Working tree limpia en master
