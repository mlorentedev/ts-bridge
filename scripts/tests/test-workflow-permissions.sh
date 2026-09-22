#!/usr/bin/env bash
# Tests for scripts/check-workflow-permissions.sh -- the guard that keeps CI credentials off.
#
# Why fixtures instead of asserting on this repo's own workflows: a guard pointed at one real
# tree only ever proves "today it is quiet". These cases prove the definition still refuses each
# way the posture can widen -- no permissions key, a checkout that persists the token, an
# explicit true, and a `contents: write-all` that reads like hardening and is the opposite. The
# clean fixture is the control that stops a guard which fails everything, and the opt-out fixture
# proves the escape hatch stays visible rather than silent.
#
# And why two shells: the first version of this guard passed under bash and died under zsh with
# "cannot parse guard output" on a clean repository, because `set -- $sum` does not word-split
# where zsh is concerned. A guard that fails differently per shell is a guard that gets deleted,
# so every fixture runs under both.
#
# Run: bash scripts/tests/test-workflow-permissions.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-workflow-permissions.sh"
[ -f "$GUARD" ] || { echo "test-workflow-permissions: guard not found at $GUARD" >&2; exit 2; }

# name:expected_exit:expected_substring
SPECS='clean:0:OK
missing-key:1:no-perms
persists-token:1:no-except
explicit-true:1:enabled
write-all:1:write-all
named-opt-out:0:opt-out'

# Each fixture is one workflow, written as data rather than as a mutation of another: the diff
# between a passing and a failing case should be readable, not derivable.
write_fixture() {                     # write_fixture <dir> <name>
  mkdir -p "$1"
  case "$2" in
    clean) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
      - run: echo hi
YAML
    ;;
    missing-key) cat > "$1/w.yml" <<'YAML'
name: t
on: push
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
YAML
    ;;
    persists-token) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
      - run: echo hi
YAML
    ;;
    explicit-true) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: true
YAML
    ;;
    write-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: write-all
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
YAML
    ;;
    named-opt-out) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  release:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7  # persist-credentials-ok: the job pushes the release tag
      - run: git push --tags
YAML
    ;;
    *) echo "test-workflow-permissions: no fixture named $2" >&2; return 3 ;;
  esac
}

failed=0
for sh_name in bash zsh; do
  if ! command -v "$sh_name" >/dev/null 2>&1; then
    echo "test-workflow-permissions: $sh_name not installed, that shell is UNTESTED"
    continue
  fi
  printf '%s\n' "$SPECS" | while IFS= read -r spec; do
    [ -n "$spec" ] || continue
    name="${spec%%:*}"; rest="${spec#*:}"; want="${rest%%:*}"; want_sub="${rest#*:}"
    dir="$(mktemp -d)"
    write_fixture "$dir" "$name"
    got="$("$sh_name" "$GUARD" "$dir" 2>&1)" && rc=0 || rc=$?
    rm -rf "$dir"
    if [ "$rc" -ne "$want" ]; then
      echo "  FAIL [$sh_name] $name: exit $rc, wanted $want"; echo "$got"; exit 1
    fi
    case "$got" in
      *"$want_sub"*) ;;
      *) echo "  FAIL [$sh_name] $name: output does not mention '$want_sub'"; echo "$got"; exit 1 ;;
    esac
    echo "  ok   [$sh_name] $name -> exit $rc, mentions \"$want_sub\""
  done
done
exit "$failed"
