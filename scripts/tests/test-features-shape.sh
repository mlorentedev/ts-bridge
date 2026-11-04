#!/usr/bin/env bash
# Tests for scripts/check-features-shape.sh -- the check that names every spec features.json
# whose shape differs from the scaffold `dotf spec init` writes (#563).
#
# The consumer is the review tooling, which decodes the file as an array of objects. So each
# fixture is judged by what that reader needs, not by whether the file parses: a top-level
# object (CI-345's shape) is valid JSON and unreadable as a feature list. `clean` and
# `reordered-keys-passing` are the controls that stop a check which fails everything; the
# `only-bad-named` and `every-bad-named` cases prove the report points at the files that differ
# and at no other.
#
# Each case builds a throwaway specs tree, so no committed file has to be deliberately wrong.
# Every case runs under bash and zsh: a guard that fails differently per shell gets deleted.
#
# Run: bash scripts/tests/test-features-shape.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-features-shape.sh"
[ -f "$GUARD" ] || { echo "test-features-shape: guard not found at $GUARD" >&2; exit 2; }

WORK="$(mktemp -d)"
trap 'chmod -R u+rwx "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

OBJ='{"id":"X-f1","behavior":"b","verification":"true","state":"pending","evidence":""}'

put() { # put <case> <path under specs/> <content>
  mkdir -p "$WORK/$1/specs/$(dirname "$2")"
  printf '%s\n' "$3" >"$WORK/$1/specs/$2"
}

# One flat fixture table: each arm is straight-line setup with no shared branching, so it stays
# in one place rather than following the 40-line / CC-10 rule.
build() {
  mkdir -p "$WORK/$1/specs"
  case "$1" in
    clean) put "$1" A/features.json "[$OBJ]" ;;
    no-files) ;;
    reordered-keys-passing)
      put "$1" A/features.json '[{"evidence":"exit 0","state":"passing","verification":"true","behavior":"b","id":"A-f1"},'"$OBJ"']'
      ;;
    archived-clean) put "$1" archive/A/features.json "[$OBJ]" ;;
    path-with-space) put "$1" "A b/features.json" '{"feature_id":"A"}' ;;
    crlf-clean) put "$1" A/features.json "$(printf '[\r\n  %s\r\n]\r' "$OBJ")" ;;
    top-level-object) put "$1" A/features.json '{"feature_id":"A","acceptance_criteria":[]}' ;;
    top-level-string) put "$1" A/features.json '"not a list"' ;;
    empty-array) put "$1" A/features.json '[]' ;;
    scalar-element) put "$1" A/features.json '["a string"]' ;;
    null-element) put "$1" A/features.json "[$OBJ,null]" ;;
    nested-array-element) put "$1" A/features.json '[[]]' ;;
    missing-key) put "$1" A/features.json '[{"id":"A-f1","behavior":"b","verification":"true","state":"pending"}]' ;;
    extra-key) put "$1" A/features.json '[{"id":"A-f1","behavior":"b","verification":"true","state":"pending","evidence":"","notes":"n"}]' ;;
    renamed-key) put "$1" A/features.json '[{"id":"A-f1","description":"b","verification":"true","state":"pending","evidence":""}]' ;;
    null-value) put "$1" A/features.json '[{"id":"A-f1","behavior":"b","verification":"true","state":"pending","evidence":null}]' ;;
    number-value) put "$1" A/features.json '[{"id":1,"behavior":"b","verification":"true","state":"pending","evidence":""}]' ;;
    second-object-bad) put "$1" A/features.json "[$OBJ,{\"id\":\"A-f2\"}]" ;;
    invalid-json) put "$1" A/features.json '[{"id":' ;;
    empty-file) put "$1" A/features.json '' ;;
    archived-bad) put "$1" archive/A/features.json '{"feature_id":"A"}' ;;
    # The one named exemption (#574): reported as exempt, not as a difference.
    exempt-ci-345) put "$1" archive/CI-345-hygiene-guard-fixes/features.json '{"feature_id":"CI-345"}' ;;
    # The exemption is a path, not a spec name: the same file outside archive/ still fails.
    exempt-name-elsewhere) put "$1" CI-345-hygiene-guard-fixes/features.json '{"feature_id":"CI-345"}' ;;
    # An exemption skips its own file only: a difference elsewhere in the same tree still fails.
    exempt-beside-bad)
      put "$1" archive/CI-345-hygiene-guard-fixes/features.json '{"feature_id":"CI-345"}'
      put "$1" zz-after/features.json '[]'
      ;;
    only-bad-named)
      put "$1" good/features.json "[$OBJ]"
      put "$1" bad/features.json '{"feature_id":"B"}'
      ;;
    every-bad-named)
      put "$1" one/features.json '{"feature_id":"1"}'
      put "$1" archive/two/features.json '[]'
      put "$1" three/features.json '[{"id":"3"}]'
      ;;
    # find cannot read the subtree, so a clean report would cover files it never saw.
    unreadable-dir)
      put "$1" A/features.json "[$OBJ]"
      mkdir "$WORK/$1/specs/locked"
      chmod 000 "$WORK/$1/specs/locked"
      ;;
    not-a-dir)
      rmdir "$WORK/$1/specs"
      : >"$WORK/$1/specs"
      ;;
  esac
}

# name:expected_exit:required substring
SPECS='clean:0:OK (1 features.json
no-files:0:OK (0 features.json
reordered-keys-passing:0:OK (1 features.json
archived-clean:0:OK (1 features.json
path-with-space:1:A b/features.json
crlf-clean:0:OK (1 features.json
top-level-object:1:top level is object
top-level-string:1:top level is string
empty-array:1:empty array
scalar-element:1:feature 0 is string
null-element:1:feature 1 is null
nested-array-element:1:feature 0 is array
missing-key:1:feature 0 is missing key evidence
extra-key:1:feature 0 has unexpected key notes
renamed-key:1:feature 0 is missing key behavior
null-value:1:feature 0: evidence is null
number-value:1:feature 0: id is number
second-object-bad:1:feature 1 is missing key behavior
invalid-json:1:not valid JSON
empty-file:1:not valid JSON
archived-bad:1:archive/A/features.json
exempt-ci-345:0:exempt, archived before the scaffold shape was checked
exempt-name-elsewhere:1:top level is object
exempt-beside-bad:1:zz-after/features.json: empty array
only-bad-named:1:bad/features.json
every-bad-named:1:3 of 3 features.json
not-a-dir:2:not a directory'
# Permissions do not bind root, so the unreadable-directory case cannot fail there.
if [ "$(id -u)" -ne 0 ]; then SPECS="$SPECS
unreadable-dir:2:find failed"; fi

# name:substring that must NOT appear in the output
ABSENT='only-bad-named:good/features.json'

# name:substring that must ALSO appear: a report names every file that differs, not the first
ALSO='every-bad-named:one/features.json
every-bad-named:archive/two/features.json
every-bad-named:three/features.json'

failed=0
count=0

# One pass of every case under one shell.
run_cases() { # run_cases <shell>
  local sh_name="$1" spec name rest want want_sub got rc also absent
  if ! command -v "$sh_name" >/dev/null 2>&1; then
    echo "  FAIL [$sh_name] not installed: both shells are required" >&2
    failed=1
    return 0
  fi
  while IFS= read -r spec; do
    name="${spec%%:*}"
    rest="${spec#*:}"
    want="${rest%%:*}"
    want_sub="${rest#*:}"
    [ -d "$WORK/$name" ] || build "$name"
    got="$("$sh_name" "$GUARD" "$WORK/$name/specs" 2>&1)" && rc=0 || rc=$?
    count=$((count + 1))
    if [ "$rc" -ne "$want" ]; then
      echo "  FAIL [$sh_name] $name: exit $rc, wanted $want"; echo "$got"; failed=1; continue
    fi
    case "$got" in
      *"$want_sub"*) echo "  ok   [$sh_name] $name -> exit $rc, mentions \"$want_sub\"" ;;
      *) echo "  FAIL [$sh_name] $name: output does not mention '$want_sub'"; echo "$got"; failed=1 ;;
    esac
    while IFS= read -r also; do
      [ -n "$also" ] || continue
      case "$got" in
        *"$also"*) ;;
        *) echo "  FAIL [$sh_name] $name: output does not name '$also'"; echo "$got"; failed=1 ;;
      esac
    done <<EOF
$(printf '%s\n' "$ALSO" | awk -F: -v n="$name" '$1 == n { print $2 }')
EOF
    absent="$(printf '%s\n' "$ABSENT" | awk -F: -v n="$name" '$1 == n { print $2 }')"
    case "$got" in
      *"${absent:-@@none@@}"*) echo "  FAIL [$sh_name] $name: output mentions '$absent'"; echo "$got"; failed=1 ;;
    esac
  done <<EOF
$SPECS
EOF
}

# zsh does not word-split an unquoted expansion, so the shell list is read line by line.
while IFS= read -r sh_name; do
  run_cases "$sh_name"
done <<EOF
$(printf '%s\n' "${FJS_SHELLS:-bash zsh}" | tr ' ' '\n')
EOF

# Detect only: a file the check reports is read, never rewritten (archived specs included).
before="$(cat "$WORK/archived-bad/specs/archive/A/features.json")"
bash "$GUARD" "$WORK/archived-bad/specs" >/dev/null 2>&1 || true
after="$(cat "$WORK/archived-bad/specs/archive/A/features.json")"
if [ "$before" != "$after" ]; then echo "  FAIL archived features.json was modified"; failed=1; fi

# Fail closed without jq: no JSON reader must be an error, never a clean tree.
empty_path="$WORK/empty-path"
mkdir -p "$empty_path"
got="$(PATH="$empty_path" "$(command -v bash)" "$GUARD" "$WORK/clean/specs" 2>&1)" && rc=0 || rc=$?
case "$rc:$got" in
  2:*jq*) echo "  ok   [bash] without jq -> exit 2" ;;
  *) echo "  FAIL without jq: exit $rc, wanted 2 naming jq"; echo "$got"; failed=1 ;;
esac

if [ "$failed" -eq 0 ]; then echo "test-features-shape: OK ($count cases)"; fi
exit "$failed"
