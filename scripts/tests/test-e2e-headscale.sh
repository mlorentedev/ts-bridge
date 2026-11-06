#!/usr/bin/env bash
# Tests for scripts/e2e/headscale.sh, the local Headscale the mesh scenarios run against
# (QA-607, #607).
#
# Each case judges what the consumer reads: the address Headscale advertises is the one
# every guest must reach, the config and policy are what the container loads, and the docker
# arguments are what decide whether a second run starts a second container or leaves state
# behind. `docker`, `route`, `ipconfig` and `dotf` are shims on PATH, so nothing here needs an
# engine or a VM.
#
# Run: bash scripts/tests/test-e2e-headscale.sh
# shellcheck disable=SC2016 # shim bodies are written verbatim and expand when the shim runs
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
HERE="$ROOT/scripts/e2e" # as run.sh defines it
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/bin" "$WORK/state"
export PATH="$WORK/bin:$PATH" WORK
export HS_DIR="$WORK/scripts/e2e/out/headscale"
export SHIM_LOG="$WORK/calls.log"
# shellcheck source-path=SCRIPTDIR source=../e2e/headscale.sh
. "$ROOT/scripts/e2e/headscale.sh"
fails=0

ok() { echo "ok   $1"; }
bad() { echo "FAIL $1: $2"; fails=$((fails + 1)); }
check() { # check <case> <command...>: passes when the command does
  local name="$1"
  shift
  if "$@" >/dev/null 2>&1; then ok "$name"; else bad "$name" "$*"; fi
}
refuses() { ! "$@"; }
logged() { grep -qE -- "$1" "$SHIM_LOG"; }
shim() { printf '#!/usr/bin/env bash\n%s\n' "$2" >"$WORK/bin/$1"; chmod +x "$WORK/bin/$1"; }

# route and ipconfig: the default interface is en7, whose address differs from en0's, so a
# function that guesses en0 returns the wrong one.
shim route 'if [ -f "$WORK/no-default" ]; then exit 1; fi; printf "   route to: default\n  interface: en7\n"'
shim ipconfig '[ "$2" = en7 ] && echo 10.9.8.7 || echo 10.0.0.1'

check host-ip-default-route test "$(hs_host_ip)" = 10.9.8.7
touch "$WORK/no-default"
msg="$(hs_host_ip 2>&1 || true)"
check host-ip-no-route-fails refuses hs_host_ip
check host-ip-no-route-names-cause grep -q 'no default route' <<<"$msg"
rm "$WORK/no-default"

cfg="$(hs_render_config 10.9.8.7)"
check config-server-url grep -qx 'server_url: http://10.9.8.7:8080' <<<"$cfg"
check config-derp-enabled grep -qx '    enabled: true' <<<"$cfg"
check config-stun grep -qE '^    stun_listen_addr: "?0\.0\.0\.0:3478' <<<"$cfg"
check config-no-derp-urls grep -qx '  urls: \[\]' <<<"$cfg"
check config-no-magic-dns grep -qx '  magic_dns: false' <<<"$cfg"
check config-no-tls refuses grep -q 'tls_' <<<"$cfg"
check config-no-placeholder refuses grep -q '@' <<<"$cfg"

# The policy is read by Headscale as HuJSON; it is written as plain JSON so Python parses it.
pol="$(python3 - "$ROOT/scripts/e2e/headscale/policy.hujson" <<'PY'
import json, sys
p = json.load(open(sys.argv[1]))
print(sorted(p["tagOwners"]))
print([(a["action"], a["src"], a["dst"]) for a in p["acls"]])
PY
)"
check policy-tag-owners grep -qx "\['tag:e2e-client', 'tag:e2e-host'\]" <<<"$pol"
check policy-one-acl grep -qx "\[('accept', \['tag:e2e-client'\], \['tag:e2e-host:22,3389'\])\]" <<<"$pol"

# The premise: the pinned tailscale.com still registers the knob the harness depends on.
mkdir -p "$WORK/mod-ok/derp/derphttp" "$WORK/mod-gone/derp/derphttp"
echo 'var useHTTP = envknob.RegisterBool("TS_DEBUG_USE_DERP_HTTP")' >"$WORK/mod-ok/derp/derphttp/derphttp_client.go"
echo 'var other = envknob.RegisterBool("TS_DEBUG_SOMETHING_ELSE")' >"$WORK/mod-gone/derp/derphttp/derphttp_client.go"
check premise-present hs_premise_ok "$WORK/mod-ok"
check premise-gone-fails refuses hs_premise_ok "$WORK/mod-gone"
check premise-gone-names-fallback grep -q 'TLS with a local CA' <<<"$(hs_premise_ok "$WORK/mod-gone" 2>&1 || true)"
check premise-no-dir-fails refuses hs_premise_ok "$WORK/missing"
check premise-real-module hs_premise_ok "$(cd "$ROOT" && go list -m -f '{{.Dir}}' tailscale.com)"

# docker keeps one bit of state: a container exists after `run` and not after `rm`.
shim docker 'echo "$*" >>"$SHIM_LOG"
case "$1" in
  run) touch "$WORK/state/up" ;;
  rm) rm -f "$WORK/state/up" ;;
  ps) [ ! -f "$WORK/state/up" ] || echo cafe01 ;;
  exec) case "$*" in *" health"*) [ ! -f "$WORK/unhealthy" ] || exit 1 ;; *"users create"*) [ ! -f "$WORK/no-user" ] || exit 1 ;; *"users list"*) echo "[{\"id\": 4, \"name\": \"e2e\"}]" ;; *preauthkeys*) echo "{\"key\": \"hskey-auth-FAKE\"}" ;; esac ;;
esac'
: >"$SHIM_LOG"
check up-first hs_up
check up-second hs_up
check up-one-container test "$(grep -c '^run ' "$SHIM_LOG")" = 1
check up-label logged '^run .*--label tsb-e2e=headscale'
published="$(grep '^run ' "$SHIM_LOG" | grep -oE -- '-p [^ ]+' | sort | tr '\n' ' ')"
check up-publishes-exactly-the-two-ports-on-all-interfaces test "$published" = "-p 3478:3478/udp -p 8080:8080 "
check up-mounts-only-out-dir test "$(grep '^run ' "$SHIM_LOG" | grep -oE -- '-v [^: ]+' | grep -cv -- "-v $HS_DIR/")" = 0
check up-mounts-something logged '^run .*-v '
check up-user-once test "$(grep -c 'users create e2e' "$SHIM_LOG")" = 1
check up-writes-config grep -qx 'server_url: http://10.9.8.7:8080' "$HS_DIR/conf/config.yaml"

# The host moved to another network: a running control plane still advertising the old address
# is replaced, not reported as up.
shim ipconfig '[ "$2" = en7 ] && echo 10.9.8.8 || echo 10.0.0.1'
: >"$SHIM_LOG"
check up-after-address-change hs_up
check up-after-address-change-recreates test "$(grep -c '^run ' "$SHIM_LOG")" = 1
check up-after-address-change-renders grep -qx 'server_url: http://10.9.8.8:8080' "$HS_DIR/conf/config.yaml"
shim ipconfig '[ "$2" = en7 ] && echo 10.9.8.7 || echo 10.0.0.1'

# A start that never gets healthy, or cannot create its user, must not leave a container the next
# hs_up would take for a working one.
HS_HEALTH_TRIES=1
for row in unhealthy:1 no-user:2; do # the user is created once in the first, twice in the second
  broken="${row%%:*}" creates="${row##*:}"
  hs_down >/dev/null 2>&1
  touch "$WORK/$broken"
  : >"$SHIM_LOG"
  check "$broken-fails" refuses hs_up
  check "$broken-leaves-no-container" test ! -f "$WORK/state/up"
  rm "$WORK/$broken"
  check "$broken-then-recovers" hs_up
  check "$broken-recovered-has-user" test "$(grep -c 'users create e2e' "$SHIM_LOG")" = "$creates"
done

# A stopped container left its database behind: the next start must not inherit it.
rm -f "$WORK/state/up"
echo stale >"$HS_DIR/data/db.sqlite"
: >"$SHIM_LOG"
check restart-after-stop hs_up
check restart-starts-a-container test "$(grep -c '^run ' "$SHIM_LOG")" = 1
check restart-drops-old-database test ! -e "$HS_DIR/data/db.sqlite"
check restart-creates-user-again test "$(grep -c 'users create e2e' "$SHIM_LOG")" = 1

check down-first hs_down
check down-second hs_down
check down-removes-container test ! -f "$WORK/state/up"
check down-removes-state test ! -e "$HS_DIR"

hs_up >/dev/null 2>&1
: >"$SHIM_LOG"
key="$(hs_mint_key tag:e2e-host)"
check mint-prints-the-key test "$key" = hskey-auth-FAKE
check mint-targets-the-user logged 'preauthkeys create .*(-u|--user) 4( |$)'
check mint-tagged logged 'preauthkeys create .*--tags tag:e2e-host'
check mint-one-hour logged 'preauthkeys create .*--expiration 1h'
check mint-single-use refuses logged '--reusable'
check mint-key-not-in-argv-log refuses grep -q "$key" "$SHIM_LOG"
hs_down >/dev/null 2>&1

# Guest reachability: a guest that cannot reach the control plane names itself and the URL.
shim dotf 'echo "$*" >>"$SHIM_LOG"; [ ! -f "$WORK/guest-blocked" ] || exit 1'
: >"$SHIM_LOG"
check guest-reaches hs_guest_reaches linux tsb-e2e-linux
check guest-reaches-url logged 'vm linux exec --session tsb-e2e-linux .*http://10\.9\.8\.7:8080/health'
touch "$WORK/guest-blocked"
check guest-blocked-fails refuses hs_guest_reaches windows tsb-e2e-windows
check guest-blocked-message grep -qx 'guest windows cannot reach the control plane at http://10.9.8.7:8080/health' <<<"$(hs_guest_reaches windows tsb-e2e-windows 2>&1 || true)"

[ "$fails" -eq 0 ] || { echo "test-e2e-headscale: $fails case(s) failed"; exit 1; }
echo "test-e2e-headscale: all cases passed"
