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
    human-name-contains-agent) commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: Claudette Geminiani <claudette@example.com>')" ;;
    folded-value) commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: Pair\n  Copilot <c@example.com>')" ;;
    fleet-agents) commit "$dir" "$(printf 'fix: x\n\nCo-authored-by: OpenCode <bot@example.com>\nCo-authored-by: Hermes Agent <hermes@example.com>')" ;;
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

CASES='no-trailer:0 human:0 human-name-contains-agent:0 prose-mention:0 before-base:0 copilot:1 claude-key-case:1 among-humans:1 earlier-in-range:1 folded-value:1 fleet-agents:1'
for name_rc in $CASES; do
  build "${name_rc%%:*}"
done

# One repository per trailer value, so dropping any single name from the guard fails a case.
# Names that are also human names (Cline, Goose, Amp, Hermes) count only beside agent/bot/ai.
AGENT_VALUES=('Antigravity <a@example.com>' 'OpenCode <o@example.com>' 'Qwen Code <q@example.com>'
  'Cline Bot <c@example.com>' 'Goose AI <g@example.com>' 'Amp Agent <a@example.com>' 'Hermes-Agent <h@example.com>')
HUMAN_VALUES=('Sam Cline <sam@example.com>' 'Lucy Goose <lucy@example.com>' 'Amp Patel <amp@example.com>'
  'Hermes Conrad <hermes@example.com>')
i=0
for value in "${AGENT_VALUES[@]}"; do
  i=$((i + 1)); git init -q "$WORK/agent-$i"; commit "$WORK/agent-$i" "chore: base"; git -C "$WORK/agent-$i" tag base
  commit "$WORK/agent-$i" "$(printf 'fix: x\n\nCo-authored-by: %s' "$value")"
  CASES="$CASES agent-$i:1"
done
i=0
for value in "${HUMAN_VALUES[@]}"; do
  i=$((i + 1)); git init -q "$WORK/human-$i"; commit "$WORK/human-$i" "chore: base"; git -C "$WORK/human-$i" tag base
  commit "$WORK/human-$i" "$(printf 'fix: x\n\nCo-authored-by: %s' "$value")"
  CASES="$CASES human-$i:0"
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
  rc=0
  "$sh" "$GUARD" base >/dev/null 2>&1 || rc=$?
  [ "$rc" = 2 ] || { echo "FAIL [$sh] one argument: exit $rc, want 2" >&2; failed=1; }
  # The report names the offending commit and the trailer value, not only the exit code.
  "$sh" "$GUARD" base HEAD "$WORK/copilot" >"$WORK/out" 2>&1 || true
  sha="$(git -C "$WORK/copilot" rev-parse --short HEAD)"
  grep -Fq "$sha: Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>" "$WORK/out" ||
    { echo "FAIL [$sh] copilot: report does not name $sha and its trailer" >&2; sed 's/^/  /' "$WORK/out" >&2; failed=1; }
done
[ "$failed" = 0 ] || exit 1
echo "test-ai-trailers: OK ($SHELLS)"
