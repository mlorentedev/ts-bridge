#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

check_file() {
  awk '
    /^[[:space:]]*#/ { next }
    /^[[:space:]]*-[[:space:]]*uses:[[:space:]]+actions\/setup-go@/ {
      if (active && !valid) bad = 1
      active = 1; valid = 0; with_indent = -1
      next
    }
    active && /^[[:space:]]*-[[:space:]]/ {
      if (!valid) bad = 1
      active = 0
    }
    active && /^[[:space:]]*with:[[:space:]]*$/ {
      with_indent = index($0, "with:") - 1
    }
    active && with_indent >= 0 &&
      /^[[:space:]]*go-version-file:[[:space:]]*go\.mod([[:space:]]*(#.*)?)?$/ {
      if (index($0, "go-version-file:") - 1 == with_indent + 2) valid = 1
    }
    active && with_indent >= 0 && /^[[:space:]]*go-version:[[:space:]]*/ {
      if (index($0, "go-version:") - 1 == with_indent + 2) bad = 1
    }
    END {
      if (active && !valid) bad = 1
      exit bad
    }
  ' "$1"
}

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version: '1.26'
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a pinned Go 1.26 toolchain" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: rejected a module-derived toolchain" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version: '1.26'
      - run: echo "go-version-file: go.mod"
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a version in an unrelated step" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
          go-version: '1.26'
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a conflicting pinned Go version" >&2
  exit 1
fi

count=0
for workflow in "$ROOT"/.github/workflows/*; do
  case "$workflow" in *.yml|*.yaml) ;; *) continue ;; esac
  if grep -Eq '^[[:space:]]*-[[:space:]]*uses:[[:space:]]+actions/setup-go@' "$workflow"; then
    count=$((count + 1))
    if ! check_file "$workflow"; then
      echo "test-go-toolchain: setup-go must use go-version-file: go.mod in $workflow" >&2
      exit 1
    fi
  fi
done
[ "$count" -gt 0 ] || { echo "test-go-toolchain: no Go workflows found" >&2; exit 1; }
echo "test-go-toolchain: OK ($count Go workflows)"
