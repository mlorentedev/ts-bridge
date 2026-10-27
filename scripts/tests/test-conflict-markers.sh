#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-conflict-markers.sh"
[ -f "$GUARD" ] || { echo "test-conflict-markers: guard not found at $GUARD" >&2; exit 2; }

# The guard scans every tracked file, so a committed fixture holding a marker would red this
# repo. Each case builds a throwaway repository instead, and the markers are assembled here
# rather than written out at the start of a line.
OPEN="$(printf '%7s' '' | tr ' ' '<')"
SEP="$(printf '%7s' '' | tr ' ' '=')"
CLOSE="$(printf '%7s' '' | tr ' ' '>')"
BASE="$(printf '%7s' '' | tr ' ' '|')"
# conflict-marker-size=32 in .gitattributes makes Git write runs of 32.
LONG_OPEN="$(printf '%32s' '' | tr ' ' '<')"
LONG_CLOSE="$(printf '%32s' '' | tr ' ' '>')"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

new_repo() {
  git init -q "$1"
  printf '# Notes\n\nPlain text.\n' >"$1/README.md"
  git -C "$1" add README.md
}

conflict() {
  printf 'before\n%s HEAD\nours\n%s\ntheirs\n%s origin/master\nafter\n' "$OPEN" "$SEP" "$CLOSE"
}

build() {
  dir="$WORK/$1"
  case "$1" in
    clean) new_repo "$dir" ;;
    tracked-conflict)
      new_repo "$dir"
      mkdir -p "$dir/docs"
      conflict >"$dir/docs/index.md"
      git -C "$dir" add docs/index.md
      ;;
    bare-open-marker)
      new_repo "$dir"
      printf 'a\n%s\nb\n' "$OPEN" >"$dir/notes.txt"
      git -C "$dir" add notes.txt
      ;;
    untracked-conflict)
      new_repo "$dir"
      conflict >"$dir/scratch.md"
      ;;
    setext-short)
      new_repo "$dir"
      printf 'Title\n===\n\nBody\n' >"$dir/guide.md"
      git -C "$dir" add guide.md
      ;;
    setext-seven)
      new_repo "$dir"
      printf 'Title\n%s\n\nBody\n' "$SEP" >"$dir/guide.md"
      git -C "$dir" add guide.md
      ;;
    indented-example)
      new_repo "$dir"
      printf 'Example:\n\n    %s HEAD\n    %s origin/master\n' "$OPEN" "$CLOSE" >"$dir/lesson.md"
      git -C "$dir" add lesson.md
      ;;
    not-a-repo) mkdir -p "$dir" ;;
    bare-repo) git init -q --bare "$dir" ;;
    dotgit/.git) new_repo "$WORK/dotgit" ;;
    long-markers)
      new_repo "$dir"
      printf '*.md conflict-marker-size=32\n' >"$dir/.gitattributes"
      printf 'a\n%s HEAD\nours\n%s origin/master\nb\n' "$LONG_OPEN" "$LONG_CLOSE" >"$dir/notes.md"
      git -C "$dir" add .gitattributes notes.md
      ;;
    diff3-base)
      new_repo "$dir"
      printf 'a\n%s merged common ancestors\nbase\nb\n' "$BASE" >"$dir/notes.txt"
      git -C "$dir" add notes.txt
      ;;
    nested/sub)
      new_repo "$WORK/nested"
      mkdir -p "$dir" "$WORK/nested/docs"
      printf 'plain\n' >"$dir/ok.txt"
      conflict >"$WORK/nested/docs/index.md"
      git -C "$WORK/nested" add sub/ok.txt docs/index.md
      ;;
  esac
}

SPECS='clean:0:OK
tracked-conflict:1:docs/index.md:2:
bare-open-marker:1:notes.txt:2:
untracked-conflict:0:OK
setext-short:0:OK
setext-seven:0:OK
indented-example:0:OK
not-a-repo:2:not a Git work tree
bare-repo:2:not a Git work tree
dotgit/.git:2:not a Git work tree
long-markers:1:notes.md:2:
diff3-base:1:notes.txt:2:
nested/sub:1:docs/index.md:2:'

failed=0
for sh_name in ${TCM_SHELLS:-bash zsh}; do
  if ! command -v "$sh_name" >/dev/null 2>&1; then
    echo "  FAIL [$sh_name] not installed: both shells are required" >&2
    failed=1
    continue
  fi

  while IFS= read -r spec; do
    [ -n "$spec" ] || continue
    name="${spec%%:*}"
    rest="${spec#*:}"
    want="${rest%%:*}"
    want_sub="${rest#*:}"
    [ -d "$WORK/$name" ] || build "$name"
    got="$("$sh_name" "$GUARD" "$WORK/$name" 2>&1)" && rc=0 || rc=$?

    if [ "$rc" -ne "$want" ]; then
      echo "  FAIL [$sh_name] $name: exit $rc, wanted $want"
      echo "$got"
      failed=1
      continue
    fi
    case "$got" in
      *"$want_sub"*) echo "  ok   [$sh_name] $name -> exit $rc, mentions \"$want_sub\"" ;;
      *)
        echo "  FAIL [$sh_name] $name: output does not mention '$want_sub'"
        echo "$got"
        failed=1
        ;;
    esac
  done <<EOF
$SPECS
EOF
done
exit "$failed"
