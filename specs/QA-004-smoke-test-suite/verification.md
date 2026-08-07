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
- [~] `powershell -File scripts/tests/smoke.ps1` — all tests green on Windows *(**dropped in #271** — suite retired unrun; 17 of its 19 cases were already covered by smoke.bats and 2 were defective)*

### QA-005 (#174) — init

- [x] `bats scripts/tests/smoke.bats` — 25/25 green (adds 6 init tests: env/yaml formats, custom path, overwrite protection ±`--force`, auth-key-not-in-yaml, no-TTY fail-fast)
- [x] Per-test CWD isolation via `BATS_TEST_TMPDIR` (idempotent, no state leakage)
- [x] `docs/qa-coverage.md` `init` row → BATS ✅

### QA-006 (#175) — status

- [x] `bats scripts/tests/smoke.bats` — 29/29 green (adds 4 status tests: not-running, `--json` degrades to not-running with no metrics JSON, `--addr` reflected, `--help` documents `--watch`/`--interval`)
- [x] `--watch` loop deliberately not executed (hang risk); flags checked via help, loop covered by `status_test.go` unit tests
- [x] `docs/qa-coverage.md` `status` row + "Deliberately not smoke-tested" note

### QA-007 (#176) — connect

- [x] `bats scripts/tests/smoke.bats` — 36/36 green (adds 7 connect tests: no-target, no-auth, malformed target, malformed auth key, missing auth-key-file, invalid flag value, `--auth-key` visibility warning)
- [x] Only pre-`Runner` paths tested (flag parse + `config.Merge` validation); `setup()` clears `TS_TARGET`/`TS_AUTHKEY` and invalid inputs use bad *format*, so no test starts the bridge or hangs
- [x] Bridge start + graceful shutdown left to `main_integration_test.go` + QA-013 (documented)

### QA-008 (#177) — discover

- [x] `bats scripts/tests/smoke.bats` — 42/42 green (adds 6 discover tests: no-auth, `--json` no-auth with no device JSON, no-tailnet, out-of-range `--port`, non-numeric `--port`, flag surface)
- [x] `setup()` clears `TS_TAILNET`/`TS_CONTROL_URL`/`TS_HEADSCALE_API_KEY` too; no test supplies both auth key and tailnet, so none reaches the tailnet API or the interactive prompt
- [x] Live fetch + interactive selection + `--filter`/`--auto` left to QA-013 (documented)

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
# QA-006 (#175) — status
ok 26 status: reports 'not running' when no bridge is up (exit 0)
ok 27 status --json: no bridge, reports not-running with no metrics JSON
ok 28 status --addr: the queried address is reflected in the message
ok 29 status --help: documents the --watch and --interval flags
# QA-007 (#176) — connect
ok 30 connect: with no target configured, fails asking for one
ok 31 connect: with a target but no auth key, fails asking for one
ok 32 connect: rejects a malformed target before starting
ok 33 connect: rejects a malformed auth key before starting
ok 34 connect: --auth-key-file pointing at a missing file fails fast
ok 35 connect: an invalid flag value is a parse error
ok 36 connect: --auth-key warns that it is visible in the process list
# QA-008 (#177) — discover
ok 37 discover: with no auth key, fails asking for one
ok 38 discover --json: with no auth key, fails and emits no device JSON
ok 39 discover: with an auth key but no tailnet, fails asking for one
ok 40 discover --port: rejects an out-of-range port
ok 41 discover --port: rejects a non-numeric port
ok 42 discover --help: documents the discovery flags
```

## Commit Hashes

- QA-004 (#173) CLI parsing — smoke suite + helpers + CI job + coverage matrix: `2543bc2` (PR #242, merged)
- QA-005 (#174) init: `fe32e7c` (PR #246, merged)
- QA-006 (#175) status: `8c0886a` (PR #247, merged)
- QA-007 (#176) connect: `0e0caa2` (PR #248, merged)
- QA-008 (#177) discover — this PR
