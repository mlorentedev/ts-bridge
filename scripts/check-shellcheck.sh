#!/usr/bin/env bash
# Run shellcheck over this repository's own shell scripts (#562).
#
# Scope, deliberately narrow: `scripts/*.sh`, `scripts/tests/*.sh` and `scripts/e2e/*.sh`, one level each, no
# recursion. Out of scope: shell embedded in workflows (the `actionlint -shellcheck` step
# of the same job covers it), `scripts/tests/helpers/*.bash` (sourced libraries that read
# variables their caller assigns, which shellcheck reports as SC2154), and
# `scripts/tests/fixtures/**` (inputs that are bad on purpose). Adding a script under a
# covered path puts it in scope with no edit here.
#
# `-x` follows `source` directives annotated with `# shellcheck source-path=SCRIPTDIR` to
# resolve what a covered script reads; it reports nothing found inside the sourced helper.
# Each glob that matches nothing fails: an emptied depth must not read as a pass.
#
# Run: bash scripts/check-shellcheck.sh [root]  (default: the repository root)
set -euo pipefail

ROOT="${1:-$(cd "$(dirname "$0")/.." && pwd)}"
cd "$ROOT"

command -v shellcheck >/dev/null || { echo "check-shellcheck: shellcheck is not installed" >&2; exit 1; }

shopt -s nullglob
files=()
for glob in 'scripts/*.sh' 'scripts/tests/*.sh' 'scripts/e2e/*.sh'; do
  # shellcheck disable=SC2206 # the glob must expand here
  matched=($glob)
  [ "${#matched[@]}" -gt 0 ] || { echo "check-shellcheck: no scripts match $ROOT/$glob" >&2; exit 1; }
  files+=("${matched[@]}")
done

shellcheck -x "${files[@]}"
echo "check-shellcheck: OK (${#files[@]} scripts)"
