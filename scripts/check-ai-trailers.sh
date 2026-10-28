#!/usr/bin/env bash
# Reject commits in a range whose Co-authored-by trailer names an AI agent (#506).
set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  echo "usage: check-ai-trailers.sh <base> <head> [repo]" >&2
  exit 2
fi
BASE="$1"
HEAD="$2"
REPO="${3:-.}"

# Matched case-insensitively against the trailer value (name and address), as whole words, so
# `Claudette` is a human while `Copilot@users`, `cursoragent@cursor.com` and `Claude <...>` match.
AI_AGENTS='copilot|claude|anthropic|chatgpt|openai|gemini|codex|cursor|devin|aider|windsurf|codeium|tabnine|codewhisperer'

# Git parses the trailer block itself, so a mention in the body prose is not a trailer, and
# the key matches in any case (`Co-Authored-By`). `unfold` joins a value continued on an
# indented line, which would otherwise escape as a line of its own. A ref that does not
# resolve fails closed.
if ! log="$(git -C "$REPO" log --format='%h%x09%(trailers:key=Co-authored-by,valueonly,unfold,separator=%x1f)' \
  "$BASE..$HEAD" -- 2>&1)"; then
  printf '%s\n' "$log" >&2
  echo "check-ai-trailers: cannot read $BASE..$HEAD" >&2
  exit 2
fi

found="$(printf '%s\n' "$log" | awk -F '\t' -v agents="$AI_AGENTS" '
  NF < 2 || $2 == "" { next }
  {
    n = split($2, values, "\037")
    for (i = 1; i <= n; i++)
      if (tolower(values[i]) ~ "(^|[^a-z])(" agents ")([^a-z]|$)") print $1 ": Co-authored-by: " values[i]
  }
')"
commits="$(printf '%s\n' "$log" | awk 'NF { n++ } END { print n + 0 }')"

if [ -n "$found" ]; then
  printf '%s\n' "$found"
  echo "check-ai-trailers: $(printf '%s\n' "$found" | awk 'END { print NR }') AI-attribution trailer(s); reword the commit(s) without them"
  exit 1
fi
echo "check-ai-trailers: OK ($commits commits)"
