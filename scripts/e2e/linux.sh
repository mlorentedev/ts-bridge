#!/bin/sh
# shellcheck disable=SC2016,SC2034,SC2329 # cases are strings eval'd by ok(), which calls the helpers
# Linux guest scenario (QA-606). Runs as root on a fresh Ubuntu 24.04 guest,
# with /tmp/tsb-new (candidate) and /tmp/tsb-old (previous release) copied in.
# Prints one line per case: PASS, FAIL, XFAIL #N or XPASS #N.
set -u
B=/tmp/tsb-new
OLD=/tmp/tsb-old
C=/etc/ts-bridge/host.yaml
W=/tmp/e2e
mkdir -p "$W" && cd "$W" || exit 2
export DEBIAN_FRONTEND=noninteractive

# result NAME OK [ISSUE]: OK is 0 when the case holds. ISSUE marks a known
# open bug: it reports XFAIL while failing and XPASS once fixed (AC5).
result() {
  if [ -n "${3:-}" ]; then
    if [ "$2" -eq 0 ]; then echo "XPASS #$3 $1"; else echo "XFAIL #$3 $1"; fi
  elif [ "$2" -eq 0 ]; then echo "PASS $1"; else echo "FAIL $1"; fi
}
# ok NAME [ISSUE] -- EXPR: the case holds when the shell expression EXPR,
# evaluated here so the helpers below are in scope, exits 0.
ok() {
  n="$1"; shift; i=""
  [ "$1" != "--" ] && { i="$1"; shift; }
  shift
  if eval "$1" >/dev/null 2>&1; then result "$n" 0 "$i"; else result "$n" 1 "$i"; fi
}
# json EXPR: reads JSON on stdin, exits 0 when the Python expression on j holds.
json() { python3 -c "import json,sys; j=json.load(sys.stdin); sys.exit(0 if ($1) else 1)"; }
outcomes() { "$B" host "$1" --config "$2" --json 2>/dev/null; }
has() { grep -q -- "$1" "$W/last"; }
run() { "$@" >"$W/last" 2>&1; echo $? >"$W/code"; }
code() { [ "$(cat "$W/code")" = "$1" ]; }

ok "version prints" -- '"$B" version' 
run "$B" host init </dev/null
ok "init: --config required" -- 'code 1 && has "--config is required"'
run "$B" host init --config "$C" --port 3389 --force </dev/null
ok "init: writes the YAML" -- "code 0 && grep -q 'kind: rdp' $C && grep -q 'port: 3389' $C"
ok "init: file is 0644 root" -- '[ "$(stat -c "%a %U" "$C")" = "644 root" ]' 
cp "$C" "$W/before"
run "$B" host init --config "$C" --port 3390 </dev/null
ok "init: existing file kept without --force" -- "! code 0 && cmp -s $C $W/before"
run "$B" host init --config "$C" --port 3390 --force </dev/null
ok "init: --force replaces" -- "grep -q 'port: 3390' $C"
"$B" host init --config "$C" --port 3389 --force </dev/null >/dev/null 2>&1
mkdir -p "$W/dir.yaml"
run "$B" host init --config "$W/dir.yaml" --force </dev/null
ok "init: directory refused even with --force" -- '! code 0'
run "$B" host init --config "$W/x.yaml" --firewall-rule R </dev/null
ok "init: --firewall-rule refused on linux" -- "! code 0 && [ ! -e $W/x.yaml ]"

# Fresh guest: no xrdp, ufw inactive, no Tailscale.
run "$B" host setup --config "$C"
ok "setup without xrdp: exit 2, xrdp step names the fix" -- 'code 2 && has "apt install xrdp"'
ok "setup not converged: no HOST READY banner" -- '! has "HOST READY"'
ok "setup without IP: no empty TS_TARGET" -- '! has "TS_TARGET=:"'
outcomes setup "$C" >"$W/j"
# Unmarked, so a missing or broken document is a FAIL and not #608.
ok "ufw inactive: setup prints its Firewall step" -- "json '[s for s in j[\"steps\"] if s[\"name\"]==\"Firewall configuration\"]' <$W/j"
ok "ufw inactive: firewall step says so" 608 -- "json '[s for s in j[\"steps\"] if s[\"name\"].startswith(\"Firewall\") and \"inactive\" in s[\"message\"]]' <$W/j"

apt-get -qq update >/dev/null 2>&1 && apt-get -qq install -y xrdp >/dev/null 2>&1
ufw --force enable >/dev/null
outcomes setup "$C" >"$W/j1"
# Name the two steps: all() over no step at all would pass.
ok "setup #1: xrdp ok, firewall converged" -- "json '{s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"xrdp service\") in (\"ok\",\"changed\") and {s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"Firewall configuration\") in (\"ok\",\"changed\") and all(s[\"outcome\"] in (\"ok\",\"changed\") for s in j[\"steps\"] if not s[\"name\"].startswith(\"Tailscale\"))' <$W/j1"
outcomes setup "$C" >"$W/j2"
# ADR-017: a second setup on a converged host reports ok, not failed or manual.
ok "setup #2: nothing changed" -- "json 'j[\"steps\"] and all(s[\"outcome\"]==\"ok\" for s in j[\"steps\"] if not s[\"name\"].startswith(\"Tailscale\"))' <$W/j2"
ok "setup: Tailscale step fails without a node" -- "json '[s for s in j[\"steps\"] if s[\"name\"].startswith(\"Tailscale\") and s[\"outcome\"]==\"failed\" and s[\"fix\"]]' <$W/j2"
outcomes check "$C" >"$W/jc"
ok "check --json: rdp 3389 enabled, active, firewall ok" -- "json 'j[\"services\"]==[{\"kind\":\"rdp\",\"port\":3389,\"enabled\":True,\"active\":True,\"firewall_ok\":True}]' <$W/jc"
ok "xrdp listens on 3389" -- "ss -ltn | grep -q ':3389 '"

ufw delete allow 3389/tcp >/dev/null 2>&1
ok "drift: removed ufw rule seen by check" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"
ok "drift: setup restores the ufw rule" -- "outcomes setup $C >/dev/null; ufw status | grep -q '^3389/tcp'"
systemctl stop xrdp
ok "drift: stopped xrdp seen by check" -- "outcomes check $C | json 'not j[\"services\"][0][\"active\"]'"
ok "drift: setup starts xrdp" -- "outcomes setup $C >/dev/null; systemctl is-active -q xrdp"

U="${SUDO_USER:-nobody}"
ok "unprivileged check names the declared service" -- 'su -s /bin/sh "$U" -c "$B host check --config $C" 2>&1 | grep -q "RDP:.*3389"'
systemctl disable --now xrdp >/dev/null 2>&1
run "$B" host check --config "$C"
ok "disabled xrdp: check names it NOT OK, not undeclared" -- '! has "no service declared" && has "RDP:.*NOT OK.*3389"'
"$B" host setup --config "$C" >/dev/null 2>&1
ok "disabled xrdp: setup enables and starts it" -- 'systemctl is-enabled -q xrdp && systemctl is-active -q xrdp'

printf 'version: 1\nhost:\n  services:\n    - kind: ssh\n      port: 22\n' >"$W/ssh.yaml"
run "$B" host setup --config "$W/ssh.yaml"
ok "ssh service refused, citing #521" -- 'code 1 && has "#521"'
printf 'version: 1\nhost:\n  services:\n    - kind: rdp\n      port: 3389\n  colour: blue\n' >"$W/unk.yaml"
run "$B" host check --config "$W/unk.yaml"
ok "unknown key warned by name, check still runs" -- 'has "colour" && has "HOST CHECK"'
printf 'version: 2\nhost: {}\n' >"$W/v2.yaml"
run "$B" host check --config "$W/v2.yaml"
ok "unsupported version refused" -- '! code 0 && ! code 2 && has "version"'
printf 'version: 1\nhost:\n  services:\n    - kind: rdp\n      port: 3390\n' >"$W/p.yaml"
# The command itself must work, unmarked, so a crash is a FAIL and not #557.
ok "declared port 3390: setup prints its steps as JSON" -- "outcomes setup $W/p.yaml | json 'len(j[\"steps\"])>0'"
ok "declared port no step applies is not converged" 557 -- "outcomes setup $W/p.yaml | json '[s for s in j[\"steps\"] if s[\"outcome\"] in (\"failed\",\"manual\") and \"3390\" in s[\"message\"]]'"

# Upgrade: the previous release's .env still applies from its directory.
mkdir -p "$W/up" && cd "$W/up" && rm -f .env
"$OLD" host init --port 3390 </dev/null >/dev/null 2>&1
ok "upgrade: previous init wrote TS_HOST_RDP_PORT" -- "grep -q '^TS_HOST_RDP_PORT=3390' .env"
ok "upgrade: .env beats the file in check" -- "$B host check --config $C --json 2>/dev/null | json 'j[\"rdp_port\"]==3390'"
ok "upgrade: init without --config fails" -- "! $B host init </dev/null"
cd "$W" && ufw delete allow 3390/tcp >/dev/null 2>&1

# host serve preflight refusals; nothing may start or reach the network.
printf 'version: 1\nhost:\n  control_url: https://hs.invalid\n  services:\n    - kind: rdp\n      port: 3389\n' >"$W/nomesh.yaml"
run timeout 20 "$B" host serve --config "$W/nomesh.yaml"
ok "serve: no host.mesh refused" -- 'code 1 && has "host.mesh"'
mesh() { printf 'version: 1\nhost:\n  control_url: https://hs.invalid\n  mesh:\n    hostname: e2e\n    state_dir: %s\n    auth_key_file: %s\n  services:\n    - kind: rdp\n      port: 3389\n' "$1" "$2"; }
mesh /tmp/e2e-state /etc/ts-bridge/authkey >"$W/loose.yaml"
run timeout 20 "$B" host serve --config "$W/loose.yaml"
ok "serve: state_dir under /tmp refused, naming it" -- 'code 1 && has "/tmp"'
install -d -m 0700 /var/lib/ts-bridge-e2e && echo k >/tmp/e2e-key && chmod 600 /tmp/e2e-key
mesh /var/lib/ts-bridge-e2e /tmp/e2e-key >"$W/key.yaml"
run timeout 20 "$B" host serve --config "$W/key.yaml"
ok "serve: auth_key_file under /tmp refused" -- 'code 1 && has "/tmp"'

run "$B" import e2e "tsb://host-e2e:3389?cp=saas"
ok "client: import a tsb:// descriptor" -- 'code 0'
run "$B" import bad "http://nope"
ok "client: malformed descriptor refused" -- '! code 0'
"$B" status --json --addr 127.0.0.1:1 >"$W/out" 2>"$W/last"; echo $? >"$W/code"
ok "client: status with no bridge says so on stderr, stdout empty" -- 'code 0 && [ ! -s "$W/out" ] && has "not running"'
exit 0
