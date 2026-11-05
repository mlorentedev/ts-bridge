#!/usr/bin/env bash
# Tests for report() in scripts/e2e/run.sh, the verdict of the local e2e harness (#606, AC5).
#
# The harness itself needs VMs and does not run in CI; its verdict does not. A known bug
# marked XFAIL must not fail the run, and its fix must: XPASS fails until the marker goes, so a
# fixed bug cannot hide behind a stale XFAIL. `clean` and `xfail-only` are the controls that
# stop a report which fails everything.
#
# Run: bash scripts/tests/test-e2e-report.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# shellcheck source-path=SCRIPTDIR source=../e2e/run.sh
. "$ROOT/scripts/e2e/run.sh"
OS=fixture # read by report()
export OS

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
fails=0

check() { # check <case> <want: pass|fail> <log lines...>
  local name="$1" want="$2" got=pass
  shift 2
  printf '%s\n' "$@" >"$WORK/$name.log"
  report "$WORK/$name.log" >/dev/null || got=fail
  if [ "$got" = "$want" ]; then echo "ok   $name"; else echo "FAIL $name: want $want, got $got"; fails=$((fails + 1)); fi
}

check clean       pass 'PASS a' 'PASS b'
check xfail-only  pass 'PASS a' 'XFAIL #1 b'
check one-fail    fail 'PASS a' 'FAIL b'
check one-xpass   fail 'PASS a' 'XPASS #1 b'
check noise-only  pass 'some guest output' 'FAILED to resolve' 'PASSWORD prompt'
check fail-word-mid pass 'PASS a mentions FAIL inside'

[ "$fails" -eq 0 ] || { echo "test-e2e-report: $fails case(s) failed"; exit 1; }
echo "test-e2e-report: all cases passed"
