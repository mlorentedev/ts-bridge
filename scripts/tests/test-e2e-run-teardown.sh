#!/usr/bin/env bash
# shellcheck disable=SC2016 # shim bodies are written verbatim and expand when the shim runs
# Tests the EXIT trap of `run.sh linux`, the path `make e2e-linux` takes (QA-606 AC3, #702).
#
# test-e2e-mesh-preflight.sh covers the mesh run's teardown; this covers main()'s. Each case runs
# the real `run.sh linux` against shims for go, gh and dotf and judges what is left running: the
# guest session goes whether the scenario passes, fails or is interrupted, a teardown that fails
# turns a passing run red, a leftover session is deleted before the boot, and the log names its
# candidate. `pass-run-exits-zero` is the control that stops a trap which fails
# everything.
#
# Run: bash scripts/tests/test-e2e-run-teardown.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RUN="$ROOT/scripts/e2e/run.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/state"
export PATH="$WORK/bin:$PATH" WORK E2E_OUT="$WORK/out" E2E_PREV=v0.0.0
fails=0

shim() { printf '#!/usr/bin/env bash\n%s\n' "$2" >"$WORK/bin/$1"; chmod +x "$WORK/bin/$1"; }
shim go 'while [ $# -gt 0 ]; do [ "$1" = -o ] && : >"$2"; shift; done'
# gh release download writes the asset and a checksums.txt that names it, in the current dir.
shim gh 'a=ts-bridge-v0.0.0-linux-arm64.tar.gz
mkdir -p ts-bridge-linux-arm64 && : >ts-bridge-linux-arm64/ts-bridge
tar czf "$a" ts-bridge-linux-arm64 && shasum -a 256 "$a" >checksums.txt'
shim dotf 'echo "$*" >>"$WORK/dotf.log"
case "$*" in
  *--help*) exit 0 ;;
  "vm linux status"*) [ ! -f "$WORK/state/vm" ] || echo "tsb-e2e-linux running" ;;
  "vm linux up"*) touch "$WORK/state/vm" ;;
  "vm linux down"*) [ ! -f "$WORK/down-fails" ] || exit 1; rm -f "$WORK/state/vm" ;;
  *--script*) [ ! -f "$WORK/hang" ] || { echo hanging >"$WORK/hanging"; sleep 30; }
    cat "$WORK/verdict" ;;
esac'

reset() { # reset <scenario result line>
  rm -rf "$WORK/state" "$WORK/hang" "$WORK/hanging" "$WORK/down-fails" "$WORK/dotf.log" "$WORK/out"
  mkdir "$WORK/state"
  touch "$WORK/dotf.log"
  echo "$1" >"$WORK/verdict"
}
linux() { bash "$RUN" linux >"$WORK/out.txt" 2>&1; }
check() { # check <case> <command...>
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then echo "ok   $name"; else echo "FAIL $name: $*"; fails=$((fails + 1)); fi
}
guest_gone() { [ ! -f "$WORK/state/vm" ] && grep -q 'vm linux down' "$WORK/dotf.log"; }

reset 'PASS shim case'
rc=0
linux || rc=$?
check pass-run-exits-zero test "$rc" -eq 0
check pass-run-booted-the-guest grep -q 'vm linux up --session' "$WORK/dotf.log"
check pass-run-tore-down-the-guest guest_gone
# The log is the run's evidence: it must name the build it describes (QA-606 round-2 review).
# reset clears the logs, so this run's is the only one.
check pass-run-log-names-its-candidate grep -q "^e2e linux: candidate $(git -C "$ROOT" rev-parse --short HEAD), previous v0.0.0\$" "$WORK"/out/linux-*.log

# A session left by an earlier run is deleted before the new one boots, not reused.
reset 'PASS shim case'
touch "$WORK/state/vm"
rc=0
linux || rc=$?
check leftover-session-deleted-before-boot awk '/vm linux down/ && !d { d = NR } /vm linux up --session/ && !u { u = NR } END { exit !(d && u && d < u) }' "$WORK/dotf.log"

reset 'FAIL shim case'
rc=0
linux || rc=$?
check fail-run-exits-nonzero test "$rc" -ne 0
check fail-run-tore-down-the-guest guest_gone

# A guest left running must not read as a green run, even when every case passed.
reset 'PASS shim case'
touch "$WORK/down-fails"
rc=0
linux || rc=$?
# The scenario ran and passed, so the failure can only be the trap's teardown.
check failed-teardown-ran-the-scenario grep -q '^PASS shim case' "$WORK"/out/linux-*.log
check failed-teardown-exits-nonzero test "$rc" -ne 0
check failed-teardown-says-why grep -q 'could not delete session tsb-e2e-linux' "$WORK/out.txt"

# An interrupt while the guest scenario runs: the trap, not a later clean, must remove it.
reset 'PASS shim case'
touch "$WORK/hang"
set -m
bash "$RUN" linux >"$WORK/out.txt" 2>&1 &
pid=$!
for _ in $(seq 1 100); do [ -f "$WORK/hanging" ] && break; sleep 0.2; done
check interrupt-reached-the-guest-scenario test -f "$WORK/hanging"
kill -INT -- "-$pid" 2>/dev/null || kill "$pid" 2>/dev/null || true
wait "$pid" || true
set +m
check interrupt-tore-down-the-guest guest_gone

[ "$fails" -eq 0 ] || { echo "test-e2e-run-teardown: $fails case(s) failed"; exit 1; }
echo "test-e2e-run-teardown: all cases passed"
