#!/bin/sh
# tailscaled firewall capture (#526). Runs as root on a fresh Ubuntu 24.04 guest, with the
# candidate at /tmp/tsb-guest. Stdin carries the control URL, then a tagged auth key: the key
# goes to a 0600 file and nowhere else, never argv or output.
#
# It enrols a real tailscaled against the local Headscale and, for each netfilter mode, prints
# what the readers of internal/host would see, verbatim, plus `ts-bridge host check` on that
# state. The output is the source of the fixtures in internal/host/linux_steps_test.go; it is a
# capture, not a pass/fail scenario. Each section starts with "=== " and each command with "--- ".
set -u
read -r url
read -r key
TS_VERSION=1.104.4 # matches tailscale.com in go.mod: the fixtures describe this tailscaled

export DEBIAN_FRONTEND=noninteractive
curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/noble.noarmor.gpg -o /usr/share/keyrings/tailscale-archive-keyring.gpg || exit 1
curl -fsSL https://pkgs.tailscale.com/stable/ubuntu/noble.tailscale-keyring.list -o /etc/apt/sources.list.d/tailscale.list || exit 1
apt-get update -qq >/dev/null 2>&1
apt-get install -y -qq "tailscale=$TS_VERSION" nftables iptables >/dev/null 2>&1 || { echo "could not install tailscale $TS_VERSION" >&2; exit 1; }

install -m 0755 /tmp/tsb-guest /usr/local/bin/ts-bridge
mkdir -p /etc/ts-bridge
cat >/etc/ts-bridge/host.yaml <<'YAML'
version: 1
host:
  services:
    - kind: rdp
      port: 3389
YAML
(umask 077 && printf '%s\n' "$key" >/root/authkey)

# defaults MODE_ENV writes tailscaled's environment; the DERP variables are the harness's own (QA-607).
defaults() {
  printf 'PORT="41641"\nFLAGS=""\nTS_DEBUG_USE_DERP_HTTP=1\nTS_DEBUG_ALWAYS_USE_DERP=1\n%s\n' "$1" >/etc/default/tailscaled
}

# enrol MODE brings tailscaled up in a netfilter mode from a clean start, and waits for ts-input.
enrol() {
  tailscale down >/dev/null 2>&1
  systemctl restart tailscaled
  sleep 2
  tailscale up --login-server "$url" --auth-key file:/root/authkey --hostname tsb-e2e-tsfw \
    --netfilter-mode "$1" --reset >/dev/null 2>&1 || { echo "tailscale up --netfilter-mode $1 failed" >&2; return 1; }
  for _ in $(seq 1 30); do
    { iptables -S ts-input || nft list chain ip filter ts-input; } >/dev/null 2>&1 && return 0
    sleep 1
  done
  echo "ts-input did not appear in mode $1" >&2
}

show() { # show CMD... prints the command's stdout and stderr verbatim, then its status
  echo "--- $*"
  "$@" 2>&1
  echo "--- rc=$?"
}

capture() {
  echo "=== $1"
  show tailscale version
  show iptables --version
  show iptables -S INPUT
  show iptables -S ts-input
  show ip6tables -S INPUT
  show ip6tables -S ts-input
  if command -v nft >/dev/null; then
    show nft list tables
    show nft list chain ip filter ts-input
    show nft list chain ip6 filter ts-input
  fi
  show ts-bridge host check --config /etc/ts-bridge/host.yaml --json
}

defaults ""
enrol on && capture "netfilter-mode on (iptables)"
enrol nodivert && capture "netfilter-mode nodivert (iptables)"
defaults "TS_DEBUG_FIREWALL_MODE=nftables"
enrol on && capture "netfilter-mode on (nftables)"
apt-get remove -y -qq nftables >/dev/null 2>&1
capture "netfilter-mode on (nftables), nft binary removed"
tailscale logout >/dev/null 2>&1
rm -f /root/authkey
