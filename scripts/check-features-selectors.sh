#!/usr/bin/env bash
# Guard: every `go test -run` selector in an active spec's features.json still names a test (#575).
#
# A feature's `verification` is usually `go test ./pkg -run '<alternation>'`. `go test` exits 0
# when the selector matches nothing ("no tests to run"), and the harness rule "exit 0 means the
# feature may be set passing" then accepts a command that checks nothing. The more alternatives a
# selector has, the easier one is to lose: renaming one row leaves the others green (BUG-530 f3
# has five). So each top-level alternative is checked on its own, not the selector as a whole.
#
# Static, not a run: the commands are never executed (some build sites, some start VMs). The
# selector is matched against the Test, Fuzz and Example functions declared in the named
# packages' *_test.go files (-run never selects a Benchmark; an Example without an Output comment
# is compiled but not run, which this static read cannot tell apart), whatever their build tags, so a test that exists only for
# another OS counts. Reading the declarations is what makes the check cheap enough for CI.
#
# Reads: only `go test` segments that carry -run, with ./relative package paths (`./pkg`,
# `./pkg/...`; no path means the current directory). For `A/sub` only the first element is
# checked: subtests are named at run time. An alternation inside a group, `^(A|B)$`, is one
# alternative: its stale half is not seen. The pattern is read as an extended regular expression,
# which covers the Go syntax these selectors use; one grep cannot parse is reported, not skipped.
#
# Active specs only: archived specs under specs/archive/ were true when they closed and are never
# rewritten. Detect only: a command that fails is reported, and what to do is its owner's call.
#
# Usage: check-features-selectors.sh [specs-dir]   (default: specs/ at the repository root)
#        Package paths resolve against the directory above specs-dir.
# Exit:  0 every selector matches, 1 at least one selects nothing, 2 the check could not run.
#
# Portable: bash and zsh, GNU and BSD find/awk/grep, NUL-separated file list so spaces cannot
# split it. jq reads the JSON; without it the check fails closed rather than pass.
set -euo pipefail

command -v jq >/dev/null 2>&1 || { echo "check-features-selectors: jq not found, cannot read features.json" >&2; exit 2; }
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
if [ "$#" -gt 0 ]; then SPECS_DIR="$1"; else cd "$ROOT" && SPECS_DIR=specs; fi
[ -d "$SPECS_DIR" ] || { echo "check-features-selectors: $SPECS_DIR is not a directory" >&2; exit 2; }
SPECS_DIR="${SPECS_DIR%/}"
BASE="$(cd "$SPECS_DIR/.." && pwd)"

# Reads one command per line; prints "<1 for a new selector, else 0>\t<package paths>\t<alternative>"
# for each `go test` segment with a non-empty -run. A tokenizer, not a regex: a selector holds `|`
# and `&` inside quotes, and the same characters outside quotes separate commands.
# shellcheck disable=SC2016 # an awk program; sq is the single quote, passed in with -v
SELECTORS='
function flush_token() {
  if (intoken) { tok[++ntok] = buf; buf = ""; intoken = 0 }
}
function flush_segment(   i, g, pkgs, run, t, hasrun) {
  flush_token()
  g = 0
  for (i = 1; i < ntok; i++) if (tok[i] == "go" && tok[i + 1] == "test") { g = i; break }
  if (g > 0) {
    pkgs = ""; run = ""; hasrun = 0
    for (i = g + 2; i <= ntok; i++) {
      t = tok[i]
      if (t == "-run" || t == "-test.run") { run = tok[i + 1]; hasrun = 1; i++ }
      else if (t ~ /^-(test\.)?run=/) { run = substr(t, index(t, "=") + 1); hasrun = 1 }
      else if (t ~ /^\.(\/|$)/) pkgs = pkgs (pkgs == "" ? "" : " ") t
    }
    if (hasrun && run != "") emit(pkgs == "" ? "." : pkgs, run)
  }
  ntok = 0
}
# The first "/" outside brackets and groups cuts the subtest levels; the first level is split
# on "|" outside brackets and groups.
function emit(pkgs, run,   i, c, depth, cls, alt, new) {
  depth = 0; cls = 0; alt = ""; new = 1
  for (i = 1; i <= length(run); i++) {
    c = substr(run, i, 1)
    if (c == "\\") { alt = alt c substr(run, i + 1, 1); i++; continue }
    if (cls) { if (c == "]") cls = 0; alt = alt c; continue }
    if (c == "[") cls = 1
    else if (c == "(") depth++
    else if (c == ")") depth--
    else if (c == "/" && depth == 0) break
    else if (c == "|" && depth == 0) { print new "\t" pkgs "\t" alt; new = 0; alt = ""; continue }
    alt = alt c
  }
  print new "\t" pkgs "\t" alt
}
{
  ntok = 0; buf = ""; intoken = 0; q = ""
  line = $0 "\n"
  for (i = 1; i <= length(line); i++) {
    c = substr(line, i, 1)
    if (q == sq) { if (c == q) q = ""; else buf = buf c; continue }
    if (q == "\"") {
      n = substr(line, i + 1, 1)
      if (c == "\\" && (n == "\"" || n == "\\" || n == "$" || n == "`")) { buf = buf n; i++ }
      else if (c == q) q = ""
      else buf = buf c
      continue
    }
    if (c == sq || c == "\"") { q = c; intoken = 1; continue }
    if (c == "\\") { buf = buf substr(line, i + 1, 1); intoken = 1; i++; continue }
    if (c == "&" || c == ";" || c == "|" || c == "\n") { flush_segment(); continue }
    if (c == " " || c == "\t") { flush_token(); continue }
    buf = buf c; intoken = 1
  }
  flush_segment()
}'

# Test, Fuzz and Example function names declared in the *_test.go files of one package
# argument (a trailing /... takes the subtree). Fails when the directory does not exist.
declared_tests() { # declared_tests <pkg path>
  local p="$1" dir f
  if [ "${p%/...}" != "$p" ]; then dir="$BASE/${p%/...}"; else dir="$BASE/$p"; fi
  [ -d "$dir" ] || return 1
  # find, not a glob: zsh aborts on a pattern that matches nothing, bash passes it through.
  while IFS= read -r f; do
    { grep -E '^func (Test|Fuzz|Example)[A-Za-z0-9_]*\(' "$f" || true; } |
      sed -E 's/^func ([A-Za-z0-9_]+)\(.*/\1/'
  done <<EOF
$(if [ "${p%/...}" != "$p" ]; then find "$dir" -type f -name '*_test.go'; else find "$dir" -maxdepth 1 -type f -name '*_test.go'; fi)
EOF
}

# The names declared in every package of a space-separated list; "no such package: P" on stderr
# and a non-zero status when one is missing.
declared_in() { # declared_in <pkgs>
  local p
  while IFS= read -r p; do
    [ -n "$p" ] || continue
    declared_tests "$p" || { echo "no such package: $p" >&2; return 1; }
  done <<EOF
$(printf '%s\n' "$1" | tr ' ' '\n')
EOF
}

list="$(mktemp)"
trap 'rm -f "$list"' EXIT
find "$SPECS_DIR" -path "$SPECS_DIR/archive" -prune -o -type f -name features.json -print0 >"$list" ||
  { echo "check-features-selectors: find failed under $SPECS_DIR, the report would be incomplete" >&2; exit 2; }
LC_ALL=C sort -z -o "$list" "$list"

selectors=0
bad=0

# One alternative of one selector: it must match a declared test in at least one of its packages.
check_alternative() { # check_alternative <file> <id> <pkgs> <alt>
  local file="$1" id="$2" pkgs="$3" alt="$4" names rc=0 err
  if ! names="$(declared_in "$pkgs" 2>&1)"; then
    err="${names##*$'\n'}"
    printf '  %s: %s: %s\n' "$file" "$id" "$err"
    bad=$((bad + 1))
    return 0
  fi
  printf '%s\n' "$names" | grep -Eq -- "$alt" 2>/dev/null || rc=$?
  case "$rc" in
    0) ;;
    1)
      printf '  %s: %s: -run alternative matches no test: %s (in %s)\n' "$file" "$id" "$alt" "$pkgs"
      bad=$((bad + 1)) ;;
    *)
      echo "check-features-selectors: cannot evaluate -run '$alt' in $file ($id): not an extended regular expression" >&2
      exit 2 ;;
  esac
}

while IFS= read -r -d '' f; do
  jq -e -s 'length == 1 and (.[0] | type) == "array"' "$f" >/dev/null 2>&1 ||
    { echo "check-features-selectors: cannot read $f: not one JSON array of features" >&2; exit 2; }
  while IFS=$'\t' read -r id cmd; do
    [ -n "$cmd" ] || continue
    while IFS=$'\t' read -r new pkgs alt; do
      [ -n "$alt" ] || continue
      if [ "$new" = 1 ]; then selectors=$((selectors + 1)); fi
      check_alternative "$f" "$id" "$pkgs" "$alt"
    done <<EOF
$(printf '%s\n' "$cmd" | awk -v sq="'" "$SELECTORS")
EOF
  done <<EOF
$(jq -r '.[] | select((.verification | type) == "string") | [.id, (.verification | gsub("\n"; " ; "))] | @tsv' "$f")
EOF
done <"$list"

if [ "$bad" -eq 0 ]; then
  echo "check-features-selectors: OK ($selectors go test -run selectors)"
  exit 0
fi
echo "check-features-selectors: $bad -run alternatives match no test, so their commands exit 0 without checking them ($selectors selectors read)"
exit 1
