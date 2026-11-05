#!/usr/bin/env bash
# Local e2e harness for `ts-bridge host` on throwaway VMs (QA-606, #606).
#
# Usage: scripts/e2e/run.sh linux|windows|clean
#
# Brings a fresh `dotf vm` session up, runs the guest scenario elevated, and
# always tears the session down. Not run in CI: the runners cannot nest a VM.
# See docs/runbooks/e2e-vm.md.
set -euo pipefail

OS="${1:-}"
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
OUT="$HERE/out"
SESSION="tsb-e2e-${OS}"
PREV="${E2E_PREV:-}"        # previous release for the upgrade case; default: latest
KEEP="${E2E_KEEP:-0}"       # 1 keeps the guest for debugging
dir=""

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

main() {
  case "$OS" in
    clean)
      local rc=0
      down linux || rc=1
      down windows || rc=1
      [ "$rc" -eq 0 ] && echo "e2e: no session left"
      exit "$rc"
      ;;
    linux | windows) ;;
    *) die "usage: $0 linux|windows|clean" ;;
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
