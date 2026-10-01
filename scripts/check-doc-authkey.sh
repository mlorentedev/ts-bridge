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
      function normalize(segment) {
        sub(/^[[:space:]]*>[[:space:]]*/, "", segment)
        sub(/^[[:space:]]+/, "", segment)
        sub(/^[$][[:space:]]+/, "", segment)
        sub(/[[:space:]]+$/, "", segment)
        if (segment ~ /^`[^`]+`$/) {
          sub(/^`/, "", segment)
          sub(/`$/, "", segment)
        }
        return segment
      }
      function is_command(segment) {
        segment = normalize(segment)
        return segment ~ /^ts-bridge(\.exe)?[[:space:]]/ ||
               segment ~ /^\.[\/\\]ts-bridge(\.exe)?[[:space:]]/
      }
      function contains_command(text, parts, count, pos) {
        count = split(text, parts, /(;|&&|\|\|)/)
        for (pos = 1; pos <= count; pos++) {
          if (is_command(parts[pos])) return 1
        }
        return 0
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
      function scan_command(text, first, last, parts, count, pos, segment) {
        count = split(text, parts, /(;|&&|\|\|)/)
        for (pos = 1; pos <= count; pos++) {
          segment = normalize(parts[pos])
          if (!is_command(segment) ||
              segment !~ /--auth-key([[:space:]]+|=)[^[:space:]`]+/) continue
          if (segment ~ /^(\.[\/\\])?ts-bridge(\.exe)?[[:space:]]+init([[:space:]]|$)/) {
            if (!has_warning(first, last)) {
              printf "%s:%d: init --auth-key example has no nearby process-table warning\n", file, last
            }
          } else {
            printf "%s:%d: inline --auth-key is forbidden in command examples\n", file, last
          }
        }
      }
      { lines[FNR] = $0 }
      END {
        command = ""
        first = 0
        for (i = 1; i <= FNR; i++) {
          line = lines[i]
          continued = line ~ /[\\`][[:space:]]*$/
          sub(/[\\`][[:space:]]*$/, "", line)
          if (command == "" && contains_command(line)) {
            command = line
            first = i
          } else if (command != "") {
            command = command " " line
          }
          if (command != "" && !continued) {
            scan_command(command, first, i)
            command = ""
            first = 0
          }
        }
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
