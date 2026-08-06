#!/usr/bin/env bash
# Shared BATS helpers for the ts-bridge smoke test suite (QA-004..QA-013).
#
# No external dependencies (no bats-assert / bats-support): the project has a
# zero-dependency design goal, so the handful of assertions we need are defined
# here against the `$status` / `$output` variables that BATS `run` populates.

# repo_root prints the absolute path to the repository root.
# The suite lives at scripts/tests/*.bats, so the root is two levels up from
# $BATS_TEST_DIRNAME. Run in a subshell so the caller's CWD is never mutated.
repo_root() {
  ( cd "${BATS_TEST_DIRNAME}/../.." && pwd )
}

# resolve_bin prints the path to the ts-bridge binary to exercise.
# Resolution order:
#   1. $BIN, if set (CI builds once and passes it — no `go` in the test path).
#   2. A prebuilt ts-bridge / ts-bridge.exe at the repo root (local dev).
#   3. Build it as a last resort (single `go build`; build noise goes to stderr
#      so stdout stays a clean path for command substitution).
resolve_bin() {
  if [ -n "${BIN:-}" ]; then
    printf '%s\n' "${BIN}"
    return 0
  fi

  local root candidate
  root="$(repo_root)"
  for candidate in "${root}/ts-bridge" "${root}/ts-bridge.exe"; do
    if [ -f "${candidate}" ]; then
      printf '%s\n' "${candidate}"
      return 0
    fi
  done

  local out="${root}/ts-bridge"
  [ "${OS:-}" = "Windows_NT" ] && out="${root}/ts-bridge.exe"
  ( cd "${root}" && go build -o "${out}" ./cmd/ts-bridge/ ) >&2 || return 1
  printf '%s\n' "${out}"
}

# assert_success fails the test if the last `run` exited non-zero.
assert_success() {
  if [ "${status}" -ne 0 ]; then
    printf 'expected exit 0, got %s\n--- output ---\n%s\n' "${status}" "${output}" >&2
    return 1
  fi
}

# assert_failure fails the test if the last `run` exited zero.
assert_failure() {
  if [ "${status}" -eq 0 ]; then
    printf 'expected non-zero exit, got 0\n--- output ---\n%s\n' "${output}" >&2
    return 1
  fi
}

# assert_contains fails the test if $output does not contain the given substring.
assert_contains() {
  if [[ "${output}" != *"$1"* ]]; then
    printf 'expected output to contain: %s\n--- output ---\n%s\n' "$1" "${output}" >&2
    return 1
  fi
}

# assert_json_object fails the test unless $output is exactly one JSON object.
#
# "Exactly one" is the load-bearing word. `jq -e .` is NOT enough: jq accepts a
# stream of concatenated values, so '{"a":1}{"b":2}' parses happily. That is the
# precise shape #254 produced under --log-format json — a JSON log line followed
# by the JSON payload — so a plain `jq -e .` guard would have passed on the very
# bug it exists to catch. `jq -se` slurps the stream into an array, letting us
# assert the array holds one element and that it is an object.
#
# The structural check runs first so the assertion still has teeth where jq is
# absent, though it is strictly weaker: it pins the first and last characters
# and so catches a log line printed before or after the payload, but cannot
# detect two objects concatenated between them.
assert_json_object() {
  local first last
  first="$(printf '%s' "${output}" | head -c 1)"
  last="$(printf '%s' "${output}" | tail -c 1)"

  if [ "${first}" != "{" ] || [ "${last}" != "}" ]; then
    printf 'expected stdout to be exactly one JSON object\n--- output ---\n%s\n' "${output}" >&2
    return 1
  fi

  if command -v jq >/dev/null 2>&1; then
    if ! printf '%s' "${output}" | jq -se 'length == 1 and (.[0] | type == "object")' >/dev/null 2>&1; then
      printf 'expected stdout to be exactly one JSON object (jq)\n--- output ---\n%s\n' "${output}" >&2
      return 1
    fi
  fi
}

# refute_contains fails the test if $output DOES contain the given substring.
refute_contains() {
  if [[ "${output}" == *"$1"* ]]; then
    printf 'expected output NOT to contain: %s\n--- output ---\n%s\n' "$1" "${output}" >&2
    return 1
  fi
}
