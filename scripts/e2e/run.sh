#!/usr/bin/env bash
# Local e2e harness for `ts-bridge host` on throwaway VMs (QA-606, #606).
#
# Usage: scripts/e2e/run.sh linux|windows|clean|headscale-up|headscale-down|mesh linux
#
# Brings a fresh `dotf vm` session up, runs the guest scenario elevated, and
# always tears the session down. Not run in CI: the runners cannot nest a VM.
# See docs/runbooks/e2e-vm.md. headscale-up and headscale-down run the local control plane the
# mesh scenarios use (QA-607); clean removes it too. `mesh linux` runs the Linux guest's
# `host serve` and the host's `connect` through it.
set -euo pipefail

OS="${1:-}"
ARG="${2:-}" # the OS of `mesh`
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
OUT="${E2E_OUT:-$HERE/out}"
SESSION="tsb-e2e-${OS}"
PREV="${E2E_PREV:-}"        # previous release for the upgrade case; default: latest
KEEP="${E2E_KEEP:-0}"       # 1 keeps the guest for debugging
dir=""

# shellcheck source-path=SCRIPTDIR source=headscale.sh
. "$HERE/headscale.sh"

die() { echo "e2e: $*" >&2; exit 2; }

# down removes the session if it exists; a missing one is not an error, but a
# session it cannot list or delete is.
down() {
  local os="$1" s="tsb-e2e-$1" list
  list="$(dotf vm "$os" status 2>/dev/null)" || { echo "e2e: could not list $os sessions" >&2; return 1; }
  if awk '{print $1}' <<<"$list" | grep -qx "$s"; then
    dotf vm "$os" down --session "$s" >/dev/null 2>&1 || { echo "e2e: could not delete session $s" >&2; return 1; }
  fi
}

preflight() {
  command -v dotf >/dev/null || die "dotf not found: install it (dotf >= 0.79 has vm sessions)"
  dotf vm "$OS" up --help >/dev/null 2>&1 || die "this dotf has no 'vm $OS up': upgrade dotf to >= 0.79"
  command -v gh >/dev/null || die "gh not found: needed to download the previous release"
  if [ "$OS" = windows ] && [ ! -f "$HOME/.local/state/dotfiles/vm/windows/clean-base.qcow2" ]; then
    die "no Windows clean-base: run 'dotf vm windows build' once"
  fi
}

# build writes the guest binaries: the working tree as new, the previous
# release (checksums verified) as old.
build() {
  local dir="$1" goos="$OS" ext="" asset tag
  [ "$OS" = windows ] && ext=".exe"
  (cd "$ROOT" && GOOS="$goos" GOARCH=arm64 CGO_ENABLED=0 go build -o "$dir/tsb-new$ext" ./cmd/ts-bridge)
  tag="${PREV:-$(gh release view --repo mlorentedev/ts-bridge --json tagName -q .tagName)}"
  if [ "$OS" = windows ]; then asset="ts-bridge-$tag-windows-arm64.zip"; else asset="ts-bridge-$tag-linux-arm64.tar.gz"; fi
  # Check the asset's own line: --ignore-missing would pass an asset with none.
  (cd "$dir" && gh release download "$tag" --repo mlorentedev/ts-bridge -p "$asset" -p checksums.txt --clobber >/dev/null &&
    grep -E "^[0-9a-f]{64}  $asset\$" checksums.txt | shasum -a 256 -c - >/dev/null) || die "could not verify $asset"
  if [ "$OS" = windows ]; then
    (cd "$dir" && unzip -oq "$asset" && cp ts-bridge-windows-arm64/ts-bridge.exe tsb-old.exe)
  else
    (cd "$dir" && tar xzf "$asset" && cp ts-bridge-linux-arm64/ts-bridge tsb-old)
  fi
  echo "$tag" > "$dir/prev-tag"
}

run_linux() {
  local dir="$1"
  dotf vm linux cp "$dir/tsb-new" "$SESSION:/tmp/tsb-new" >/dev/null
  dotf vm linux cp "$dir/tsb-old" "$SESSION:/tmp/tsb-old" >/dev/null
  dotf vm linux exec --session "$SESSION" --elevated --timeout 20m --script "$HERE/linux.sh" < /dev/null
}

run_windows() {
  local dir="$1"
  dotf vm windows exec --session "$SESSION" -- powershell -NoProfile -Command \
    'New-Item -ItemType Directory -Force C:\tsb | Out-Null' < /dev/null >/dev/null
  dotf vm windows cp "$dir/tsb-new.exe" "$SESSION:C:\\tsb\\tsb-new.exe" >/dev/null
  dotf vm windows cp "$dir/tsb-old.exe" "$SESSION:C:\\tsb\\tsb-old.exe" >/dev/null
  dotf vm windows exec --session "$SESSION" --elevated --timeout 30m --script "$HERE/windows.ps1" < /dev/null
}

# report LOG [RC] counts the result lines and fails on FAIL or XPASS (AC1, AC5).
# It also fails when the scenario exited non-zero (RC) or printed no result:
# a guest script that died early must not read as a green run.
report() {
  local log="$1" rc="${2:-0}" pass fail xfail xpass
  pass=$(grep -c '^PASS ' "$log" || true)
  fail=$(grep -c '^FAIL ' "$log" || true)
  xfail=$(grep -c '^XFAIL ' "$log" || true)
  xpass=$(grep -c '^XPASS ' "$log" || true)
  echo "e2e $OS: $pass pass, $fail fail, $xfail xfail, $xpass xpass (log: $log)"
  [ "$rc" -eq 0 ] || { echo "e2e $OS: the scenario exited $rc before it finished" >&2; return 1; }
  [ $((pass + xfail)) -gt 0 ] || { echo "e2e $OS: no case ran" >&2; return 1; }
  [ "$fail" -eq 0 ] && [ "$xpass" -eq 0 ]
}

# Mesh scenarios (QA-607): the same sessions, with a local Headscale between the client on this
# host and the guest's `host serve`. Both ends carry the two harness-only DERP variables.
MESH_ENV=(TS_DEBUG_USE_DERP_HTTP=1 TS_DEBUG_ALWAYS_USE_DERP=1)

preflight_mesh() {
  command -v dotf >/dev/null || die "dotf not found: install it (dotf >= 0.79 has vm sessions)"
  dotf vm linux up --help >/dev/null 2>&1 || die "this dotf has no 'vm linux up': upgrade dotf to >= 0.79"
  command -v python3 >/dev/null || die "python3 not found: the probe needs it"
  { command -v docker >/dev/null && docker info >/dev/null 2>&1; } || die "no docker engine answers: start colima"
  hs_premise_ok "${E2E_TS_MODDIR:-}" || exit 2
  hs_host_ip >/dev/null || exit 2
}

# mesh_rdp_case DIR reaches the guest's xrdp through the mesh from a client on this host. The
# client key is a 0600 file; nothing secret reaches argv.
mesh_rdp_case() {
  local dir="$1" gip port pid ok=1 i
  gip="$(hs_nodes_json | python3 -c 'import json,sys; print(next(a for n in json.load(sys.stdin) if n["name"] == "tsb-e2e-linux" for a in n["ip_addresses"] if "." in a))')" || return 1
  port="$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')"
  (umask 077 && hs_mint_key tag:e2e-client >"$dir/clientkey") || return 1
  env "${MESH_ENV[@]}" "$dir/tsb-client" connect --control-url "http://$(hs_host_ip):8080" --auth-key-file "$dir/clientkey" \
    --state-dir "$dir/state" --target "$gip:3389" --local-addr "127.0.0.1:$port" --manual-mode --hostname tsb-e2e-client >"$dir/client.log" 2>&1 &
  pid=$!
  for i in $(seq 1 60); do grep -q '^READY ' "$dir/client.log" && break; sleep 1; done
  python3 -I "$HERE/probe.py" x224 "127.0.0.1:$port" >/dev/null 2>&1 && ok=0
  kill "$pid" 2>/dev/null || true
  wait "$pid" 2>/dev/null || true
  [ "$ok" -eq 0 ] || tail -5 "$dir/client.log" | sed 's/^/client: /'
  return "$ok"
}

run_mesh_linux() {
  local dir="$1" url="http://$(hs_host_ip):8080"
  dotf vm linux cp "$dir/tsb-guest" "$SESSION:/tmp/tsb-guest" >/dev/null
  dotf vm linux cp "$HERE/probe.py" "$SESSION:/tmp/probe.py" >/dev/null
  # The guest reads the control URL, then the tagged single-use key, from stdin.
  { echo "$url"; hs_mint_key tag:e2e-host; } |
    dotf vm linux exec --session "$SESSION" --elevated --timeout 20m --script "$HERE/mesh_linux.sh" || return 1
  if mesh_rdp_case "$dir"; then echo "PASS mesh rdp: X.224 confirm through connect"; else echo "FAIL mesh rdp: X.224 confirm through connect"; return 1; fi
}

# teardown_mesh is the EXIT trap of a mesh run. bash runs it when SIGINT or SIGTERM ends the
# run too, so an interrupted run removes the guest and the control plane without a later clean.
teardown_mesh() {
  local rc=$?
  trap - EXIT
  rm -rf "$dir"
  if [ "$KEEP" = 1 ]; then echo "e2e: kept $SESSION and headscale (make e2e-clean removes them)"; exit "$rc"; fi
  down linux || { [ "$rc" != 0 ] || rc=1; }
  hs_down || { [ "$rc" != 0 ] || rc=1; }
  exit "$rc"
}

main_mesh() {
  [ "${1:-}" = linux ] || die "usage: $0 mesh linux (mesh windows is not implemented yet: QA-607 PR 4)"
  OS=linux SESSION=tsb-e2e-linux
  preflight_mesh
  local log
  dir="$(mktemp -d)" # global: the EXIT trap runs after main_mesh returns
  trap teardown_mesh EXIT
  mkdir -p "$OUT"
  log="$OUT/mesh-linux-$(date -u +%Y%m%dT%H%M%SZ).log"
  hs_up
  down linux
  dotf vm linux up --session "$SESSION" >/dev/null
  hs_guest_reaches linux "$SESSION" || exit 1
  (cd "$ROOT" && go build -o "$dir/tsb-client" ./cmd/ts-bridge && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o "$dir/tsb-guest" ./cmd/ts-bridge)
  echo "e2e mesh linux: candidate $(cd "$ROOT" && git rev-parse --short HEAD)"
  # The exit status goes through a file, and a failed log write must not pass, as in main.
  if ! { run_mesh_linux "$dir" 2>&1 && echo 0 >"$dir/rc" || echo $? >"$dir/rc"; } |
    tee "$log" | { grep -E '^(PASS|FAIL|XFAIL|XPASS) ' || true; }; then
    die "could not write the log $log"
  fi
  report "$log" "$(cat "$dir/rc")"
}

main() {
  case "$OS" in
    clean)
      local rc=0
      down linux || rc=1
      down windows || rc=1
      hs_down || rc=1
      [ "$rc" -eq 0 ] && echo "e2e: no session left, no headscale container"
      exit "$rc"
      ;;
    headscale-up) hs_up; exit ;;
    headscale-down) hs_down; exit ;;
    mesh) main_mesh "$ARG"; exit ;;
    linux | windows) ;;
    *) die "usage: $0 linux|windows|clean|headscale-up|headscale-down|mesh linux" ;;
  esac
  preflight
  local log
  dir="$(mktemp -d)" # global: the EXIT trap runs after main returns
  trap 'rm -rf "$dir"' EXIT # until the session exists; replaced below
  mkdir -p "$OUT"
  log="$OUT/$OS-$(date -u +%Y%m%dT%H%M%SZ).log"
  build "$dir"
  down "$OS"
  if [ "$KEEP" = 1 ]; then
    trap 'rm -rf "$dir"; echo "e2e: kept session $SESSION (make e2e-clean removes it)"' EXIT
  else
    # A teardown that fails leaves the guest running: the run must not pass.
    trap 'rc=$?; rm -rf "$dir"; down "$OS" || { [ "$rc" != 0 ] || rc=1; }; exit "$rc"' EXIT
  fi
  dotf vm "$OS" up --session "$SESSION" >/dev/null
  echo "e2e $OS: candidate $(cd "$ROOT" && git rev-parse --short HEAD), previous $(cat "$dir/prev-tag")"
  # The exit status goes through a file: the pipeline would hide it.
  # A failed log write must not pass: tee keeps copying to stdout after one.
  if ! { "run_$OS" "$dir" 2>&1 && echo 0 >"$dir/rc" || echo $? >"$dir/rc"; } |
    tee "$log" | { grep -E '^(PASS|FAIL|XFAIL|XPASS) ' || true; }; then
    die "could not write the log $log"
  fi
  report "$log" "$(cat "$dir/rc")"
}

# Sourced (scripts/tests/test-e2e-report.sh), only the functions are defined.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then main; fi
