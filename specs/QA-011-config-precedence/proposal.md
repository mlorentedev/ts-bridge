---
id: "QA-011-config-precedence"
type: spec
status: draft # draft | implementing | verifying | archived
created: "2026-08-07"
issue: "ts-bridge#181"
tags: [spec, proposal, testing, qa, config, precedence, mutation]
template_version: "1.0"
---

# QA-011: Config Precedence

<!-- from issue #181: QA-011: BATS smoke test — config precedence (flags > env > YAML > defaults) -->

## Why

`Merge()` implements the documented precedence contract — flags > env > YAML >
defaults — across ~17 configuration fields, each behind its own guard. The
existing tests assert that contract for exactly **two** of them: `Target` and
`Timeout`. The other ~15 fields have their per-field guards exercised but never
discriminated, so a wrong precedence rule on any of them ships green.

This is not a hypothesis. The QA-014 mutation harness reports **43 surviving
mutants in `internal/config/merge.go`** — 25 `CONDITIONALS_NEGATION` and 18
`CONDITIONALS_BOUNDARY` — clustered exactly in the per-field apply guards:
`applyFlags` (12), `applyDurationFields` (11), `applyYAML` (4),
`applyStringFields` (3), and the env appliers (5). Each surviving mutant is a
condition that can be inverted or shifted today with no test turning red.

## What

A table-driven Go test suite in `internal/config` that asserts, **per field and
per layer**, which value wins — replacing a two-field spot check with the full
contract.

Concretely, after this PR:

1. Every field `Merge()` applies has at least one case proving the winning layer,
   with each layer given a **unique value distinct from the default** (so the
   assertion cannot be satisfied by "no layer applied anything").
2. Explicit **zero/boundary rows** prove the sentinel rule: a zero or empty value
   in a higher layer does *not* override a lower one — zero means "unset", not
   "set to zero". This is what kills the 18 `CONDITIONALS_BOUNDARY` mutants, and
   unique-value rows alone would not.
3. The suite is Go, so it counts toward `go test -cover` and toward the QA-014
   mutation harness, and runs on Windows via the existing `test-windows` job.

Written in Go rather than BATS (the issue's original title) because gremlins runs
`go test` per mutant: a BATS case kills none of them, so the hardening would be
unmeasurable. Decision recorded on #181 and in #271/#277.

## Out of scope

- **`internal/config/config.go`'s 31 surviving mutants.** Adjacent (env parsing
  and validation) but a different surface; that is QA-014 hardening item #1.
  Take whatever dies incidentally, do not chase it.
- **`cmd/cli` wiring.** The 205 `NOT COVERED` mutants there are a reachability
  problem blocked on #278, not a precedence problem.
- **Any change to `merge.go` behavior.** This is a test-only PR. If a test
  reveals a genuine precedence bug, it gets its own issue and its own PR rather
  than being fixed inline.

## Risks / open questions

- **The zero-sentinel rule is currently implicit.** `applyYAML` uses `> 0` /
  `!= ""` guards, so a YAML `timeout: 0` silently leaves the default in place.
  Writing tests for it **freezes it as contract**. That is almost certainly the
  intended design (it is how "unset" is representable in a struct without
  pointers), but it has never been stated anywhere. **MUST be confirmed before
  the boundary rows are written** — if it is instead a latent bug, this spec
  changes shape.
- **`resolveAutoInstance` has its own precedence chain**
  (`flags.ManualMode` > `TS_AUTO_INSTANCE` > yaml `auto_instance` > default) and
  already has six dedicated tests. Including it would broaden the table
  considerably. [AGENT-DRAFT — review before archive] Proposed: exclude it, note
  the existing coverage, and revisit only if mutants survive there.
- **`LoadYAMLConfig` contributes 3 survivors** but is file I/O, not precedence.
  [AGENT-DRAFT — review before archive] Proposed: in scope only where a temp-dir
  YAML fixture makes it free.

## Acceptance criteria

- [ ] Every field applied by `Merge()` has at least one case asserting which
      layer wins, with per-layer values unique and distinct from the default.
- [ ] Boundary cases exist proving a zero/empty value in a higher layer does not
      override a lower layer, for at least one string, one duration, and one
      numeric field.
- [ ] Surviving mutants in `internal/config/merge.go` drop from 43 to ≤5 in a
      gremlins run, with any remainder explained in `verification.md`.
- [ ] No production code changed — `git diff` touches only `_test.go` files and
      this spec.

## References

- Bitácora board: `ts-bridge#181` (see `issue:` frontmatter)
- Mutation evidence: `specs/archive/QA-014-mutation-testing/verification.md`
- Related: #278 (`cmd/cli` reachability), #271/#277 (why Go, not BATS)
- `docs/qa-coverage.md` — the QA-011 row is updated by this ticket
