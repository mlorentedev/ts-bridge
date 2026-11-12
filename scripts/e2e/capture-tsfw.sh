#!/usr/bin/env bash
# Captures what tailscaled installs in the Linux firewall in each netfilter mode (#526), on a
# throwaway Ubuntu guest enrolled against the local Headscale (QA-607). The fixtures in
# internal/host/linux_steps_test.go are copied from its log; see tsfw_linux.sh for the modes.
#
# Usage: scripts/e2e/capture-tsfw.sh
#
# Writes $OUT/tsfw-linux-<time>.log and always removes the guest and the control plane. The
# Headscale container is shared with the mesh scenarios: do not run both at once.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
OUT="${E2E_OUT:-$HERE/out}"
SESSION=tsb-e2e-tsfw
dir=""

# shellcheck source-path=SCRIPTDIR source=headscale.sh
. "$HERE/headscale.sh"

die() { echo "capture: $*" >&2; exit 2; }

teardown() {
  local rc=$?
  trap - EXIT
  rm -rf "$dir"
  if dotf vm linux status 2>/dev/null | awk '{print $1}' | grep -qx "$SESSION"; then
    dotf vm linux down --session "$SESSION" >/dev/null 2>&1 || { echo "capture: could not delete $SESSION" >&2; [ "$rc" != 0 ] || rc=1; }
  fi
  hs_down || { [ "$rc" != 0 ] || rc=1; }
  exit "$rc"
}

main() {
  command -v dotf >/dev/null || die "dotf not found"
  { command -v docker >/dev/null && docker info >/dev/null 2>&1; } || die "no docker engine answers: start colima"
  local log
  dir="$(mktemp -d)"
  trap teardown EXIT
  mkdir -p "$OUT"
  log="$OUT/tsfw-linux-$(date -u +%Y%m%dT%H%M%SZ).log"
  hs_up
  dotf vm linux up --session "$SESSION" >/dev/null
  hs_guest_reaches linux "$SESSION" || exit 1
  (cd "$ROOT" && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o "$dir/tsb-guest" ./cmd/ts-bridge)
  dotf vm linux cp "$dir/tsb-guest" "$SESSION:/tmp/tsb-guest" >/dev/null
  echo "capture: candidate $(cd "$ROOT" && git rev-parse --short HEAD)" | tee "$log"
  { echo "http://$(hs_host_ip):$HS_PORT"; hs_mint_key tag:e2e-host; } |
    dotf vm linux exec --session "$SESSION" --elevated --timeout 20m --script "$HERE/tsfw_linux.sh" >>"$log" 2>&1 ||
    { tail -5 "$log" >&2; die "the guest script failed (log: $log)"; }
  echo "capture: $(grep -c '^=== ' "$log") modes captured (log: $log)"
}

main "$@"
