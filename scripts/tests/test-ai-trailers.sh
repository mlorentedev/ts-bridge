#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-ai-trailers.sh"
[ -f "$GUARD" ] || { echo "test-ai-trailers: guard not found at $GUARD" >&2; exit 2; }
# bash by default, so the suite runs on any workstation; CI passes ATG_SHELLS='bash zsh'.
SHELLS="${ATG_SHELLS:-bash}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

commit() {
  printf '%s\n' "$2" >"$WORK/msg"
  git -C "$1" -c user.name=Dev -c user.email=dev@example.com \
    commit -q --allow-empty -F "$WORK/msg"
}

# Each case builds base..head in its own repository and names the exit the guard must give.
build() {
  dir="$WORK/$1"
  git init -q "$dir"
  commit "$dir" "chore: base"
  git -C "$dir" tag base
  case "$1" in
    no-trailer) commit "$dir" "feat: plain change" ;;
    copilot) commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>')" ;;
    claude-key-case) commit "$dir" "$(printf 'fix: x\n\nCo-Authored-By: Claude <noreply@anthropic.com>')" ;;
    among-humans) commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: Ana <ana@example.com>\nCo-authored-by: Cursor Agent <cursoragent@cursor.com>')" ;;
    human) commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: Ana <ana@example.com>')" ;;
    prose-mention) commit "$dir" "$(printf 'docs: x\n\nCo-authored-by: Copilot is what the harness appends.\n\nThe gate rejects it.')" ;;
    earlier-in-range)
      commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: Copilot <c@example.com>')"
      commit "$dir" "fix: clean follow-up"
      ;;
    before-base)
      commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: Copilot <c@example.com>')"
      git -C "$dir" tag -f base >/dev/null
      commit "$dir" "fix: clean change"
      ;;
  esac
}

CASES='no-trailer:0 human:0 prose-mention:0 before-base:0 copilot:1 claude-key-case:1 among-humans:1 earlier-in-range:1'
for name_rc in $CASES; do
  build "${name_rc%%:*}"
done

failed=0
for sh in $SHELLS; do
  if ! command -v "$sh" >/dev/null 2>&1; then
    echo "FAIL [$sh] not installed: every shell in ATG_SHELLS is required" >&2
    failed=1
    continue
  fi
  for name_rc in $CASES; do
    name="${name_rc%%:*}"
    want="${name_rc##*:}"
    rc=0
    "$sh" "$GUARD" base HEAD "$WORK/$name" >"$WORK/out" 2>&1 || rc=$?
    if [ "$rc" != "$want" ]; then
      echo "FAIL [$sh] $name: exit $rc, want $want" >&2
      sed 's/^/  /' "$WORK/out" >&2
      failed=1
    fi
  done
  rc=0
  "$sh" "$GUARD" base no-such-ref "$WORK/no-trailer" >/dev/null 2>&1 || rc=$?
  [ "$rc" = 2 ] || { echo "FAIL [$sh] unresolvable ref: exit $rc, want 2" >&2; failed=1; }
done
[ "$failed" = 0 ] || exit 1
echo "test-ai-trailers: OK ($SHELLS)"
