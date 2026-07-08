---
tags: [spec, verification]
created: "2026-07-08"
---

# Verification - UX-004-structured-lifecycle-signals

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [ ] Criterion 1 (READY line, actual bound addr) -> test `<name>` / commit `<hash>`
- [ ] Criterion 2 (`--quiet` suppresses banner, keeps READY) -> test `<name>`
- [ ] Criterion 3 (ERROR reason token per category) -> test `<name>`
- [ ] Criterion 4 (`unknown` fallback never silent) -> test `<name>`
- [ ] Criterion 5 (`detail=` escaping, single line) -> test `<name>`
- [ ] Criterion 6 (no regression in existing `Run`/flag behavior) -> test `<name>`

## Test status

- Test suite: `go test ./... -count=1 -> <output>`
- Manual smoke test: run `connect` against a bad authkey and observe the `ERROR` line; run against a good target and observe `READY`
- No regressions in existing test suite: yes / no (if no, document)

## Decisions made during implementation

-
-

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons.md`? <yes / no>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes / no — the `TOKEN key=value` signal grammar as a stable CLI contract may warrant a short ADR>
- [ ] New pattern candidate for `00_meta/patterns/`? <yes / no>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/UX-004-structured-lifecycle-signals/` -> `specs/archive/`
- [ ] Bitácora tickets #203 and #204 closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
