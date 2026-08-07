---
tags: [spec, tasks, testing, qa, config, precedence]
created: "2026-08-07"
---

# Tasks - QA-011-config-precedence

> TDD order. One task = one focused commit. Tick as you go.
>
> **TDD note.** These are tests over already-correct code, so "failing test
> first" cannot mean "red until implemented". It means each test must be shown
> *capable* of failing. The gremlins run in the Closing section is that proof at
> scale: a test that kills no mutant has not been demonstrated to constrain
> anything.

## Setup

- [x] Work-gate verified: #181 OPEN
- [x] `proposal.md` complete and acceptance criteria testable
- [ ] No open questions left in `proposal.md` "Risks / open questions"
      — **blocked on the zero-sentinel confirmation** (see Q1 below)

## Open question blocking AC2

- [ ] **Q1 — zero-sentinel rule.** Confirm that a zero/empty value in a higher
      layer deliberately does NOT override a lower layer (`timeout: 0` in YAML
      leaves the default). Required before the boundary rows are written; AC1
      work does not depend on it.

## Implementation

- [ ] [P] [AC1] Inventory every field `Merge()` applies, per layer, into a
      single table fixture — the contract made explicit as data.
- [ ] [AC1] String fields: cases proving flag > env > YAML > default, unique
      values per layer, none equal to the default (`applyStringFields`,
      `applyFlagString`, `applyEnvString`).
- [ ] [AC1] Duration fields: same, for the six duration fields
      (`applyDurationFields`, `applyEnvDuration`) — the second-largest survivor
      cluster at 11.
- [ ] [AC1] Numeric fields: same, for `DialRetries` / `MaxConnections`
      (`applyEnvInt`, `applyEnvInt64`).
- [ ] [AC2] Boundary rows: zero/empty in a higher layer does not override —
      at least one string, one duration, one numeric. **Blocked on Q1.**
- [ ] Fix the value collision in `TestMergeFullPrecedence`: the flag is set to
      30s, which equals `defaultTimeout`. The test still constrains (env is 1m,
      so deleting `applyFlags` fails it), but the assertion cannot distinguish
      "flag won" from "default survived". Give the flag a non-default value.
- [ ] Refactor: fold overlapping existing precedence tests into the table where
      it reduces duplication without losing a case. Do not mass-rewrite the
      other 40 tests — the mutation run adjudicates what still earns its place.

## Closing

- [ ] Every acceptance criterion covered by at least one test
- [ ] `go test ./...` green; `go test -race ./internal/config/` green
- [ ] `golangci-lint run` clean (pinned v2.12.2, scratchpad GOBIN — never
      overwrite the global toolchain)
- [ ] **Local gremlins run** (`gremlins@v0.6.0` into a scratchpad GOBIN):
      `merge.go` survivors 43 → ≤5, remainder explained
- [ ] `docs/qa-coverage.md` QA-011 row updated (the doc's own contract: each
      landing ticket updates its row)
- [ ] No production code in the diff
- [ ] `verification.md` filled in
- [ ] PR opened with `Closes #181`

## Notes

- **Fixtures:** QA-004 anticipated `scripts/tests/fixtures/` for this ticket.
  That was the BATS-era plan. In Go the fixtures materialize as YAML written to
  `t.TempDir()` inside the test — no committed fixture files needed.
- **Env:** use `t.Setenv` (auto-restores, fails on parallel tests) rather than
  `os.Setenv` + `defer os.Unsetenv`, which the existing file uses 39 times and
  which does not restore a pre-existing value.
- **Mutation turnaround:** the full module run took 111s in CI, so the backstop
  is same-day locally, not a one-week wait for the Monday cron.
