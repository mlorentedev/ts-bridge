#!/usr/bin/env bash
# Guard command examples in README.md and docs/ against inline auth keys.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
SCAN_ROOT="${1:-$ROOT}"
files=()

[ -f "$SCAN_ROOT/README.md" ] && files+=("$SCAN_ROOT/README.md")
if [ -d "$SCAN_ROOT/docs" ]; then
  while IFS= read -r -d '' file; do files+=("$file"); done < <(
    find "$SCAN_ROOT/docs" -type f \( -name '*.md' -o -name '*.mdx' \) -print0
  )
fi

findings=0
for file in "${files[@]}"; do
  display="${file#"$SCAN_ROOT"/}"
  output="$(
    awk -v file="$display" '
      # The guard asks where an inline key appears, not how ts-bridge was invoked:
      # three review rounds showed a command matcher can always be wrapped
      # (sudo, quotes, prompts, subshells, runners). So every logical command in
      # a code region is checked, and in prose only an inline code span that
      # names ts-bridge is.
      function auth_key_count(text, n) {
        n = 0
        while (match(text, /--auth-key([[:space:]]+|=)[^[:space:]`]+/)) {
          n++
          text = substr(text, RSTART + RLENGTH)
        }
        return n
      }
      function has_warning(first, last, pos, warning) {
        first = first > 3 ? first - 3 : 1
        last = last + 3 < FNR ? last + 3 : FNR
        for (pos = first; pos <= last; pos++) {
          warning = tolower(lines[pos])
          if (warning ~ /process[- ](list|table)/) return 1
        }
        return 0
      }
      # init may still take an inline key next to a process-table warning, but
      # only when it is the one key in an unsplittable segment: a second key in
      # the same segment (a subshell, a brace group) is a finding.
      function scan_command(text, first, last, parts, count, pos, keys) {
        count = split(text, parts, /(;|&&|\|\|?)/)
        for (pos = 1; pos <= count; pos++) {
          keys = auth_key_count(parts[pos])
          if (keys == 0) continue
          if (keys == 1 && init_token(parts[pos])) {
            if (!has_warning(first, last)) {
              printf "%s:%d: init --auth-key example has no nearby process-table warning\n", file, last
            }
          } else {
            printf "%s:%d: inline --auth-key is forbidden in command examples\n", file, last
          }
        }
      }
      # `init` counts only as a bare word: one inside quotes or a command
      # substitution (VAR=" init ", $(echo init)) is data, not the subcommand.
      function init_token(text) {
        gsub(/"[^"]*"|\047[^\047]*\047|\$\([^)]*\)|`[^`]*`/, "", text)
        return text ~ /(^|[[:space:]])init([[:space:]]|$)/
      }
      function scan_prose(line, i, span) {
        while (match(line, /`[^`]+`/)) {
          span = substr(line, RSTART + 1, RLENGTH - 2)
          line = substr(line, RSTART + RLENGTH)
          if (span ~ /ts-bridge/) scan_command(span, i, i)
        }
      }
      { lines[FNR] = $0 }
      END {
        command = ""
        first = 0
        fence = ""
        icode = 0
        in_list = 0
        prev_blank = 1
        for (i = 1; i <= FNR; i++) {
          line = lines[i]
          # A fence closes only on a run of the same character at least as long
          # as the one that opened it, so a ```` block may contain ``` lines.
          if (match(line, /^[[:space:]]*(`{3,}|~{3,})/)) {
            run = substr(line, RSTART, RLENGTH); sub(/^[[:space:]]+/, "", run)
            # A command still open when a fence opens or closes (a dangling
            # continuation) is scanned, never dropped.
            if (command != "") { scan_command(command, first, i - 1); command = "" }
            if (fence == "") fence = run
            else if (substr(run, 1, 1) == substr(fence, 1, 1) && length(run) >= length(fence) &&
                     substr(line, RSTART + RLENGTH) ~ /^[[:space:]]*$/) fence = ""
            continue
          }
          if (fence == "") {
            blank = line ~ /^[[:space:]]*$/
            indented = line ~ /^(    |\t)/
            # Indented code is a block after a blank line. Inside a list only a
            # nested list item stays prose; any other indented line is code.
            marker = line ~ /^[[:space:]]*([-*+]|[0-9]+\.)[[:space:]]/
            if (indented && (icode || (prev_blank && (!in_list || !marker)))) icode = 1
            else if (!blank) {
              icode = 0
              if (!indented) in_list = line ~ /^([-*+]|[0-9]+\.)[[:space:]]/ || (in_list && !prev_blank)
            }
            prev_blank = blank
          }
          if (fence == "" && !icode) {
            if (command != "") { scan_command(command, first, i - 1); command = "" }
            scan_prose(line, i)
            continue
          }
          # A PowerShell continuation backtick follows whitespace; a backtick
          # glued to the text closes an inline code span instead.
          continued = line ~ /(\\|[[:space:]]`)[[:space:]]*$/
          if (continued) sub(/[\\`][[:space:]]*$/, "", line)
          if (command == "") first = i
          command = command == "" ? line : command " " line
          if (!continued) {
            scan_command(command, first, i)
            command = ""
          }
        }
        if (command != "") scan_command(command, first, FNR)
      }
    ' "$file"
  )"
  if [ -n "$output" ]; then
    printf '%s\n' "$output"
    count="$(printf '%s\n' "$output" | awk 'END { print NR }')"
    findings=$((findings + count))
  fi
done

if [ "$findings" -gt 0 ]; then
  echo "check-doc-authkey: $findings finding(s); use --auth-key-file in command examples"
  exit 1
fi
echo "check-doc-authkey: OK (${#files[@]} files)"
