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
      { lines[FNR] = $0 }
      END {
        command = ""
        for (i = 1; i <= FNR; i++) {
          line = lines[i]
          if (line ~ /ts-bridge(\.exe)?[[:space:]]/) {
            command = line ~ /ts-bridge(\.exe)?[[:space:]]+init([[:space:]]|$)/ ? "init" : "other"
          }
          inline_key = line ~ /--auth-key([[:space:]]+|=)[^[:space:]`]+/
          if (inline_key && command == "other") {
            printf "%s:%d: inline --auth-key is forbidden in command examples\n", file, i
          }
          if (inline_key && command == "init") {
            warned = 0
            first = i > 3 ? i - 3 : 1
            last = i + 3 < FNR ? i + 3 : FNR
            for (j = first; j <= last; j++) {
              warning = tolower(lines[j])
              if (warning ~ /process[- ](list|table)/) warned = 1
            }
            if (!warned) {
              printf "%s:%d: init --auth-key example has no nearby process-table warning\n", file, i
            }
          }
          if (line !~ /[\\`][[:space:]]*$/) command = ""
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
