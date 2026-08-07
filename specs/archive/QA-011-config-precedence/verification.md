---
tags: [spec, verification, testing, qa, config, precedence]
created: "2026-08-07"
---

# Verification - QA-011-config-precedence

## Evidence

- [x] **AC1** — every applied field has a winning-layer case, unique non-default
      values per layer -> `TestPrecedenceStringFields` (6 fields × 3 layers),
      `TestPrecedenceDurationFields` (5 × 3), `TestPrecedenceMaxConnections`,
      `TestPrecedenceDefaultsWhenNoLayerSupplies`. Commit `65a9279`.
      The duration table asserts the property of its own data: a layer value
      equal to the default is a `t.Fatalf`, not a silent weak case.
- [x] **AC2** — boundary rows prove zero/empty does not override, for a string,
      a duration and a numeric field -> `TestZeroInHigherLayerDoesNotOverride`
      (5 subtests). Commit `65a9279`.
- [x] **AC3 (amended)** — every remaining survivor attributed. `merge.go` went
      **43 -> 11** lived, killed **50 -> 90**, not-covered **8 -> 0**.
      Attribution table below. Commits `65a9279`, `818a4f9`.
- [x] **AC4** — no production code changed -> the diff touches only
      `internal/config/precedence_test.go`, `internal/config/merge_test.go`,
      `specs/`, and `docs/qa-coverage.md`.

## Test status

- `go test ./...` — green.
- `go test ./internal/config/` — green.
- `golangci-lint run ./internal/config/...` (v2.12.2, the CI pin, installed into
  a scratchpad `GOBIN`) — **0 issues**.
- Mutation, before: `merge.go` 50 killed / **43 lived** / 8 not covered
  (CI run `30789587228`, 2026-08-03).
- Mutation, after: `merge.go` **90 killed / 11 lived / 0 not covered**.
  Package totals: **151 killed / 75 lived / 17 not covered -> 193 / 41 / 9**;
  efficacy **66.81% -> 82.47%**. Both runs enumerate exactly 243 mutants, so the
  comparison is like-for-like. (The baseline is the CI report of 2026-08-03, not
  an intermediate local run — a mid-work local figure would already include
  these tests and understate the change.)
- No regressions: the pre-existing 40 tests in `merge_test.go` still pass.

## Attribution of the 11 remaining survivors

| Line | Function | Guard | Why it survives |
|---|---|---|---|
| 183 ×2 | `applyYAML` | `yamlCfg.DialRetries > 0` | #282 |
| 225 ×2 | `applyDurationFields` | `idleTimeout >= 0` | #282 |
| 288, 291 | `applyEnvInt` | parse / validate paths | #282 — `TS_DIAL_RETRIES` is its only caller |
| 313 ×2 | `nonNegativeInt` | `n >= 0` | #282 — used only for `DialRetries` |
| 337 ×2 | `applyFlags` | `flags.IdleTimeout >= 0` | #282 |
| 250 | `applyEnv` | `TS_MANUAL_MODE != ""` | Auto-instance chain, out of scope per `proposal.md` |

**Ten of eleven are blocked by #282.** An unset `--dial-retries` / `--idle-timeout`
overwrites whatever env or YAML supplied, so no test can observe which layer
produced the value — the guards are unobservable, not merely untested. When #282
is fixed, `TestPrecedenceIssue282UnsetNumericFlagsClobber` loses its `t.Skip` and
these become killable. The eleventh belongs to `resolveAutoInstance`, which this
spec placed out of scope and which has six dedicated tests already.

## Decisions made during implementation

- **The bug was found by reading, not by testing.** Following the six surviving
  `CONDITIONALS_BOUNDARY` mutants in `applyFlags` led to `>= 0` guards paired
  with a cobra flag default of `0`. Filed as #282 and pinned with a skipped
  test rather than fixed here, keeping this change test-only (AC4).
- **`timeout-coefficient: 3` is not portable across scopes.** Calibrated in CI
  against a whole-module baseline, it produced **206 of 243 mutants `TIMED OUT`**
  when the run was narrowed to one package — the per-mutant timeout appears to
  include compilation, which the baseline measurement does not. Local runs used
  `--timeout-coefficient 200 --workers 2`, giving 0 timeouts in ~50s. A
  `TIMED OUT` verdict is absence of data, not a killed mutant; a narrowed run
  read without checking that column looks like an efficacy collapse.
- **`gremlins` was installed into a scratchpad `GOBIN`**, leaving the global
  toolchain and `go.mod`/`go.sum` untouched (verified by `git diff`).

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons.md`? **yes** — two: a precedence test
      whose winning value equals the default cannot distinguish "this layer won"
      from "nothing applied"; and `timeout-coefficient` does not transfer between
      whole-module and single-package mutation runs.
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no
- [ ] New pattern candidate for `00_meta/patterns/`? no

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/QA-011-config-precedence/` -> `specs/archive/`
- [x] #181 closed by PR #283 (`4089c29`)
- [x] Lessons appended to `docs/lessons.md` (3 entries under `## 2026-08-07`)
