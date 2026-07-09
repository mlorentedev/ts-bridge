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
