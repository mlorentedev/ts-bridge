---
tags: [spec, verification, testing, qa, config, precedence]
created: "2026-08-07"
---

# Verification - QA-011-config-precedence

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof.

- [ ] AC1 — every applied field has a winning-layer case, unique non-default
      values per layer -> test `<name>` / commit `<hash>`
- [ ] AC2 — boundary rows prove zero/empty does not override, for a string, a
      duration, and a numeric field -> test `<name>` / commit `<hash>`
- [ ] AC3 — `merge.go` survivors 43 -> ≤5 -> gremlins run `<local/CI>`, output
      pasted below
- [ ] AC4 — no production code changed -> `git diff --stat` shows only
      `_test.go` + `specs/`

## Test status

- Test suite: `go test ./... -> <output>`
- Race: `go test -race ./internal/config/ -> <output>`
- Lint: `golangci-lint run (v2.12.2) -> <output>`
- Mutation, before: `merge.go` 50 killed / **43 lived** / 8 not covered
  (run `30789587228`, 2026-08-03)
- Mutation, after: `<paste killed/lived/not-covered for merge.go>`
- No regressions in existing suite: <yes / no>

## Baseline for comparison

Survivors in `merge.go` by function at the start of this work (43 total):

| Function | NEGATION | BOUNDARY |
|---|---|---|
| `applyFlags` | 6 | 6 |
| `applyDurationFields` | 5 | 6 |
| `applyStringFields` | 3 | 0 |
| `applyYAML` | 2 | 2 |
| `LoadYAMLConfig` | 3 | 0 |
| `applyEnvInt` | 2 | 0 |
| `validateTarget` | 0 | 2 |
| `applyEnv` / `applyEnvDuration` / `applyEnvInt64` | 3 | 0 |
| `validateDialRetries` | 0 | 1 |
| predicates (`positiveDuration` etc.) | 1 | 1 |

Any survivor remaining after this work must be named here with a reason —
"not reachable from `Merge()`", "equivalent mutant", or "deferred to
`<issue>`". An unexplained survivor means AC3 is not met.

## Decisions made during implementation

-
-

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons.md`? <yes / no - one line>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes / no>
- [ ] New pattern candidate for `00_meta/patterns/`? <yes / no>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/QA-011-config-precedence/` -> `specs/archive/`
- [ ] #181 closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
