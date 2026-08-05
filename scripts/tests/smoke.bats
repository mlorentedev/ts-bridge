#!/usr/bin/env bats
#
# ts-bridge CLI smoke tests (POSIX / BATS).
#
# This is the shared smoke suite for the QA-004..QA-013 sequence: each QA
# ticket appends its own section below, keeping one file as the single source
# of CLI coverage on Linux/macOS. The PowerShell mirror lives at smoke.ps1.
#
# This section (QA-004, issue #173) covers CLI PARSING only — the surface that
# every other command depends on:
#   * version / --version / --short
#   * help discovery (--help, -h, help, no-args)
#   * per-subcommand --help reachability
#   * global flags
#   * unknown-command / unknown-flag error handling
#
# Per-command behaviour (init writes files, status probes a socket, connect
# dials, discover queries the API, host mutates the machine) is intentionally
# NOT tested here — those belong to QA-005..QA-013 and get their own sections.

load helpers/smoke_helpers

# Resolve the binary once for the whole file. Exported vars set in setup_file
# are visible to every test in the file.
setup_file() {
  BIN="$(resolve_bin)" || {
    echo "could not resolve ts-bridge binary (set \$BIN or build ./cmd/ts-bridge)" >&2
    return 1
  }
  export BIN
}

# Each test runs in its own throwaway directory. Commands like `init` write
# config files into the CWD, and BATS_TEST_TMPDIR is unique per test and
# removed afterwards — so tests stay idempotent and never leak state into the
# repo or into each other. Harmless for the parsing tests, which ignore CWD.
setup() {
  cd "${BATS_TEST_TMPDIR}" || return 1
  # connect/discover resolve target, auth key, tailnet and control plane from
  # these env vars. Clear them so a value inherited from the caller's
  # environment can never let a test satisfy validation and reach a
  # long-running or network path (the bridge runner, or the tailnet API).
  # Harmless for the other commands, which take their inputs from flags.
  unset TS_TARGET TS_AUTHKEY TS_TAILNET TS_CONTROL_URL TS_HEADSCALE_API_KEY
}

# ---------------------------------------------------------------------------
# QA-004 (#173) — CLI parsing
# ---------------------------------------------------------------------------

@test "version: prints name and commit" {
  run "${BIN}" version
  assert_success
  assert_contains "ts-bridge"
}

@test "version --short: prints only the version token" {
  run "${BIN}" version --short
  assert_success
  [ -n "${output}" ]
  # The short form is Version() alone; the long form adds "(commit ...)".
  refute_contains "commit"
}

@test "--version flag: mirrors the version subcommand" {
  run "${BIN}" --version
  assert_success
  assert_contains "ts-bridge"
}

@test "--help: lists every top-level subcommand" {
  run "${BIN}" --help
  assert_success
  assert_contains "connect"
  assert_contains "discover"
  assert_contains "host"
  assert_contains "import"
  assert_contains "init"
  assert_contains "status"
  assert_contains "version"
}

@test "-h: short help flag works" {
  run "${BIN}" -h
  assert_success
  assert_contains "Usage:"
}

@test "help subcommand: equivalent to --help" {
  run "${BIN}" help
  assert_success
  assert_contains "Available Commands:"
  assert_contains "connect"
}

@test "no args: falls back to help, exits 0" {
  run "${BIN}"
  assert_success
  assert_contains "Usage:"
}

@test "-v (verbose) without a subcommand: prints help, not a version" {
  # Guard against the smoke.ps1 misconception that -v is a version flag:
  # -v is the global --verbose flag; with no subcommand cobra shows help.
  run "${BIN}" -v
  assert_success
  assert_contains "Usage:"
}

@test "connect --help: reachable, documents --target" {
  run "${BIN}" connect --help
  assert_success
  assert_contains "--target"
}

@test "init --help: reachable, documents --target" {
  run "${BIN}" init --help
  assert_success
  assert_contains "--target"
}

@test "status --help: reachable" {
  run "${BIN}" status --help
  assert_success
  assert_contains "Usage:"
}

@test "discover --help: reachable, documents --json" {
  run "${BIN}" discover --help
  assert_success
  assert_contains "--json"
}

@test "import --help: reachable, documents the descriptor argument" {
  run "${BIN}" import --help
  assert_success
  assert_contains "descriptor"
}

@test "version --help: reachable, documents --short" {
  run "${BIN}" version --help
  assert_success
  assert_contains "--short"
}

@test "host --help: lists its subcommands" {
  run "${BIN}" host --help
  assert_success
  assert_contains "setup"
  assert_contains "check"
}

@test "host setup --help: reachable" {
  run "${BIN}" host setup --help
  assert_success
  assert_contains "Usage:"
}

@test "host check --help: reachable" {
  run "${BIN}" host check --help
  assert_success
  assert_contains "Usage:"
}

@test "unknown command: exits non-zero with a clear message" {
  run "${BIN}" definitely-not-a-command
  assert_failure
  assert_contains "unknown command"
}

@test "unknown flag: exits non-zero with a clear message" {
  run "${BIN}" version --definitely-not-a-flag
  assert_failure
  assert_contains "unknown flag"
}

# ---------------------------------------------------------------------------
# QA-005 (#174) — init
# ---------------------------------------------------------------------------
# Non-interactive mode requires BOTH --auth-key and --target; with only one it
# falls back to the interactive wizard. init writes files into the CWD, which
# setup() has pointed at a fresh per-test tmp dir.

AUTH_KEY="tskey-auth-test-dummy"
TARGET="100.64.0.1:3389"

@test "init: env format (default) writes .env with auth key and target" {
  run "${BIN}" init --auth-key "${AUTH_KEY}" --target "${TARGET}"
  assert_success
  assert_contains "written successfully"
  [ -f .env ]
  run cat .env
  assert_contains "TS_AUTHKEY=${AUTH_KEY}"
  assert_contains "TS_TARGET=${TARGET}"
}

@test "init: yaml format writes ts-bridge.yaml and keeps the auth key OUT of it" {
  run "${BIN}" init --auth-key "${AUTH_KEY}" --target "${TARGET}" --format yaml
  assert_success
  [ -f ts-bridge.yaml ]
  [ -f .env ]
  # Security invariant: the auth key must never be written to the YAML file...
  run cat ts-bridge.yaml
  assert_contains "target: ${TARGET}"
  refute_contains "${AUTH_KEY}"
  # ...it goes to .env instead.
  run cat .env
  assert_contains "TS_AUTHKEY=${AUTH_KEY}"
}

@test "init: --config writes to a custom output path" {
  run "${BIN}" init --auth-key "${AUTH_KEY}" --target "${TARGET}" --config custom.env
  assert_success
  [ -f custom.env ]
}

@test "init: refuses to overwrite an existing config without --force" {
  run "${BIN}" init --auth-key "${AUTH_KEY}" --target "${TARGET}"
  assert_success
  # A second run must fail loudly rather than clobber the existing file.
  run "${BIN}" init --auth-key "${AUTH_KEY}" --target "${TARGET}"
  assert_failure
  assert_contains "already exists"
}

@test "init: --force overwrites an existing config" {
  run "${BIN}" init --auth-key "${AUTH_KEY}" --target "${TARGET}"
  assert_success
  run "${BIN}" init --auth-key "${AUTH_KEY}" --target "${TARGET}" --force
  assert_success
  assert_contains "written successfully"
}

@test "init: missing a required flag with no TTY fails fast (does not hang)" {
  # Only --auth-key → falls back to the interactive wizard. With stdin closed
  # (as in CI) the wizard must error out, never block the run.
  run "${BIN}" init --auth-key "${AUTH_KEY}" </dev/null
  assert_failure
}

# ---------------------------------------------------------------------------
# QA-006 (#175) — status
# ---------------------------------------------------------------------------
# No live bridge runs during smoke tests, so these cover the not-running path
# and the flag surface. The RUNNING summary and the metrics JSON require a live
# health server — covered by the Go unit tests (status_test.go) and the QA-013
# e2e run, not duplicated here.
#
# --watch is a signal-terminated infinite loop. It is deliberately NOT executed
# here: a stop signal that fails to reach the process (observed on native
# Windows) would hang the run, and a hung BATS test would stall CI until the
# job's global timeout. Its flags are verified via `status --help`, and
# runWatchLoop has unit coverage via an injected signal channel.

@test "status: reports 'not running' when no bridge is up (exit 0)" {
  run "${BIN}" status
  assert_success
  assert_contains "Bridge not running"
}

@test "status --json: no bridge, reports not-running with no metrics JSON" {
  # The health check fails before the JSON branch, so --json degrades to the
  # same not-running message rather than printing an empty/garbage object.
  run "${BIN}" status --json
  assert_success
  assert_contains "Bridge not running"
  refute_contains "active_connections"
}

@test "status --addr: the queried address is reflected in the message" {
  run "${BIN}" status --addr 127.0.0.1:9999
  assert_success
  assert_contains "127.0.0.1:9999"
}

@test "status --help: documents the --watch and --interval flags" {
  run "${BIN}" status --help
  assert_success
  assert_contains "--watch"
  assert_contains "--interval"
}

# ---------------------------------------------------------------------------
# QA-007 (#176) — connect
# ---------------------------------------------------------------------------
# Once config validates, connect starts the bridge (tsnet init) — a
# long-running, network-touching operation. So these tests only exercise paths
# that FAIL during flag parsing or config validation, before the bridge is ever
# started. TS_TARGET/TS_AUTHKEY are cleared in setup(), and invalid inputs use a
# bad *format* (rejected by config.Merge, never sent to the control plane), so
# no test can reach the runner and hang. Actually starting the bridge and its
# graceful shutdown are covered by main_integration_test.go and the QA-013 e2e
# run — see docs/qa-coverage.md.

@test "connect: with no target configured, fails asking for one" {
  run "${BIN}" connect
  assert_failure
  assert_contains "target invalid format"
}

@test "connect: with a target but no auth key, fails asking for one" {
  run "${BIN}" connect --target 100.64.0.1:3389
  assert_failure
  assert_contains "auth key is required"
}

@test "connect: rejects a malformed target before starting" {
  run "${BIN}" connect --target no-port-here --auth-key tskey-x
  assert_failure
  assert_contains "target invalid format"
  assert_contains "no-port-here"
}

@test "connect: rejects a malformed auth key before starting" {
  run "${BIN}" connect --auth-key notavalidkey --target 100.64.0.1:3389
  assert_failure
  assert_contains "must start with tskey- or hskey-"
}

@test "connect: --auth-key-file pointing at a missing file fails fast" {
  run "${BIN}" connect --auth-key-file "${BATS_TEST_TMPDIR}/nope" --target 100.64.0.1:3389
  assert_failure
  assert_contains "read auth key file"
}

@test "connect: an invalid flag value is a parse error" {
  run "${BIN}" connect --timeout not-a-duration
  assert_failure
  assert_contains "invalid argument"
}

@test "connect: --auth-key warns that it is visible in the process list" {
  # Security nudge toward --auth-key-file. No target, so it still fails before
  # the bridge starts — but the warning must be emitted first.
  run "${BIN}" connect --auth-key tskey-dummy
  assert_contains "visible in the process list"
}

# ---------------------------------------------------------------------------
# QA-008 (#177) — discover
# ---------------------------------------------------------------------------
# discover queries the tailnet API, and its default mode is interactive (it
# prints a host list and prompts). Both are reached only AFTER a successful
# device fetch, which needs a valid auth key + tailnet + network. setup()
# clears the relevant env vars and these tests never supply both an auth key
# and a tailnet, so every one fails during flag validation or the pre-fetch
# required-input checks — no network call, no prompt. Filter/auto selection
# against a live tailnet is covered by the QA-013 e2e run (see qa-coverage.md).

@test "discover: with no auth key, fails asking for one" {
  run "${BIN}" discover
  assert_failure
  assert_contains "auth key is required"
}

@test "discover --json: with no auth key, fails and emits no device JSON" {
  run "${BIN}" discover --json
  assert_failure
  assert_contains "auth key is required"
  refute_contains "hostname"
}

@test "discover: with an auth key but no tailnet, fails asking for one" {
  run "${BIN}" discover --auth-key tskey-x
  assert_failure
  assert_contains "tailnet name is required"
}

@test "discover --port: rejects an out-of-range port" {
  run "${BIN}" discover --port 99999
  assert_failure
  assert_contains "invalid port"
}

@test "discover --port: rejects a non-numeric port" {
  run "${BIN}" discover --port not-a-number
  assert_failure
  assert_contains "invalid argument"
}

@test "discover --help: documents the discovery flags" {
  run "${BIN}" discover --help
  assert_success
  assert_contains "--json"
  assert_contains "--filter"
  assert_contains "--auto"
  assert_contains "--tailnet"
}

# ---------------------------------------------------------------------------
# QA-009 (#178) — host setup
# ---------------------------------------------------------------------------
# host setup MUTATES the machine (Windows registry/firewall/UPnP/sleep; Linux
# xrdp/UFW/iptables) and requires admin/root, so the real setup path is NEVER
# exercised here. Only two things are safe to run:
#   * --help and flag-parse errors short-circuit inside Cobra before RunE, so
#     they never touch the system.
#   * The elevation guard — runHostSetup checks host.IsElevated() (euid == 0 on
#     Linux) and returns BEFORE any side effect when not elevated. The CI smoke
#     job runs as a non-root user, so exercising it only proves the refusal and
#     mutates nothing. The `id -u` skip is belt-and-suspenders: if the suite is
#     ever run as root, the destructive branch is never entered.
# Real setup behaviour (idempotency, per-step results, partial-failure summary)
# needs admin and is manual-only / QA-013 e2e — see docs/qa-coverage.md.

@test "host setup --help: documents the setup flags and all three platforms" {
  run "${BIN}" host setup --help
  assert_success
  assert_contains "--no-sleep"
  assert_contains "--firewall-rule"
  assert_contains "--port"
  # The Long help enumerates the per-platform behaviour.
  assert_contains "Windows"
  assert_contains "Linux"
  assert_contains "macOS"
}

@test "host setup: without root, refuses before making any system change" {
  # The only test that reaches runHostSetup. Safe because the elevation guard
  # returns before any side effect when euid != 0 (the CI runner is non-root).
  if [ "$(id -u 2>/dev/null || echo 0)" = "0" ]; then
    skip "running as root would enter the real (mutating) setup path"
  fi
  run "${BIN}" host setup
  assert_failure
  assert_contains "elevated privileges"
}

@test "host setup --no-sleep --firewall-rule: flags parse, then the guard refuses" {
  # Passing the flags and reaching the elevation guard (not an 'unknown flag'
  # error) proves they are registered and accepted, without running setup.
  if [ "$(id -u 2>/dev/null || echo 0)" = "0" ]; then
    skip "running as root would enter the real (mutating) setup path"
  fi
  run "${BIN}" host setup --no-sleep --firewall-rule "MyRDPRule"
  assert_failure
  assert_contains "elevated privileges"
  refute_contains "unknown flag"
}

@test "host setup --port: a non-numeric value is a parse error, never reaches setup" {
  run "${BIN}" host setup --port not-a-number
  assert_failure
  assert_contains "invalid argument"
}

# ---------------------------------------------------------------------------
# QA-010 (#179) — host check
# ---------------------------------------------------------------------------
# Unlike host setup, host check is READ-ONLY: it has no elevation guard and only
# reads state (Tailscale IP via `tailscale ip`, RDP port, firewall status). Every
# probe fails fast on the CI runner — tailscale/xrdp aren't installed, and
# ufw/iptables error without root — so check exits 0 and reports "not OK" without
# mutating or hanging. That makes its happy path safe to exercise directly here.

@test "host check --help: reachable, documents --json and read-only" {
  run "${BIN}" host check --help
  assert_success
  assert_contains "--json"
  assert_contains "Read-only"
}

@test "host check: read-only, runs without admin and prints the status block" {
  # No elevation guard: it exits 0 on the non-root CI runner (proving the
  # read-only/no-admin path) and prints every status label even when nothing is
  # installed (the 'Tailscale not installed' edge case reports rather than fails).
  run "${BIN}" host check
  assert_success
  assert_contains "HOST CHECK"
  assert_contains "Platform:"
  assert_contains "Tailscale:"
  assert_contains "RDP:"
  assert_contains "Firewall:"
}

# BATS `run` merges stdout and stderr into $output, so the stream-separation
# assertions below go through a subshell that redirects explicitly rather than
# relying on `run --separate-stderr` (a BATS >= 1.5 feature; CI installs the
# distro package, whose version is not pinned).

@test "host check --json: emits the documented JSON fields" {
  run bash -c '"${BIN}" host check --json 2>/dev/null'
  assert_success
  assert_contains "\"platform\""
  assert_contains "\"tailscale_ip\""
  assert_contains "\"rdp_port\""
  assert_contains "\"rdp_enabled\""
  assert_contains "\"firewall_ok\""
  assert_contains "\"tailscale_up\""
}

@test "host check --json: stdout is exactly one parseable JSON object" {
  # Regression guard for #254: the logger used to write to stdout ahead of the
  # payload, so `host check --json | jq .` failed on the interleaved line.
  run bash -c '"${BIN}" host check --json 2>/dev/null'
  assert_success
  assert_json_object
}

@test "host check --json --log-format json: stdout is still exactly one object" {
  # The nastiest variant of #254, and the reason assert_json_object slurps
  # rather than calling `jq -e .`: with a JSON-formatted logger on stdout the
  # output was two concatenated objects, which `jq -e .` accepts as a valid
  # stream. Only an "exactly one value" assertion catches it.
  run bash -c '"${BIN}" host check --json --log-format json 2>/dev/null'
  assert_success
  assert_json_object
}

@test "host check --json: the JSON payload does not leak onto stderr" {
  # `2>&1 >/dev/null` captures stderr only (order matters: stderr is duped to
  # the original stdout before stdout is redirected away).
  #
  # Only the negative is asserted. What the logger emits here is
  # platform-dependent — Linux logs an xrdp line, Windows logs "host check",
  # macOS's checkImpl logs nothing at all — so "stderr is non-empty" would not
  # be portable. That logs reach stderr rather than stdout is pinned
  # deterministically by TestHostInitLogger_WritesToStderrNotStdout.
  run bash -c '"${BIN}" host check --json 2>&1 >/dev/null'
  assert_success
  refute_contains "\"tailscale_ip\""
  refute_contains "\"rdp_port\""
}
