#!/bin/sh
# shellcheck disable=SC2016,SC2034,SC2317,SC2329 # SC2317 is SC2329 before shellcheck 0.11; cases are strings eval'd by ok(), which calls the helpers
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

# Inactive ufw whose only allow is off the tailnet: setup still adds the tailscale0 allow for when
# ufw is enabled (#631 review). Both allows are removed after, so the fresh-guest cases below hold.
ufw allow from 10.0.0.0/8 to any port 3389 proto tcp >/dev/null 2>&1
ok "ufw inactive, off-tailnet allow only: setup adds the tailscale0 allow, keeps the operator's" -- 'outcomes setup "$C" >/dev/null; ufw show added | grep -qx "ufw allow in on tailscale0 to any port 3389 proto tcp" && ufw show added | grep -qx "ufw allow from 10.0.0.0/8 to any port 3389 proto tcp"'
ufw delete allow from 10.0.0.0/8 to any port 3389 proto tcp >/dev/null 2>&1
ufw delete allow in on tailscale0 to any port 3389 proto tcp >/dev/null 2>&1

# Fresh guest: no xrdp, ufw inactive, no Tailscale.
run "$B" host setup --config "$C"
ok "setup without xrdp: exit 2, xrdp step names the fix" -- 'code 2 && has "apt install xrdp"'
ok "setup not converged: no HOST READY banner" -- '! has "HOST READY"'
ok "setup without IP: no empty TS_TARGET" -- '! has "TS_TARGET=:"'
outcomes setup "$C" >"$W/j"
# The step must exist, so all() over no step at all cannot pass.
ok "ufw inactive: setup prints its Firewall step" -- "json '[s for s in j[\"steps\"] if s[\"name\"]==\"Firewall configuration\"]' <$W/j"
ok "ufw inactive: firewall step says so" -- "json '[s for s in j[\"steps\"] if s[\"name\"].startswith(\"Firewall\") and \"inactive\" in s[\"message\"]]' <$W/j"
ok "ufw inactive: check does not report firewall_ok" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"

apt-get -qq update >/dev/null 2>&1 && apt-get -qq install -y xrdp >/dev/null 2>&1
ufw --force enable >/dev/null
outcomes setup "$C" >"$W/j1"
# Name the two steps: all() over no step at all would pass.
ok "setup #1: xrdp ok, firewall converged" -- "json '{s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"xrdp service\") in (\"ok\",\"changed\") and {s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"Firewall configuration\") in (\"ok\",\"changed\") and all(s[\"outcome\"] in (\"ok\",\"changed\") for s in j[\"steps\"] if not s[\"name\"].startswith(\"Tailscale\"))' <$W/j1"
outcomes setup "$C" >"$W/j2"
# ADR-017: a second setup on a converged host reports ok, not failed or manual.
ok "setup #2: nothing changed" -- "json '{s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"xrdp service\")==\"ok\" and {s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"Firewall configuration\")==\"ok\" and all(s[\"outcome\"]==\"ok\" for s in j[\"steps\"] if not s[\"name\"].startswith(\"Tailscale\"))' <$W/j2"
ok "setup: Tailscale step fails without a node" -- "json '[s for s in j[\"steps\"] if s[\"name\"].startswith(\"Tailscale\") and s[\"outcome\"]==\"failed\" and s[\"fix\"]]' <$W/j2"
outcomes check "$C" >"$W/jc"
ok "check --json: rdp 3389 enabled, active, firewall ok" -- "json 'j[\"services\"]==[{\"kind\":\"rdp\",\"port\":3389,\"enabled\":True,\"active\":True,\"firewall_ok\":True,\"unread\":False}]' <$W/jc"
ok "xrdp listens on 3389" -- "ss -ltn | grep -q ':3389 '"

ufw delete allow in on tailscale0 to any port 3389 proto tcp >/dev/null 2>&1
ok "drift: removed ufw rule seen by check" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"
ok "drift: setup restores the ufw rule" -- "outcomes setup $C >/dev/null; ufw status | grep -q '^3389/tcp *on tailscale0 '"
systemctl stop xrdp
ok "drift: stopped xrdp seen by check" -- "outcomes check $C | json 'not j[\"services\"][0][\"active\"]'"
ok "drift: setup starts xrdp" -- "outcomes setup $C >/dev/null; systemctl is-active -q xrdp"

U="${SUDO_USER:-nobody}"
ok "unprivileged check names the declared service" -- 'su -s /bin/sh "$U" -c "$B host check --config $C" 2>&1 | grep -q "RDP:.*3389"'
# #638: a systemctl that answers nothing leaves the unit unread, whoever runs the
# check, and elevation cannot fix that. The declared service must still be
# listed, flagged unread, and the wording must offer elevation only to a non-root
# run.
mkdir -p "$W/shim" && printf '#!/bin/sh\nexit 1\n' >"$W/shim/systemctl" && chmod 755 "$W/shim/systemctl"
as_user() { su -s /bin/sh "$U" -c "PATH=$W/shim $B host check --config $C $1" 2>/dev/null; }
ok "unprivileged unread: check --json lists rdp 3389 with unread true" -- "as_user --json | json 'j[\"rdp_port\"]==0 and [(s[\"kind\"],s[\"port\"],s[\"unread\"]) for s in j[\"services\"]]==[(\"rdp\",3389,True)]'"
ok "unprivileged unread: line names the steps, elevation only as a maybe" -- 'as_user "" | grep -q "RDP:.*unread (rdp, port 3389): see the steps below for the cause; running as root/administrator may help"'
run env PATH="$W/shim" "$B" host check --config "$C"
ok "root unread: line names the steps and does not suggest elevation" -- 'has "RDP:.*unread (rdp, port 3389): see the steps below for the cause" && ! has "may help"'
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
# A declared port moves the xrdp listener, not only the firewall (#557).
xport() { json "{s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"xrdp port\")==\"$1\""; }
ok "declared port 3390: setup points xrdp at it" -- "outcomes setup $W/p.yaml | xport changed"
ok "declared port 3390: xrdp listens on it" -- "sleep 2; ss -ltn | grep -q ':3390 ' && grep -q '^port=3390$' /etc/xrdp/xrdp.ini"
ok "declared port 3390: second setup changes nothing" -- "outcomes setup $W/p.yaml | xport ok"
ok "declared port 3390: the stock file sees the drift" -- "outcomes check $C | xport changed"
ok "declared port 3389: setup moves xrdp back" -- "outcomes setup $C | xport changed && sleep 2 && ss -ltn | grep -q ':3389 '"
ok "declared port 3389: back to the stock step list" -- "outcomes check $C | json 'not [s for s in j[\"steps\"] if s[\"name\"]==\"xrdp port\"]'"

# Upgrade: the previous release's .env still applies from its directory.
mkdir -p "$W/up" && cd "$W/up" && rm -f .env
"$OLD" host init --port 3390 </dev/null >/dev/null 2>&1
ok "upgrade: previous init wrote TS_HOST_RDP_PORT" -- "grep -q '^TS_HOST_RDP_PORT=3390' .env"
ok "upgrade: .env beats the file in check" -- "$B host check --config $C --json 2>/dev/null | json 'j[\"rdp_port\"]==3390'"
ok "upgrade: init without --config fails" -- "! $B host init </dev/null"
cd "$W" && ufw delete allow in on tailscale0 to any port 3390 proto tcp >/dev/null 2>&1

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

SCOPED='-A INPUT -i tailscale0 -p tcp -m tcp --dport 3389 -j ACCEPT'
LEGACY='-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT'
FOREIGN='-A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT'
fwstep() { json "{s[\"name\"]:s[\"outcome\"] for s in j[\"steps\"]}.get(\"Firewall configuration\")==\"$1\""; }
# Firewall scope (#631) on a ufw host: setup writes the rule on tailscale0 only.
ok "ufw host: setup wrote the allow on tailscale0" -- "ufw status | grep -q '^3389/tcp *on tailscale0 '"
ok "ufw host: no unscoped 3389 allow beside it" -- '! ufw show added | grep -qx "ufw allow 3389/tcp"'
ufw allow 3389/tcp >/dev/null 2>&1
ok "legacy ufw allow: check says setup would change it" -- 'outcomes check "$C" | fwstep changed'
ok "legacy ufw allow: check does not report firewall_ok" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"
ok "legacy ufw allow: setup replaces it" -- 'outcomes setup "$C" | fwstep changed && ! ufw show added | grep -qx "ufw allow 3389/tcp" && ufw show added | grep -qx "ufw allow in on tailscale0 to any port 3389 proto tcp"'
ok "legacy ufw allow: check is ok afterwards" -- 'outcomes check "$C" | fwstep ok'
ufw allow 22,3389/tcp >/dev/null 2>&1
ok "foreign ufw allow: check says manual" -- 'outcomes check "$C" | fwstep manual'
ok "foreign ufw allow: setup reports it and leaves it" -- 'outcomes setup "$C" | fwstep manual && ufw show added | grep -qx "ufw allow 22,3389/tcp"'
ufw delete allow 22,3389/tcp >/dev/null 2>&1
iptables -A INPUT -p tcp --dport 3389 -j ACCEPT
ok "legacy iptables copy on a ufw host: check says setup would change it" -- 'outcomes check "$C" | fwstep changed'
ok "legacy iptables copy on a ufw host: setup deletes it" -- 'outcomes setup "$C" | fwstep changed && ! iptables -S INPUT | grep -qx -- "$LEGACY" && outcomes check "$C" | fwstep ok'
# An allow scoped off the tailnet opens the port, but not to the tailnet: setup still owes its rule (#631).
OFFNET='ufw allow from 10.0.0.0/8 to any port 3389 proto tcp'
ufw delete allow in on tailscale0 to any port 3389 proto tcp >/dev/null 2>&1
ufw allow from 10.0.0.0/8 to any port 3389 proto tcp >/dev/null 2>&1
ok "off-tailnet ufw allow only: check says setup would change it" -- 'outcomes check "$C" | fwstep changed'
ok "off-tailnet ufw allow only: check does not report firewall_ok" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"
ok "off-tailnet ufw allow only: setup adds the tailscale0 allow, keeps the operator's" -- 'outcomes setup "$C" | fwstep changed && ufw show added | grep -qx "ufw allow in on tailscale0 to any port 3389 proto tcp" && ufw show added | grep -qx -- "$OFFNET"'
ok "off-tailnet ufw allow only: check is ok afterwards" -- 'outcomes check "$C" | fwstep ok'
ufw delete allow from 10.0.0.0/8 to any port 3389 proto tcp >/dev/null 2>&1

# Firewall scope (#631) on an iptables host: ufw is removed, so setup writes through iptables.
ufw --force disable >/dev/null 2>&1
apt-get -qq remove -y ufw >/dev/null 2>&1
ok "iptables host: ufw is gone" -- '[ ! -e /usr/sbin/ufw ]'
# A default-ACCEPT INPUT policy (#691) opens every port on every interface, so the tailscale0 scope
# restricts nothing: setup still writes the scoped rule, reports manual, and never changes the policy.
ok "permissive policy: the guest's INPUT policy is ACCEPT" -- 'iptables -S INPUT | grep -qx -- "-P INPUT ACCEPT"'
ok "permissive policy: check says manual, naming the policy" -- 'outcomes check "$C" | fwstep manual && outcomes check "$C" | json "\"iptables -P INPUT DROP\" in [s for s in j[\"steps\"] if s[\"name\"]==\"Firewall configuration\"][0][\"fix\"]"'
ok "permissive policy: check does not report firewall_ok" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"
ok "permissive policy: setup writes the scoped rule, reports manual, leaves the policy" -- 'outcomes setup "$C" | fwstep manual && iptables -S INPUT | grep -qx -- "$SCOPED" && iptables -S INPUT | grep -qx -- "-P INPUT ACCEPT"'
iptables -D INPUT -i tailscale0 -p tcp --dport 3389 -j ACCEPT
# The cases below need a closed policy; SSH and replies stay open so the harness keeps its channel.
iptables -A INPUT -i lo -j ACCEPT
iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT
iptables -A INPUT -p tcp --dport 22 -j ACCEPT
iptables -P INPUT DROP
ok "closed policy: the guest's INPUT policy is DROP" -- 'iptables -S INPUT | grep -qx -- "-P INPUT DROP"'
ok "iptables host: setup #1 appends the rule" -- 'outcomes setup "$C" | fwstep changed'
ok "iptables host: the rule is scoped to tailscale0, as printed" -- 'iptables -S INPUT | grep -qx -- "$SCOPED"'
ok "iptables host: setup #2 changes nothing (the printed form agrees)" -- 'outcomes setup "$C" | fwstep ok'
ok "iptables host: check reports firewall_ok" -- "outcomes check $C | json 'j[\"services\"][0][\"firewall_ok\"]'"
iptables -A INPUT -p tcp --dport 3389 -j ACCEPT
ok "legacy unscoped rule: check says setup would change it" -- 'outcomes check "$C" | fwstep changed'
ok "legacy unscoped rule: check does not report firewall_ok" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"
ok "legacy unscoped rule: setup replaces it" -- 'outcomes setup "$C" | fwstep changed && ! iptables -S INPUT | grep -qx -- "$LEGACY" && iptables -S INPUT | grep -qx -- "$SCOPED"'
ok "legacy unscoped rule: check is ok afterwards" -- 'outcomes check "$C" | fwstep ok'
iptables -A INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT
ok "foreign unscoped rule: check says manual" -- 'outcomes check "$C" | fwstep manual'
ok "foreign unscoped rule: setup reports it and leaves it" -- 'outcomes setup "$C" | fwstep manual && iptables -S INPUT | grep -qx -- "$FOREIGN"'
iptables -D INPUT -p tcp -m multiport --dports 22,3389 -j ACCEPT
ok "foreign rule removed by the operator: check is ok again" -- 'outcomes check "$C" | fwstep ok'
OFFNET='-A INPUT -s 10.0.0.0/8 -p tcp -m tcp --dport 3389 -j ACCEPT'
iptables -D INPUT -i tailscale0 -p tcp --dport 3389 -j ACCEPT
iptables -A INPUT -s 10.0.0.0/8 -p tcp --dport 3389 -j ACCEPT
ok "off-tailnet iptables allow only: check says setup would change it" -- 'outcomes check "$C" | fwstep changed'
ok "off-tailnet iptables allow only: setup adds the tailscale0 rule, keeps the operator's" -- 'outcomes setup "$C" | fwstep changed && iptables -S INPUT | grep -qx -- "$SCOPED" && iptables -S INPUT | grep -qx -- "$OFFNET"'
ok "off-tailnet iptables allow only: check is ok afterwards" -- 'outcomes check "$C" | fwstep ok'
iptables -D INPUT -s 10.0.0.0/8 -p tcp --dport 3389 -j ACCEPT
# IPv6 exposure (#526): an unscoped v6 allow for the port is reported, never deleted, and check
# returns ok once the operator removes it.
V6OPEN='-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT'
ip6tables -A INPUT -p tcp --dport 3389 -j ACCEPT
ok "unscoped v6 allow: check says manual, naming the ip6tables fix" -- 'outcomes check "$C" | fwstep manual && outcomes check "$C" | json "\"ip6tables -D INPUT\" in [s for s in j[\"steps\"] if s[\"name\"]==\"Firewall configuration\"][0][\"fix\"]"'
ok "unscoped v6 allow: check does not report firewall_ok" -- "outcomes check $C | json 'not j[\"services\"][0][\"firewall_ok\"]'"
ok "unscoped v6 allow: setup reports it and leaves it" -- 'outcomes setup "$C" | fwstep manual && ip6tables -S INPUT | grep -qx -- "$V6OPEN"'
ip6tables -D INPUT -p tcp --dport 3389 -j ACCEPT
ok "v6 allow removed by the operator: check is ok again" -- 'outcomes check "$C" | fwstep ok'
# POSIX ACLs on a state_dir ancestor (#598). setfacl raises the mask with a named entry and the group
# bits mirror it, so the mode check alone already refuses that; the ACL read names the principal, and is
# the only layer that sees an entry the mask hides (setfacl -n).
apt-get -qq install -y acl >/dev/null 2>&1
ok "acl: setfacl is installed" -- 'command -v setfacl'
install -d -m 0755 /var/lib/acl-e2e && install -d -m 0755 /etc/ts-bridge && echo k >/etc/ts-bridge/e2e-key && chmod 600 /etc/ts-bridge/e2e-key
mesh /var/lib/acl-e2e/state /etc/ts-bridge/e2e-key >"$W/acl.yaml"
setfacl -m u:nobody:rwx /var/lib/acl-e2e
ok "acl: setfacl raises the mask, and stat shows it as the group bits (0775)" -- '[ "$(stat -c %a /var/lib/acl-e2e)" = 775 ]'
run timeout 20 "$B" host serve --config "$W/acl.yaml"
ok "serve: ancestor with a named-user write ACL refused, naming it and the user" -- 'code 1 && has "/var/lib/acl-e2e" && has "ACL grants write to user"'
setfacl -b /var/lib/acl-e2e && setfacl -n -m u:nobody:rwx /var/lib/acl-e2e
ok "acl: setfacl -n leaves the mask at r-x, so the mode stays 0755" -- '[ "$(stat -c %a /var/lib/acl-e2e)" = 755 ]'
run timeout 20 "$B" host serve --config "$W/acl.yaml"
ok "serve: a named-user write the mask hides is still refused" -- 'code 1 && has "/var/lib/acl-e2e" && has "ACL grants write to user"'
setfacl -b /var/lib/acl-e2e && setfacl -m g:nogroup:rwx /var/lib/acl-e2e
run timeout 20 "$B" host serve --config "$W/acl.yaml"
ok "serve: ancestor with a named-group write ACL refused, naming it and the group" -- 'code 1 && has "/var/lib/acl-e2e" && has "ACL grants write to group"'
# A refusal creates nothing; a pass creates state_dir (EnsureStateDir walks the ancestors first). That
# makes the directory the positive half of the next case, so it cannot go green on an unrelated error.
ok "acl: the refused runs created no state_dir" -- '[ ! -e /var/lib/acl-e2e/state ]'
setfacl -b /var/lib/acl-e2e && setfacl -m u:nobody:r-x /var/lib/acl-e2e
run timeout 20 "$B" host serve --config "$W/acl.yaml"
ok "serve: ancestor with a read-only named-user ACL is not refused, and the walk lets state_dir be created" -- '! has "ancestor" && [ -d /var/lib/acl-e2e/state ]'
rm -rf /var/lib/acl-e2e/state
setfacl -b /var/lib/acl-e2e

run "$B" import e2e "tsb://host-e2e:3389?cp=saas"
ok "client: import a tsb:// descriptor" -- 'code 0'
run "$B" import bad "http://nope"
ok "client: malformed descriptor refused" -- '! code 0'
"$B" status --json --addr 127.0.0.1:1 >"$W/out" 2>"$W/last"; echo $? >"$W/code"
ok "client: status with no bridge prints running false, hint on stderr" -- 'code 0 && json "j[\"running\"] is False and j[\"addr\"]==\"127.0.0.1:1\"" <"$W/out" && has "not running"'
exit 0
