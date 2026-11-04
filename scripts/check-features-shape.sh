#!/usr/bin/env bash
# Guard: every spec features.json has the shape `dotf spec init` scaffolds (#563).
#
# The review tooling decodes a features.json as an array of objects, each with exactly the keys
# id, behavior, verification, state and evidence. A file in another shape is still valid JSON:
# it parses, so nothing complains, and the reader then sees a stale or empty feature list (a
# top-level object, CI-345 style) or drops the object it cannot read. This check names every
# such file and says what differs. The values stay free: only the keys and their string type
# are the shape, because `state` and `evidence` are written by the harness, not by the author.
#
# Detect only. Archived specs under specs/archive/ are read like any other and never rewritten:
# a spec that no longer matches is reported, and what to do about it is a decision for its owner.
#
# Usage: check-features-shape.sh [specs-dir]   (default: specs/ at the repository root)
# Exit:  0 every file matches, 1 at least one differs, 2 the check could not run.
#
# Portable: bash and zsh, GNU and BSD find (no -printf), NUL-separated file list so spaces
# cannot split it. jq is the JSON reader (preinstalled on ubuntu-latest, as the PR-Agent
# review-published guard already relies on); without it the check fails closed rather than pass.
set -euo pipefail

command -v jq >/dev/null 2>&1 || { echo "check-features-shape: jq not found, cannot read features.json" >&2; exit 2; }
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
# Paths are reported as given; the default run is from the root so they read `specs/...`.
if [ "$#" -gt 0 ]; then SPECS_DIR="$1"; else cd "$ROOT" && SPECS_DIR=specs; fi
[ -d "$SPECS_DIR" ] || { echo "check-features-shape: $SPECS_DIR is not a directory" >&2; exit 2; }

# One line per deviation. `any(...; . == $k)` rather than `contains`/`index`: those match
# substrings, so a key named `i` would count as the key `id`.
# shellcheck disable=SC2016 # a jq program: `\(...)` is jq string interpolation, not shell
FILTER='
  def want: ["behavior", "evidence", "id", "state", "verification"];
  def keyed($k): any(want[]; . == $k);
  if type != "array" then "top level is \(type), want an array of features"
  elif length == 0 then "empty array, the scaffold has one feature"
  else
    to_entries[] | .key as $i | .value as $f |
    if ($f | type) != "object" then "feature \($i) is \($f | type), want an object"
    else
      ((want - ($f | keys_unsorted))[] | "feature \($i) is missing key \(.)"),
      ((($f | keys_unsorted) - want)[] | "feature \($i) has unexpected key \(.)"),
      ($f | to_entries[] | select(keyed(.key) and (.value | type) != "string") |
        "feature \($i): \(.key) is \(.value | type), want string")
    end
  end'

# Findings for one file on stdout. Exactly one JSON document is required: `jq .` alone exits 0
# on an empty file, and the reader rejects both an empty file and trailing documents.
findings() {
  if ! jq -e -s 'length == 1' "$1" >/dev/null 2>&1; then
    echo "not valid JSON (want exactly one document)"
    return 0
  fi
  jq -r "$FILTER" "$1"
}

# The list goes to a file, not a process substitution: that would discard find's exit status,
# and a subtree find could not read would then be reported as a clean tree.
list="$(mktemp)"
trap 'rm -f "$list"' EXIT
find "$SPECS_DIR" -type f -name features.json -print0 >"$list" ||
  { echo "check-features-shape: find failed under $SPECS_DIR, the report would be incomplete" >&2; exit 2; }

# Exempt, by path under the specs dir, with the reason printed beside it (#574). CI-345 was
# archived in a top-level-object shape before this check existed, and an archive is never
# rewritten (#563). A path, not a spec name: the same file anywhere else is still checked.
exempt_reason() {
  case "${1#"$SPECS_DIR"/}" in
    archive/CI-345-hygiene-guard-fixes/features.json) echo "archived before the scaffold shape was checked; archives are not rewritten (#563)" ;;
  esac
}

total=0
bad=0
while IFS= read -r -d '' f; do
  total=$((total + 1))
  out="$(findings "$f")"
  [ -n "$out" ] || continue
  reason="$(exempt_reason "$f")"
  if [ -n "$reason" ]; then
    printf '  %s: exempt, %s\n' "$f" "$reason"
    continue
  fi
  bad=$((bad + 1))
  printf '%s\n' "$out" | while IFS= read -r line; do printf '  %s: %s\n' "$f" "$line"; done
done <"$list"

if [ "$bad" -eq 0 ]; then
  echo "check-features-shape: OK ($total features.json files)"
  exit 0
fi
echo "check-features-shape: $bad of $total features.json files differ from the scaffold (an array of objects with id, behavior, verification, state, evidence)"
exit 1
