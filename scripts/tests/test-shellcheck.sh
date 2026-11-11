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
  mkdir -p "$1/scripts/tests" "$1/scripts/e2e"
  cat >"$1/scripts/clean.sh" <<'SH'
#!/usr/bin/env bash
set -euo pipefail
echo "$HOME"
SH
  cp "$1/scripts/clean.sh" "$1/scripts/tests/clean-test.sh"
  cp "$1/scripts/clean.sh" "$1/scripts/e2e/clean-e2e.sh"
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

make_tree "$tmp/e2e"
write_bad "$tmp/e2e/scripts/e2e/bad-e2e.sh"
expect "violation in scripts/e2e/*.sh fails" 1 "$tmp/e2e"

# Out of scope by decision: fixtures hold deliberately bad inputs.
make_tree "$tmp/fixture"
mkdir -p "$tmp/fixture/scripts/tests/fixtures/x"
write_bad "$tmp/fixture/scripts/tests/fixtures/x/bad.sh"
expect "fixtures are not scanned" 0 "$tmp/fixture"

# `-x` resolves a sourced helper without reporting on it: helpers stay out of scope.
make_tree "$tmp/helper"
mkdir -p "$tmp/helper/scripts/tests/helpers"
write_bad "$tmp/helper/scripts/tests/helpers/bad.bash"
printf '#!/usr/bin/env bash\n# shellcheck source-path=SCRIPTDIR\nsource helpers/bad.bash\n' \
  >"$tmp/helper/scripts/tests/uses-helper.sh"
expect "a sourced helper is not reported" 0 "$tmp/helper"

mkdir -p "$tmp/empty/scripts/tests"
expect "an empty scope fails closed" 1 "$tmp/empty"

# Each covered depth fails closed on its own: one populated glob must not hide an empty one.
make_tree "$tmp/no-tests"
rm "$tmp/no-tests/scripts/tests/clean-test.sh"
expect "an empty scripts/tests fails closed" 1 "$tmp/no-tests"

make_tree "$tmp/no-e2e"
rm "$tmp/no-e2e/scripts/e2e/clean-e2e.sh"
expect "an empty scripts/e2e fails closed" 1 "$tmp/no-e2e"

make_tree "$tmp/no-top"
rm "$tmp/no-top/scripts/clean.sh"
expect "an empty scripts/ fails closed" 1 "$tmp/no-top"

# The real tree is clean, which is what the CI step asserts.
expect "the repository scripts pass" 0 "$ROOT"
echo "test-shellcheck: OK"
