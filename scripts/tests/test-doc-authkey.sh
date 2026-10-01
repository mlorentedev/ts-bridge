#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-doc-authkey.sh"
FIXTURES="$ROOT/scripts/tests/fixtures/doc-authkey"
[ -f "$GUARD" ] || { echo "test-doc-authkey: guard not found at $GUARD" >&2; exit 2; }

SPECS='clean:0:OK
prose-only:0:OK
connect-inline:1:inline --auth-key
connect-equals:1:inline --auth-key
connect-multiline:1:inline --auth-key
connect-powershell-multiline:1:inline --auth-key
init-warning:0:OK
init-no-warning:1:init --auth-key example has no nearby process-table warning'

failed=0
for sh_name in ${TDA_SHELLS:-bash zsh}; do
  if ! command -v "$sh_name" >/dev/null 2>&1; then
    echo "  FAIL [$sh_name] not installed: both shells are required" >&2
    failed=1
    continue
  fi

  while IFS= read -r spec; do
    [ -n "$spec" ] || continue
    name="${spec%%:*}"
    rest="${spec#*:}"
    want="${rest%%:*}"
    want_sub="${rest#*:}"
    got="$("$sh_name" "$GUARD" "$FIXTURES/$name" 2>&1)" && rc=0 || rc=$?

    if [ "$rc" -ne "$want" ]; then
      echo "  FAIL [$sh_name] $name: exit $rc, wanted $want"
      echo "$got"
      failed=1
      continue
    fi
    case "$got" in
      *"$want_sub"*) echo "  ok   [$sh_name] $name -> exit $rc, mentions \"$want_sub\"" ;;
      *)
        echo "  FAIL [$sh_name] $name: output does not mention '$want_sub'"
        echo "$got"
        failed=1
        ;;
    esac
  done <<EOF
$SPECS
EOF
done
exit "$failed"
