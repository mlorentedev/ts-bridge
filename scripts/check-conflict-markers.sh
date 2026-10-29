#!/usr/bin/env bash
# Guard every tracked text file against committed merge-conflict markers (#491).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCAN_ROOT="${1:-$ROOT}"

# A bare repository, or a path inside `.git`, answers `false` with exit 0, so test the answer.
if [ "$(git -C "$SCAN_ROOT" rev-parse --is-inside-work-tree 2>/dev/null)" != true ]; then
  echo "check-conflict-markers: $SCAN_ROOT is not a Git work tree" >&2
  exit 2
fi
# A path inside the work tree still means the whole tree: a marker elsewhere ships all the same.
SCAN_ROOT="$(git -C "$SCAN_ROOT" rev-parse --show-toplevel)"

# The open, close and diff3 base markers are matched, at any length (`conflict-marker-size`
# lengthens them). The separator is not: a bare run of `=` is also a valid Markdown setext
# underline, and Git never writes a separator without an open and a close marker around it.
# `-a` reads files Git treats as binary (a `binary` attribute, a NUL byte) as text: skipping
# them would pass a marker silently. `[|]` is a literal pipe with no escape to interpret.
# The marker ends at any whitespace or the line end: a label follows a space (or a tab), and a
# CRLF checkout leaves `\r` after a marker that has no label.
output="$(git -C "$SCAN_ROOT" grep -naE '^(<{7,}|>{7,}|[|]{7,})([[:space:]]|$)' -- . 2>&1)" && rc=0 || rc=$?

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
