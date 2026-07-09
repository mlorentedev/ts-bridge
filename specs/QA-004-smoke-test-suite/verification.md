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
- [x] `smoke` CI job added (`.github/workflows/ci.yml`, Linux/BATS) — green on PR #242
- [x] `docs/qa-coverage.md` created (living matrix; 100% flag coverage is the QA-004 umbrella end-state, filled by QA-005..QA-013)
- [x] CI `smoke` job green (PR #242, 11/11 jobs) — **merged** `2543bc2`
- [ ] `powershell -File scripts/tests/smoke.ps1` — all tests green on Windows *(existing suite; expansion deferred)*

### QA-005 (#174) — init

- [x] `bats scripts/tests/smoke.bats` — 25/25 green (adds 6 init tests: env/yaml formats, custom path, overwrite protection ±`--force`, auth-key-not-in-yaml, no-TTY fail-fast)
- [x] Per-test CWD isolation via `BATS_TEST_TMPDIR` (idempotent, no state leakage)
- [x] `docs/qa-coverage.md` `init` row → BATS ✅

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
# QA-005 (#174) — init
ok 20 init: env format (default) writes .env with auth key and target
ok 21 init: yaml format writes ts-bridge.yaml and keeps the auth key OUT of it
ok 22 init: --config writes to a custom output path
ok 23 init: refuses to overwrite an existing config without --force
ok 24 init: --force overwrites an existing config
ok 25 init: missing a required flag with no TTY fails fast (does not hang)
```

## Commit Hashes

- QA-004 (#173) CLI parsing — smoke suite + helpers + CI job + coverage matrix: `2543bc2` (PR #242, merged)
- QA-005 (#174) init — this PR
