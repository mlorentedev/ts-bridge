#!/usr/bin/env bash
# Guard every tracked text file against committed merge-conflict markers (#491).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCAN_ROOT="${1:-$ROOT}"

if ! git -C "$SCAN_ROOT" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "check-conflict-markers: $SCAN_ROOT is not a Git work tree" >&2
  exit 2
fi

# Only the open and close markers are matched. Git writes both around every separator (and
# around a diff3 base), so they cover each real conflict, while a bare run of seven `=` is also
# a valid Markdown setext underline.
output="$(git -C "$SCAN_ROOT" grep -nIE '^(<<<<<<<|>>>>>>>)( |$)' -- . 2>&1)" && rc=0 || rc=$?

case "$rc" in
  1)
    echo "check-conflict-markers: OK ($(git -C "$SCAN_ROOT" ls-files | awk 'END { print NR }') tracked files)"
    ;;
  0)
    printf '%s\n' "$output"
    echo "check-conflict-markers: $(printf '%s\n' "$output" | awk 'END { print NR }') marker line(s); resolve the conflict before committing"
    exit 1
    ;;
  *)
    printf '%s\n' "$output" >&2
    echo "check-conflict-markers: git grep failed (exit $rc)" >&2
    exit 2
    ;;
esac
