#!/usr/bin/env bash
# Tests for scripts/check-workflow-permissions.sh -- the guard that keeps CI credentials off.
#
# Why fixtures instead of asserting on this repo's own workflows: a guard pointed at one real
# tree only ever proves "today it is quiet". These cases prove the definition still refuses each
# way the posture can widen -- no permissions key, a checkout that persists the token, an
# explicit true, and a `contents: write-all` that reads like hardening and is the opposite. The
# clean fixture is the control that stops a guard which fails everything, and the opt-out fixture
# proves the escape hatch stays visible rather than silent. Two more fixtures say the negative of
# that: a commented-out checkout and a comment that mentions `write-all` are prose, not structure,
# and must not turn a clean repository red -- which the first version did, on line 8 of a valid
# workflow (PR-Agent found it; reproduced before fixing). Three more cover quoting, the other half
# of the same lesson: a matcher that knows only the unquoted spelling is not a rule about
# permissions, it is a rule about the spellings this file happens to recognise --
# `permissions: "write-all"` walked through the first version too (CodeRabbit, #336).
#
# The CI-322 adversarial review found the same lesson one level up, in structure rather than
# quoting: a step whose `uses:` is not its first key (`checkout-not-first`, `with-before-uses`)
# was a false red, and `write-all` written as a block scalar or on the line after its key
# (`block-scalar-write-all`, `next-line-write-all`) was a false green. `persist-capital-false`
# covers YAML's case-insensitive booleans. `name-first-persists` and `block-scalar-read-all` are
# the controls that stop the new step and scalar handling from passing everything.
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
named-opt-out:0:opt-out
commented-checkout:0:OK
commented-write-all:0:OK
quoted-write-all:1:write-all
quoted-checkout:1:no-except
quoted-false:0:OK
checkout-not-first:0:OK
with-before-uses:0:OK
name-first-persists:1:no-except
block-scalar-write-all:1:write-all
next-line-write-all:1:write-all
block-scalar-read-all:0:OK
persist-capital-false:0:OK
crlf-hardened:0:OK
checkout-text-in-run-block:0:OK
write-all-inside-string:0:OK
flow-style-hardened:0:OK
flow-style-persists:1:no-except
anchored-write-all:1:write-all
tagged-write-all:1:write-all
run-block-then-sibling-key:1:no-except'

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
    commented-checkout) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      # - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
      - run: echo hi
YAML
    ;;
    commented-write-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read   # the sibling repos solved this as: write-all never, and this is prose
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
YAML
    ;;
    quoted-write-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions: "write-all"
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
YAML
    ;;
    quoted-checkout) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - uses: "actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5" # v7
      - run: echo hi
YAML
    ;;
    quoted-false) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: "false"
YAML
    ;;
    checkout-not-first) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - name: Checkout
        uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
      - run: echo hi
YAML
    ;;
    with-before-uses) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - name: Checkout
        with:
          persist-credentials: false
        uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
      - run: echo hi
YAML
    ;;
    name-first-persists) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - name: Checkout
        uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
      - name: Build
        run: echo hi
YAML
    ;;
    block-scalar-write-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions: >-
  write-all
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
YAML
    ;;
    next-line-write-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  write-all
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
YAML
    ;;
    block-scalar-read-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions: >-
  read-all
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: false
YAML
    ;;
    persist-capital-false) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
        with:
          persist-credentials: False
YAML
    ;;
    crlf-hardened) printf '%s\r\n' 'name: t' 'on: push' 'permissions:' '  contents: read' 'jobs:' '  a:' '    steps:' '      - uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7' '        with:' '          persist-credentials: false' > "$1/w.yml"
    ;;
    checkout-text-in-run-block) cat > "$1/w.yml" <<'YAML'
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
      - run: |
          cat <<EOF
          - uses: actions/checkout@v4
          EOF
YAML
    ;;
    write-all-inside-string) cat > "$1/w.yml" <<'YAML'
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
      - run: echo "$MSG"
        env:
          MSG: "never grant contents: write-all"
YAML
    ;;
    flow-style-hardened) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - {uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5, with: {persist-credentials: false}}
YAML
    ;;
    flow-style-persists) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - {uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5}
YAML
    ;;
    anchored-write-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions: &p write-all
jobs:
  a:
    steps:
      - run: echo
YAML
    ;;
    tagged-write-all) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions: !!str write-all
jobs:
  a:
    steps:
      - run: echo
YAML
    ;;
    run-block-then-sibling-key) cat > "$1/w.yml" <<'YAML'
name: t
on: push
permissions:
  contents: read
jobs:
  a:
    steps:
      - name: Checkout
        run: |
          echo before
        uses: actions/checkout@028fa82250a01b7d398affb59a3b85b679bb34d5 # v7
YAML
    ;;
    *) echo "test-workflow-permissions: no fixture named $2" >&2; return 3 ;;
  esac
}

# The shells are a contract, so a missing one is a failure, not a skip. The first version
# printed "zsh not installed, that shell is UNTESTED" and exited 0 -- and the CI runner has no
# zsh, so from #336 on the job certified a two-shell guard with one shell, and the exact
# `set -- $sum` regression this matrix exists for would have passed green (CI-322 adversarial
# review, Major). TWP_SHELLS narrows the set on purpose, visibly; it never happens by absence.
SHELLS="${TWP_SHELLS:-bash zsh}"
failed=0
for sh_name in $SHELLS; do
  if ! command -v "$sh_name" >/dev/null 2>&1; then
    echo "  FAIL [$sh_name] not installed: the guard is contracted to run under it, so it cannot be certified here" >&2
    failed=1
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
# Self-test of the rule above: with a required shell absent, this suite must not exit 0.
if [ -z "${TWP_NESTED:-}" ]; then
  if TWP_NESTED=1 TWP_SHELLS="bash no-such-shell-for-test" bash "$0" >/dev/null 2>&1; then
    echo "  FAIL missing-shell: the suite exited 0 with a required shell absent"; failed=1
  else
    echo "  ok   missing-shell -> non-zero when a required shell is absent"
  fi
fi
exit "$failed"
