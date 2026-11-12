#!/bin/sh
# shellcheck disable=SC2016,SC2034,SC2329 # cases are strings eval'd by ok(), which calls the helpers
# Linux guest mesh scenario (QA-607). Runs as root on a fresh Ubuntu 24.04 guest, with the
# candidate at /tmp/tsb-guest and probe.py at /tmp/probe.py. Stdin carries the control URL, then a
# tagged single-use auth key: the key goes to a 0600 file and nowhere else, never argv or output.
# Prints one line per case, as linux.sh does; exits 1 if one failed.
set -u
read -r url
read -r key
ok() { # ok NAME -- EXPR
  if eval "$3" >/dev/null 2>&1; then echo "PASS $1"; else echo "FAIL $1"; rc=1; fi
}
rc=0
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq >/dev/null 2>&1
apt-get install -y -qq xrdp >/dev/null 2>&1
systemctl enable --now xrdp >/dev/null 2>&1
ok "mesh rdp: xrdp answers inside the guest" -- 'python3 /tmp/probe.py x224 127.0.0.1:3389'

install -m 0755 /tmp/tsb-guest /usr/local/bin/ts-bridge
mkdir -p /etc/ts-bridge
cat >/etc/ts-bridge/host.yaml <<YAML
version: 1
host:
  control_url: $url
  services:
    - kind: rdp
      port: 3389
  mesh:
    hostname: tsb-e2e-linux
    state_dir: /var/lib/ts-bridge/serve
    auth_key_file: /etc/ts-bridge/authkey
    health_addr: 127.0.0.1:9090
YAML
(umask 077 && printf '%s\n' "$key" >/etc/ts-bridge/authkey)
cat >/etc/systemd/system/ts-bridge-serve.service <<'UNIT'
[Unit]
Description=ts-bridge host serve
Wants=network-online.target
After=network-online.target

[Service]
Environment=TS_DEBUG_USE_DERP_HTTP=1
Environment=TS_DEBUG_ALWAYS_USE_DERP=1
ExecStart=/usr/local/bin/ts-bridge host serve --config /etc/ts-bridge/host.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
systemctl enable --now ts-bridge-serve.service >/dev/null 2>&1
ready() { journalctl -u ts-bridge-serve --no-pager 2>/dev/null | grep -q 'READY node=tsb-e2e-linux services=rdp:3389 enrolled=true'; }
for _ in $(seq 1 60); do ready && break; sleep 1; done
ok "mesh enrol: READY enrolled=true" -- ready
[ "$rc" -eq 0 ] || journalctl -u ts-bridge-serve --no-pager 2>&1 | tail -15 | sed 's/^/serve: /'
exit "$rc"
