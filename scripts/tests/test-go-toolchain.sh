#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

check_file() {
  awk '
    function finish_job() {
      if (job_go && !job_setup) bad = 1
      job_go = 0; job_setup = 0
    }
    /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
    in_jobs && /^[^[:space:]]/ { finish_job(); in_jobs = 0; in_job = 0 }
    in_jobs && /^  [[:alnum:]_-]+:[[:space:]]*$/ {
      finish_job(); in_job = 1; next
    }
    /^[[:space:]]*#/ { next }
    in_job && /(^|[[:space:]])go[[:space:]]+(build|clean|env|fmt|generate|install|list|mod|run|test|tool|version|vet|work)([[:space:]]|$)/ {
      if (!job_setup) bad = 1
      job_go = 1
    }
    /^[[:space:]]*-[[:space:]]*uses:[[:space:]]+actions\/setup-go@/ {
      if (active && !valid) bad = 1
      if (in_job) job_setup = 1
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
      finish_job()
      exit bad
    }
  ' "$1"
}

check_linter() {
  local module_go lint_version built_go
  module_go="$(awk '$1 == "go" { sub(/\r$/, "", $2); print $2 }' "$1")"
  lint_version="$(awk '$1 == "version:" { sub(/\r$/, "", $2); if ($2 ~ /^v[0-9]+\.[0-9]+\.[0-9]+$/) print $2 }' "$2")"
  # v2.14.0 was built with Go 1.27.0; verify build metadata before approving another pin.
  case "$lint_version" in
    v2.14.0) built_go=1.27 ;;
    *) return 1 ;;
  esac
  grep -Fq "Go $module_go+" "$3" &&
    grep -Fq "golangci-lint@$lint_version" "$3" &&
    awk -v required="$module_go" -v built="$built_go" '
      BEGIN {
        n = split(required, r, "\\.")
        split(built, b, "\\.")
        exit !(n == 3 && r[1] == b[1] && r[2] <= b[2] &&
               r[1] ~ /^[0-9]+$/ && r[2] ~ /^[0-9]+$/ && r[3] ~ /^[0-9]+$/)
      }
    '
}

tmp="$(mktemp)"
tmp_mod="$(mktemp)"
tmp_ci="$(mktemp)"
tmp_docs="$(mktemp)"
trap 'rm -f "$tmp" "$tmp_mod" "$tmp_ci" "$tmp_docs"' EXIT
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
          cache: true
      - uses: example/action@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a version in an unrelated step" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a Go job without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  configured:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
      - run: go build ./...
  missing:
    steps:
      - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a Go job without setup-go in a mixed workflow" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go test ./...
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a Go command before setup-go" >&2
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

printf 'go 1.27.1\n' > "$tmp_mod"
printf '          version: v2.14.0\n' > "$tmp_ci"
printf 'Go 1.27.1+\ngolangci-lint@v2.14.0\n' > "$tmp_docs"
if ! check_linter "$tmp_mod" "$tmp_ci" "$tmp_docs"; then
  echo "test-go-toolchain: rejected the compatible linter pin" >&2
  exit 1
fi
printf 'go 1.28.0\n' > "$tmp_mod"
printf 'Go 1.28.0+\ngolangci-lint@v2.14.0\n' > "$tmp_docs"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a module newer than the linter compiler" >&2
  exit 1
fi
printf 'go 1.27.1\n' > "$tmp_mod"
printf 'Go 1.27.1+\ngolangci-lint@v2.12.2\n' > "$tmp_docs"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a different documented linter pin" >&2
  exit 1
fi
printf '          version: v2.12.2\n' > "$tmp_ci"
printf 'Go 1.27.1+\ngolangci-lint@v2.12.2\n' > "$tmp_docs"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a linter built with an older Go" >&2
  exit 1
fi

count=0
for workflow in "$ROOT"/.github/workflows/*; do
  case "$workflow" in *.yml|*.yaml) ;; *) continue ;; esac
  if grep -Eq '^[[:space:]]*-[[:space:]]*uses:[[:space:]]+actions/setup-go@' "$workflow"; then
    count=$((count + 1))
  fi
  if ! check_file "$workflow"; then
    echo "test-go-toolchain: Go jobs require setup-go with go-version-file: go.mod in $workflow" >&2
    exit 1
  fi
done
[ "$count" -gt 0 ] || { echo "test-go-toolchain: no Go workflows found" >&2; exit 1; }
if ! check_linter "$ROOT/go.mod" "$ROOT/.github/workflows/ci.yml" "$ROOT/AGENTS.md"; then
  echo "test-go-toolchain: module Go minimum, CI linter and operator instructions disagree" >&2
  exit 1
fi
echo "test-go-toolchain: OK ($count Go workflows)"
