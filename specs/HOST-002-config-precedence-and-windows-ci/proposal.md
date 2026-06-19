---
id: "HOST-002"
type: spec
status: proposed
created: "2026-06-19"
tags: [spec, host, config, precedence, ci, windows, bugfix]
issue: 193
---

# HOST-002: Host config precedence + Windows CI — Proposal

## Problem

An audit of the merged HOST-001 work (#187) surfaced two live bugs in `master`
plus the structural CI gap that let the original Windows build break (C1) reach
CI undetected.

1. **Bool-flag precedence (BUG-A).** `applyFlags` assigned `cfg.NoSleep =
   flags.NoSleep` and `cfg.Verbose = flags.Verbose` unconditionally. Cobra
   defaults a bool flag to `false`, so an *unset* `--no-sleep` overwrites a real
   `TS_HOST_NO_SLEEP=true` from the environment — breaking the documented
   `flags > env > defaults` chain. CodeRabbit flagged this as Major on #187; it
   shipped unfixed.
2. **Windows port reporting (I3).** On Windows the RDP listening port comes from
   the registry; the firewall opens that port, but the setup summary and
   `--json` output printed `cfg.Port` (the *requested* value). `--port 4000`
   was reported even though the firewall opened the registry port.
3. **No native Windows CI runner.** The Linux `test` job never compiles
   `//go:build windows` files, so a Windows-only break passes every check.

## Goal

Close the precedence bug, make the reported port truthful, and add a Windows
runner so this class of regression is caught automatically.

## Scope

### In scope

- `internal/host/config.go` — add `NoSleepSet`/`VerboseSet` to `Flags`; guard
  the bool assignments in `applyFlags`.
- `cmd/cli/host.go` — populate the `*Set` fields from `cmd.Flags().Changed(...)`
  in `runHostSetup` and `runHostCheck`; report `result.RDPPort` in the summary
  and JSON; warn on Windows when an explicit `--port` differs from the actual
  port.
- `internal/host/config_test.go` — update the two affected `Merge` tests; add a
  precedence regression test (`TestMerge_BoolFlagPrecedence`).
- `cmd/cli/host_test.go` — un-skip `TestWriteHostEnv_FilePermissions` on
  non-Windows.
- `.github/workflows/ci.yml` — add a `test-windows` job (build + vet + test).
- `scripts/tests/smoke.ps1` — assert `host --help` lists `init`.

### Out of scope (deferred, separate cleanup)

- Pure code smells: dead `host.LoadConfig()`, triplicated `tailscaleIPImpl()`,
  magic `3389` literals, `--verbose` flag shadowing, Unicode ✓/✗ on Windows.
- I5 `.env` rewriter hardening (overwrite protection) and I6 elevation check.
- QA-004 smoke-suite implementation (`smoke.bats`, helpers, fixtures, coverage
  matrix) — tracked under #78 / PR #191.

## Design notes

- **Why `*Set` for bools only.** A bool's zero value (`false`) is a valid,
  indistinguishable user choice, so it needs an explicit "was it passed" signal.
  `Port` already uses `0` as a sentinel (`if flags.Port > 0`) and
  `FirewallRule`/`LogFormat` use `""`, so they need no companion field.
- **Why report `result.RDPPort`.** `SetupResult.RDPPort` is the port that was
  actually configured (registry on Windows, `cfg.Port`/default on Linux).
  Reporting it is correct on both platforms; the Windows warning makes an
  ineffective `--port` explicit rather than silent.
- **Why a separate Windows job, not `-race`.** The race detector needs a C
  toolchain on Windows; the Linux `test` job already runs `-race`. The Windows
  job's value is compiling the build-tagged files and running the suite.

## Acceptance Criteria

- [ ] Unset `--no-sleep`/`--verbose` do not override env; explicit flags win
      (proven by `TestMerge_BoolFlagPrecedence`).
- [ ] Setup summary and `--json` report the actually-configured RDP port; an
      explicit Windows `--port` mismatch warns.
- [ ] `test-windows` runs build + vet + test on `windows-latest` in CI.
- [ ] `TestWriteHostEnv_FilePermissions` runs (not skipped) on Linux CI.
- [ ] `go build`, `go vet`, gofmt, and golangci-lint are clean; `go.mod`/`go.sum`
      unchanged.

## Non-functional

- No new module dependencies.
- Behavior change is limited to honoring the documented precedence chain and to
  output accuracy — no change to what setup actually configures.
