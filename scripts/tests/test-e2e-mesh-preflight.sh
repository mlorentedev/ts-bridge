#!/usr/bin/env bash
# shellcheck disable=SC2016 # shim bodies are written verbatim and expand when the shim runs
# Tests that `run.sh mesh` calls its preflight and tears down after it (QA-607, #607, AC2, AC9).
#
# hs_premise_ok and hs_guest_reaches are unit-tested in test-e2e-headscale.sh; that cannot show
# the run calls them. Each case here runs the real `run.sh mesh linux` against shims for docker,
# dotf, route, ipconfig and go, and judges the effects: the exit status, the check's own message,
# whether any guest scenario started, and what is left running afterwards. `scenario-runs` is the
# control that stops a preflight which refuses everything. The pre-auth key travels on the guest
# script's stdin, so the argument logs must not hold it.
#
# Run: bash scripts/tests/test-e2e-mesh-preflight.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
RUN="$ROOT/scripts/e2e/run.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/state"
export PATH="$WORK/bin:$PATH" WORK E2E_OUT="$WORK/out" HS_DIR="$WORK/out/headscale"
fails=0

shim() { printf '#!/usr/bin/env bash\n%s\n' "$2" >"$WORK/bin/$1"; chmod +x "$WORK/bin/$1"; }
shim route 'printf "  interface: en7\n"'
shim ipconfig 'echo 10.9.8.7'
shim go 'while [ $# -gt 0 ]; do [ "$1" = -o ] && : >"$2"; shift; done'
shim docker 'echo "$*" >>"$WORK/docker.log"
case "$1" in
  run) touch "$WORK/state/hs" ;;
  rm) rm -f "$WORK/state/hs" ;;
  ps) [ ! -f "$WORK/state/hs" ] || echo cafe01 ;;
  exec) case "$*" in *"users list"*) echo "[{\"id\": 1}]" ;; *preauthkeys*) echo "{\"key\": \"hskey-auth-FAKE\"}" ;; esac ;;
esac'
shim dotf 'echo "$*" >>"$WORK/dotf.log"
case "$*" in
  *--help*) exit 0 ;;
  "vm linux status"*) [ ! -f "$WORK/state/vm" ] || echo "tsb-e2e-linux running" ;;
  "vm linux up"*) touch "$WORK/state/vm" ;;
  "vm linux down"*) rm -f "$WORK/state/vm" ;;
  *"curl "*) [ ! -f "$WORK/blocked" ] || exit 1 ;;
  *--script*) cat >"$WORK/stdin"; [ ! -f "$WORK/hang" ] || { echo hanging >"$WORK/hanging"; sleep 30; }; echo "PASS mesh enrol: shim" ;;
esac'

mkdir -p "$WORK/mod-ok/derp/derphttp" "$WORK/mod-gone/derp/derphttp"
echo 'x = envknob.RegisterBool("TS_DEBUG_USE_DERP_HTTP")' >"$WORK/mod-ok/derp/derphttp/derphttp_client.go"
echo 'x = 1' >"$WORK/mod-gone/derp/derphttp/derphttp_client.go"

reset() { rm -rf "$WORK/state" "$WORK/blocked" "$WORK/hang" "$WORK/hanging" "$WORK/stdin" "$WORK/docker.log" "$WORK/dotf.log"; mkdir "$WORK/state"; touch "$WORK/docker.log" "$WORK/dotf.log"; }
mesh() { E2E_TS_MODDIR="${MOD:-$WORK/mod-ok}" bash "$RUN" mesh "$@" >"$WORK/out.txt" 2>&1; }
check() { # check <case> <command...>
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then echo "ok   $name"; else echo "FAIL $name: $*"; fails=$((fails + 1)); fi
}
said() { grep -q -- "$1" "$WORK/out.txt"; }
never() { ! grep -q -- "$2" "$WORK/$1.log"; }
nothing_left() { [ ! -f "$WORK/state/hs" ] && [ ! -f "$WORK/state/vm" ]; }

reset
rc=0
MOD="$WORK/mod-gone" mesh linux || rc=$?
check premise-exits-nonzero test "${rc:-0}" -ne 0
check premise-says-why said 'TS_DEBUG_USE_DERP_HTTP'
check premise-starts-no-headscale never docker '^run '
check premise-boots-no-guest never dotf 'vm linux up --session'

reset
touch "$WORK/blocked"
mesh linux || rc=$?
check guest-exits-nonzero test "${rc:-0}" -ne 0
check guest-says-why said 'guest linux cannot reach the control plane at http://10.9.8.7:8080/health'
check guest-runs-no-scenario never dotf '--script'
check guest-run-leaves-nothing nothing_left

reset
mesh windows || rc=$?
check windows-refused test "${rc:-0}" -ne 0
check windows-starts-nothing never docker '^run '

reset
rc=0
mesh linux || rc=$?
check scenario-runs grep -q -- '--script' "$WORK/dotf.log"
check scenario-key-on-stdin grep -q '^hskey-' "$WORK/stdin"
check scenario-key-not-in-argv never dotf 'hskey'
check scenario-leaves-nothing nothing_left
check scenario-log-written test -n "$(ls "$WORK"/out/mesh-linux-*.log 2>/dev/null)"

# An interrupt while the guest scenario runs: the trap, not a later clean, must remove both.
reset
touch "$WORK/hang"
set -m
E2E_TS_MODDIR="$WORK/mod-ok" bash "$RUN" mesh linux >"$WORK/out.txt" 2>&1 &
pid=$!
for _ in $(seq 1 100); do [ -f "$WORK/hanging" ] && break; sleep 0.2; done
check interrupt-reached-the-guest-scenario test -f "$WORK/hanging"
kill -INT -- "-$pid" 2>/dev/null || kill "$pid" 2>/dev/null || true
wait "$pid" || true
set +m
check interrupt-leaves-nothing nothing_left
check interrupt-tore-down-the-guest grep -q 'vm linux down' "$WORK/dotf.log"

[ "$fails" -eq 0 ] || { echo "test-e2e-mesh-preflight: $fails case(s) failed"; exit 1; }
echo "test-e2e-mesh-preflight: all cases passed"
