---
id: "HOST-002"
type: spec
status: proposed
created: "2026-06-19"
tags: [spec, host, config, precedence, ci, windows, bugfix]
issue: 193
---

# HOST-002: Tasks

## This PR

- [x] **Fix BUG-A:** add `NoSleepSet`/`VerboseSet` to `host.Flags`; guard
      `cfg.NoSleep`/`cfg.Verbose` assignments in `applyFlags`.
- [x] **Wire it:** populate `*Set` from `cmd.Flags().Changed(...)` in
      `runHostSetup` and `runHostCheck`.
- [x] **Test BUG-A:** `TestMerge_BoolFlagPrecedence` (unset flag keeps env;
      explicit flag overrides env); update `TestMerge_FlagsOverrideDefaults` and
      `TestMerge_VerboseFlag` for the new `*Set` requirement.
- [x] **Fix I3:** report `result.RDPPort` in `printSetupSummary` /
      `printSetupJSON`; warn on Windows when an explicit `--port` differs from
      the actual port. (`printSetupJSON` no longer needs `cfg`.)
- [x] **Un-skip** `TestWriteHostEnv_FilePermissions` on non-Windows.
- [x] **Add** `test-windows` CI job (build + vet + test on `windows-latest`).
- [x] **Strengthen** `smoke.ps1` to assert `host --help` lists `init`.
- [x] **Fix flake exposed by the Windows job:** `TestStartServer`
      (`internal/health`) slept a fixed 50ms then made one request; on Windows
      the listener wasn't accepting yet. Poll until ready with a 3s deadline.

## Verification

- [ ] `go build ./...`, `go vet ./...`, gofmt clean (verified locally on Linux
      ending; CRLF is a Windows checkout artifact).
- [ ] `go test ./internal/host/... ./cmd/cli/...` green, including the new
      precedence test and the now-running permissions test.
- [ ] CI: `test`, `test-windows`, `lint`, `security`, `build-matrix` all green.

## Deferred (not this PR)

- Code smells: dead `host.LoadConfig()`, triplicated `tailscaleIPImpl()`, magic
  `3389`, `--verbose` shadowing, Unicode ✓/✗ — own cleanup PR.
- I5 `.env` rewriter hardening; I6 deterministic Windows elevation check.
- QA-004 smoke-suite implementation — tracked under #78 / PR #191.

## Decisions

- `*Set` companion fields for bools only; non-bool flags keep their natural
  sentinels (`0`, `""`).
- Report the configured port (`result.RDPPort`), not the requested one — correct
  on both platforms and makes Windows `--port` non-effectiveness explicit.
- Windows CI job omits `-race` (needs a C toolchain on Windows; Linux covers it).
