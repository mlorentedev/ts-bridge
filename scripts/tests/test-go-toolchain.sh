#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

check_file() {
  awk '
    function finish_job() {
      if (job_go && !job_setup) bad = 1
      job_go = 0; job_setup = 0
    }
    /^jobs:[[:space:]]*(#.*)?$/ { in_jobs = 1; next }
    in_jobs && /^[^[:space:]]/ { finish_job(); in_jobs = 0; in_job = 0 }
    # Any line at job depth opens a new job, whatever the key spelling
    # (quoted, commented), so one job never inherits the setup-go of another.
    in_jobs && /^  [^[:space:]#]/ {
      finish_job(); in_job = 1; next
    }
    /^[[:space:]]*#/ { next }
    # Every Go command counts, not only those inside a recognised job: one the
    # parser cannot place fails closed, so an unrecognised key spelling can
    # never switch the checks off.
    /(^|[[:space:]])go[[:space:]]+(build|clean|env|fmt|generate|install|list|mod|run|test|tool|version|vet|work)([[:space:]]|$)/ {
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
  local module_go lint_version mise_version built_go
  module_go="$(awk '$1 == "go" { sub(/\r$/, "", $2); print $2 }' "$1")"
  lint_version="$(awk '$1 == "version:" { sub(/\r$/, "", $2); if ($2 ~ /^v[0-9]+\.[0-9]+\.[0-9]+$/) print $2 }' "$2")"
  # mise.toml is the local pin (#452): `golangci-lint = "X.Y.Z"` must equal CI's vX.Y.Z.
  mise_version="$(awk -F'"' '/^golangci-lint[[:space:]]*=/ { print $2 }' "$3")"
  [ -n "$lint_version" ] && [ "v$mise_version" = "$lint_version" ] || return 1
  # v2.14.0 was built with Go 1.27.0; verify build metadata before approving another pin.
  case "$lint_version" in
    v2.14.0) built_go=1.27 ;;
    *) return 1 ;;
  esac
  # The instructions carry the install command that reads the pin, and never
  # restate a linter version in any spelling (`@v2.x`, `@2.x`, ` v2.x`).
  grep -Fq "Go $module_go+" "$4" &&
    grep -Eq '^mise install([[:space:]]|$)' "$4" &&
    ! grep -Eiq 'golangci-lint[@[:space:]]+v?[0-9]+\.[0-9]+' "$4" &&
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
tmp_mise="$(mktemp)"
trap 'rm -f "$tmp" "$tmp_mod" "$tmp_ci" "$tmp_docs" "$tmp_mise"' EXIT
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
cat > "$tmp" <<'YAML'
jobs:   # job list
  build:
    steps:
      - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a Go job without setup-go under a commented jobs key" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:   # unit tests
    steps:
      - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a Go job without setup-go under a commented job key" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
"jobs":
  build:
    steps:
      - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a Go command outside any recognised job" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:   # job list
  build:   # unit tests
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
      - run: go test ./...
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: rejected a configured job under commented keys" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  lint:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
  "test":
    steps:
      - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: let a quoted job key inherit the previous job's setup-go" >&2
  exit 1
fi

printf 'go 1.27.1\n' > "$tmp_mod"
printf '          version: v2.14.0\n' > "$tmp_ci"
printf '[tools]\ngolangci-lint = "2.14.0"\n' > "$tmp_mise"
printf 'Go 1.27.1+\nmise install\ngolangci-lint run\n' > "$tmp_docs"
if ! check_linter "$tmp_mod" "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: rejected the compatible linter pin" >&2
  exit 1
fi
printf '[tools]\ngolangci-lint = "2.12.2"\n' > "$tmp_mise"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a local pin that differs from CI" >&2
  exit 1
fi
printf '[tools]\n' > "$tmp_mise"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a missing local pin" >&2
  exit 1
fi
printf '[tools]\ngolangci-lint = "2.14.0"\n' > "$tmp_mise"
for restated in 'go install golangci-lint@v2.14.0' 'golangci-lint@2.12.2' 'golangci-lint v2.12.2'; do
  printf 'Go 1.27.1+\nmise install\n%s\n' "$restated" > "$tmp_docs"
  if check_linter "$tmp_mod" "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
    echo "test-go-toolchain: accepted instructions that restate the linter version ($restated)" >&2
    exit 1
  fi
done
printf 'Go 1.27.1+\nLocal tool pins live in mise.toml\ngolangci-lint run\n' > "$tmp_docs"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted instructions that mention mise.toml but drop mise install" >&2
  exit 1
fi
printf 'go 1.28.0\n' > "$tmp_mod"
printf 'Go 1.28.0+\nmise install\n' > "$tmp_docs"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a module newer than the linter compiler" >&2
  exit 1
fi
printf 'go 1.27.1\n' > "$tmp_mod"
printf '          version: v2.12.2\n' > "$tmp_ci"
printf '[tools]\ngolangci-lint = "2.12.2"\n' > "$tmp_mise"
printf 'Go 1.27.1+\nmise install\ngolangci-lint run\n' > "$tmp_docs"
if check_linter "$tmp_mod" "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
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
if ! check_linter "$ROOT/go.mod" "$ROOT/.github/workflows/ci.yml" "$ROOT/mise.toml" "$ROOT/AGENTS.md"; then
  echo "test-go-toolchain: module Go minimum, CI linter, mise.toml pin and operator instructions disagree" >&2
  exit 1
fi
echo "test-go-toolchain: OK ($count Go workflows)"
