---
id: "qa-004-smoke-test-suite"
type: spec
status: proposed
created: "2026-06-18"
tags: [spec, testing, qa, smoke, e2e, automation]
issue: 78
---

# QA-004: Verification

## Evidence

### QA-004 (#173) — CLI parsing

- [x] `bats scripts/tests/smoke.bats` — 19/19 green (bats 1.13.0, local) covering the CLI-parsing surface
- [x] `smoke` CI job added (`.github/workflows/ci.yml`, Linux/BATS) — pending first run on the PR
- [x] `docs/qa-coverage.md` created (living matrix; 100% flag coverage is the QA-004 umbrella end-state, filled by QA-005..QA-013)
- [ ] `powershell -File scripts/tests/smoke.ps1` — all tests green on Windows *(existing suite; expansion deferred)*
- [ ] CI `smoke` job green on the PR *(confirm after push)*

## Test Output

```
$ bats scripts/tests/smoke.bats
1..19
ok 1 version: prints name and commit
ok 2 version --short: prints only the version token
ok 3 --version flag: mirrors the version subcommand
ok 4 --help: lists every top-level subcommand
ok 5 -h: short help flag works
ok 6 help subcommand: equivalent to --help
ok 7 no args: falls back to help, exits 0
ok 8 -v (verbose) without a subcommand: prints help, not a version
ok 9 connect --help: reachable, documents --target
ok 10 init --help: reachable, documents --target
ok 11 status --help: reachable
ok 12 discover --help: reachable, documents --json
ok 13 import --help: reachable, documents the descriptor argument
ok 14 version --help: reachable, documents --short
ok 15 host --help: lists its subcommands
ok 16 host setup --help: reachable
ok 17 host check --help: reachable
ok 18 unknown command: exits non-zero with a clear message
ok 19 unknown flag: exits non-zero with a clear message
```

## Commit Hashes

- Smoke test implementation: 
- CI integration: 
- Coverage matrix: 
