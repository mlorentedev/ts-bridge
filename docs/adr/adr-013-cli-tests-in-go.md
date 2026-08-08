---
id: "adr-013"
type: adr
status: accepted
date: "2026-08-08"
tags: [testing, ci, cli, tooling]
owner: manu
---

# ADR-013: CLI Coverage Belongs in Go Tests, Not a Second Native Suite

## Context

CLI behaviour was covered by two subprocess suites in `scripts/tests/`:

- `smoke.bats` — BATS, Linux, run by the `smoke` CI job
- `smoke.ps1` — PowerShell, Windows, run by **no CI job at all**

The split looked like platform parity. It was not. Three problems compounded:

1. **Half of it never ran.** No workflow invoked `smoke.ps1`. A test file that
   no job executes is documentation, and it rots invisibly: the file asserted
   that `-v` prints a version, when `-v` is the shorthand for the global
   `--verbose` flag and never was a version alias. `--version` itself had no
   coverage. Filed as #244 and #271.

2. **Dual suites drift by construction.** Two files asserting the same
   behaviour in two languages have no mechanism keeping them in agreement.
   Every fix has to be written twice, and the second write is the one that gets
   skipped. #244 is that failure, observed.

3. **Subprocess suites are invisible to the tooling that matters.** Coverage
   does not see them, and mutation testing does not either — `gremlins` runs
   `go test` per mutant, so a BATS or PowerShell assertion kills exactly zero
   mutants. QA-014 measured 205 of 241 module-wide `NOT COVERED` mutants living
   in `cmd/cli`, the package the subprocess suites were nominally covering.

The decisive fact is that the cross-platform problem was already solved: CI's
`test-windows` job runs `go test` on `windows-latest`. Go tests are therefore
cross-platform at **zero** additional CI cost.

## Decision

**Cross-platform CLI coverage belongs in Go tests under `cmd/cli/`.**

- New CLI behaviour is tested in Go. It runs on Linux *and* Windows through the
  existing `test` and `test-windows` jobs, and is visible to coverage and to
  `gremlins`.
- `smoke.bats` is **frozen** as the Linux wiring check — that the built binary
  starts, parses flags, and refuses what it should refuse. It is not extended
  with new behavioural cases and is not rewritten.
- `smoke.ps1` is retired (#277). Its 19 cases were mapped first: 17 were already
  covered by `smoke.bats`, 2 were defective, and **0** were unique.
- A test that must reach unexported production symbols uses an internal test
  file (`package cmd`) rather than a hand-built replica of the command tree.
  Asserting against a replica tests the replica; see #279 for four tests that
  did exactly that, and #278 for the constructor refactor that would remove the
  need.

## Consequences

**Positive**

- One suite per concern instead of two per platform; no drift surface.
- CLI code becomes measurable — coverage and mutation testing finally see it.
- No new CI minutes: the jobs that run these tests already existed.

**Negative**

- Go tests do not exercise the *binary*, only the code it is built from. The
  wiring check that `smoke.bats` provides is genuinely different, which is why
  it is frozen rather than deleted.
- `cmd/cli` currently frustrates testing: `rootCmd` is private and seven
  `init()` functions register subcommands, so a test cannot construct the real
  tree (#278). Internal test files are the interim workaround, and applying it
  while fixing #282 is what re-surfaced the issue.

## Related

- #277 — retires `smoke.ps1`; #271 — it ran in no CI job; #244 — the drift it caused
- #278 — extract a command-tree constructor; #279 — tests asserting against replicas
- [ADR-010](adr-010-cli-package-layout.md) — the `cmd/cli` package layout these tests target
- [`docs/lessons.md`](../lessons.md) — the smoke-test principle and the unexecuted-test finding
