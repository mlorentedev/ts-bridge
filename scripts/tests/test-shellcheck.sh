#!/usr/bin/env bash
# Fixture suite for scripts/check-shellcheck.sh (#562): the gate fails on a script that
# the linter rejects, at both depths it covers, and fails closed when it has nothing to
# check, so an emptied scope cannot read as a pass.
#
# Run: bash scripts/tests/test-shellcheck.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-shellcheck.sh"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# A tree with one clean script at each covered depth, in $1.
make_tree() {
  mkdir -p "$1/scripts/tests"
  cat >"$1/scripts/clean.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
echo "$HOME"
SH
  cp "$1/scripts/clean.sh" "$1/scripts/tests/clean-test.sh"
}

# SC2086: an unquoted expansion, written to the path in $1.
write_bad() {
  cat >"$1" <<'SH'
#!/usr/bin/env bash
echo $HOME
SH
}

expect() { # name want_exit root
  local out status=0
  out="$(bash "$GUARD" "$3" 2>&1)" || status=$?
  if [ "$status" -ne "$2" ]; then
    echo "test-shellcheck: $1: want exit $2, got $status" >&2
    printf '%s\n' "$out" >&2
    exit 1
  fi
}

make_tree "$tmp/clean"
expect "clean tree passes" 0 "$tmp/clean"

make_tree "$tmp/top"
write_bad "$tmp/top/scripts/bad.sh"
expect "violation in scripts/*.sh fails" 1 "$tmp/top"

make_tree "$tmp/tests"
write_bad "$tmp/tests/scripts/tests/bad-test.sh"
expect "violation in scripts/tests/*.sh fails" 1 "$tmp/tests"

# Out of scope by decision: fixtures hold deliberately bad inputs.
make_tree "$tmp/fixture"
mkdir -p "$tmp/fixture/scripts/tests/fixtures/x"
write_bad "$tmp/fixture/scripts/tests/fixtures/x/bad.sh"
expect "fixtures are not scanned" 0 "$tmp/fixture"

mkdir -p "$tmp/empty/scripts/tests"
expect "an empty scope fails closed" 1 "$tmp/empty"

# The real tree is clean, which is what the CI step asserts.
expect "the repository scripts pass" 0 "$ROOT"
echo "test-shellcheck: OK"
