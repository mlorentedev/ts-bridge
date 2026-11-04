#!/usr/bin/env bash
# Guard: every workflow job that runs Go installs it with setup-go from go.mod,
# and the linter pins agree with the module's Go version.
#
# Scope (#499). A Go command is recognised on the workflow line itself: any
# `go` or `go.exe` subcommand from `go help`, with or without flags before it,
# quoted, continued or inside `sh -c`. Out of scope, because no line matcher
# can see them without a shell parser: Go run by wrapper scripts (`make test`,
# `./scripts/*.sh`), Go run inside other actions, and `go${IFS}test`-style
# word splitting. Also out of scope: `GOTOOLCHAIN` and the go.mod `toolchain`
# directive. go.mod has no `toolchain` line, so the `go` version setup-go installs
# from it is the only one the module asks for, and no workflow here sets
# `GOTOOLCHAIN`. Adding either one reopens this decision: whether the pinned
# setup-go reads a `toolchain` line is not checked here.
set -euo pipefail

# Run the whole suite under bash and zsh, as the repo requires of scripts/*.sh
# (#500). The outer run re-executes this file once per shell and fails when
# either is missing; TGT_SHELL marks the inner run. TGT_SHELLS narrows the set,
# to bash or zsh only, so no other command can stand in for a shell.
tgt_running=""
if [ -n "${ZSH_VERSION:-}" ]; then tgt_running=zsh; elif [ -n "${BASH_VERSION:-}" ]; then tgt_running=bash; fi
if [ -z "${TGT_SHELL:-}" ]; then
  tgt_failed=0
  tgt_ran=0
  # Command substitution splits on IFS in both shells; a bare expansion does
  # not in zsh.
  # shellcheck disable=SC2046
  for tgt_sh in $(printf '%s' "${TGT_SHELLS:-bash zsh}"); do
    case "$tgt_sh" in
      bash | zsh) ;;
      *)
        echo "test-go-toolchain: TGT_SHELLS names $tgt_sh: only bash and zsh are accepted" >&2
        tgt_failed=1
        continue
        ;;
    esac
    if ! command -v "$tgt_sh" >/dev/null 2>&1; then
      echo "test-go-toolchain: [$tgt_sh] not installed: both shells are required" >&2
      tgt_failed=1
      continue
    fi
    tgt_ran=$((tgt_ran + 1))
    TGT_SHELL="$tgt_sh" "$tgt_sh" "$0" "$@" || { echo "test-go-toolchain: failed under $tgt_sh" >&2; tgt_failed=1; }
  done
  # A blank TGT_SHELLS splits into no words: a run of no shell proves nothing.
  if [ "$tgt_ran" -eq 0 ]; then
    echo "test-go-toolchain: TGT_SHELLS names no shell: nothing ran" >&2
    tgt_failed=1
  fi
  exit "$tgt_failed"
fi
# An inherited TGT_SHELL must not skip the loop: the inner run is valid only
# under the shell it names.
if [ "$TGT_SHELL" != "$tgt_running" ]; then
  echo "test-go-toolchain: TGT_SHELL=$TGT_SHELL but running under ${tgt_running:-an unknown shell}" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

check_file() {
  awk '
    function finish_job() {
      if (job_go && !job_setup) bad = 1
      job_go = 0; job_setup = 0
    }
    function indent_of(line) { match(line, /[^ ]/); return RSTART - 1 }
    /^jobs:[[:space:]]*(#.*)?$/ { in_jobs = 1; job_indent = -1; next }
    in_jobs && /^[^[:space:]]/ { finish_job(); in_jobs = 0; in_job = 0 }
    # Job depth is the indent of the first key under jobs:, not a literal two
    # spaces (#505). Any line at that depth opens a new job, whatever the key
    # spelling (quoted, commented), so one job never inherits another job setup-go.
    in_jobs && job_indent < 0 && /^ +[^[:space:]#]/ { job_indent = indent_of($0) }
    in_jobs && job_indent > 0 && /^ +[^[:space:]#]/ && indent_of($0) == job_indent {
      finish_job(); in_job = 1; cont = ""; next
    }
    /^[[:space:]]*#/ { next }
    # A job or step name is a label, never a command.
    { is_name = ($0 ~ /^[[:space:]]*(-[[:space:]]+)?name:/) }
    # A `\` continuation is matched joined to the next line, so `go \` +
    # `test ./...` is still one Go command.
    {
      cmd = cont $0; cont = ""
      if (cmd ~ /\\[[:space:]]*$/) { cont = cmd; sub(/\\[[:space:]]*$/, " ", cont) }
    }
    # Every Go command counts, not only those inside a recognised job: one the
    # parser cannot place fails closed, so an unrecognised key spelling can
    # never switch the checks off. Any non-word character may precede the
    # token (a quote, `sh -c "`, a subshell) or follow the subcommand (`;`,
    # `&&`, `|`), not only whitespace. The subcommands are the full `go help`
    # list, `go.exe` is the same tool, and a flag after `go` (`go -C sub test`)
    # can only be the Go tool, so it counts whatever follows (#499).
    !is_name && cmd ~ /(^|[^[:alnum:]_.-])go(\.exe)?[[:space:]]+(-|(bug|build|clean|doc|env|fix|fmt|generate|get|install|list|mod|run|telemetry|test|tool|version|vet|work)([^[:alnum:]_.-]|$))/ {
      if (!job_setup) bad = 1
      job_go = 1
    }
    /^[[:space:]]*-[[:space:]]*uses:[[:space:]]+actions\/setup-go@/ {
      if (active && !valid) bad = 1
      if (in_job) job_setup = 1
      active = 1; valid = 0; with_indent = -1; child_indent = -1
      next
    }
    active && /^[[:space:]]*-[[:space:]]/ {
      if (!valid) bad = 1
      active = 0
    }
    active && /^[[:space:]]*with:[[:space:]]*$/ {
      with_indent = index($0, "with:") - 1; child_indent = -1; next
    }
    # A with: child sits at the depth of the first key below with:, whatever it is.
    active && with_indent >= 0 && child_indent < 0 && /^ +[^[:space:]#]/ &&
      indent_of($0) > with_indent { child_indent = indent_of($0) }
    active && with_indent >= 0 &&
      /^[[:space:]]*go-version-file:[[:space:]]*(\.\/)?go\.mod([[:space:]]*(#.*)?)?$/ {
      if (indent_of($0) == child_indent) valid = 1
    }
    active && with_indent >= 0 && /^[[:space:]]*go-version:[[:space:]]*/ {
      if (indent_of($0) == child_indent) bad = 1
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

# gosec is pinned in three places that must name one version (#528): the
# ci.yml `security` job installs it, mise.toml pins it for a local run, and
# security-audit.md tells the reader which one runs. Arguments: ci.yml,
# mise.toml, security-audit.md. Fails closed on `@latest`, a missing or second
# install line, a missing pin, or a second version in the doc, since any of them
# leaves a version unchecked.
check_gosec() {
  local installs ci_version mise_version
  installs="$(grep -E 'securego/gosec' "$1" | grep -Ev '^[[:space:]]*#' || true)"
  [ "$(printf '%s\n' "$installs" | grep -c .)" -eq 1 ] || return 1
  ci_version="$(printf '%s\n' "$installs" |
    sed -nE 's|.*go install github\.com/securego/gosec/v2/cmd/gosec@v([0-9]+\.[0-9]+\.[0-9]+)([[:space:]].*)?$|\1|p')"
  [ -n "$ci_version" ] || return 1
  mise_version="$(awk -F'"' '/^"aqua:securego\/gosec"[[:space:]]*=/ { print $4 }' "$2")"
  [ "$mise_version" = "$ci_version" ] || return 1
  # The doc names the pin, and no other version: a stale second mention misleads as much.
  [ "$(grep -oE 'gosec/v2/cmd/gosec@[^[:space:]`]*' "$3" | sort -u)" = "gosec/v2/cmd/gosec@v$ci_version" ]
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
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          cache: true
      - name: other
        uses: example/action@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a version in an unrelated named step" >&2
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
cat > "$tmp" <<'YAML'
"jobs":
  lint:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
  test:
    steps:
      - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: counted a setup-go outside any recognised job" >&2
  exit 1
fi
# The command token counts whatever precedes it: a quote, `sh -c`, or a `\`
# continuation that puts the subcommand on the next line.
for spelling in '"go test ./..."' "'go vet ./...'" "sh -c \"go build ./...\"" '(go test ./...)' \
  'sh -c "go test; echo done"' 'go vet&&echo done' 'go test|tee log'; do
  printf 'jobs:\n  build:\n    steps:\n      - run: %s\n' "$spelling" > "$tmp"
  if check_file "$tmp"; then
    echo "test-go-toolchain: accepted a Go job without setup-go ($spelling)" >&2
    exit 1
  fi
done
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: |
          go \
            test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a continued Go command without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
      - run: "go test ./..."
      - run: |
          go \
            vet ./...
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: rejected quoted and continued Go commands in a configured job" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    name: "go test"
    steps:
      - name: go vet
        run: echo lint
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
      - run: go vet ./...
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: read a job or step name as a Go command" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: cargo test && mongo run
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: matched go inside another command name" >&2
  exit 1
fi

# Depth comes from the file, not from a literal two spaces (#505).
cat > "$tmp" <<'YAML'
jobs:
    build:
        steps:
            - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
              with:
                  go-version-file: go.mod
            - run: go test ./...
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: rejected a 4-space-indented module-derived toolchain" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
    build:
        steps:
            - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
              with:
                  go-version-file: ./go.mod
            - run: go test ./...
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: rejected go-version-file: ./go.mod" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
    lint:
        steps:
            - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
              with:
                  go-version-file: go.mod
    build:
        steps:
            - run: go test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a 4-space-indented Go job without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
    build:
        steps:
            - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
              with:
                  go-version-file: go.mod
                  cache: true
                  go-version: '1.26'
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a pinned toolchain under a 4-space with:" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go -C sub test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted a flag before the subcommand without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go.exe test ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted the Windows binary name without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go get tool example.com/x@v1.0.0
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted go get without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go fix ./...
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted go fix without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go bug
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted go bug without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go doc fmt.Println
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted go doc without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - run: go telemetry off
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted go telemetry without setup-go" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          go-version-file: go.mod
      - run: go -C sub test ./...
YAML
if ! check_file "$tmp"; then
  echo "test-go-toolchain: rejected a flag before the subcommand in a configured job" >&2
  exit 1
fi
cat > "$tmp" <<'YAML'
jobs:
  build:
    steps:
      - uses: actions/setup-go@0123456789abcdef0123456789abcdef01234567
        with:
          cache: true
          options:
            go-version-file: go.mod
YAML
if check_file "$tmp"; then
  echo "test-go-toolchain: accepted go-version-file nested below a with: child" >&2
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

# gosec pin: CI, mise.toml and the audit doc must name one exact version (#528).
gosec_line='          go install github.com/securego/gosec/v2/cmd/gosec@v2.30.0'
printf '%s\n' "$gosec_line" > "$tmp_ci"
printf '[tools]\n"aqua:securego/gosec" = "2.30.0"\n' > "$tmp_mise"
printf 'installed with go install github.com/securego/gosec/v2/cmd/gosec@v2.30.0\n' > "$tmp_docs"
if ! check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: rejected the matching gosec pins" >&2
  exit 1
fi
printf '%s\n' "$gosec_line # pinned" > "$tmp_ci"
if ! check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: rejected a gosec pin followed by a comment" >&2
  exit 1
fi
printf '[tools]\n"aqua:securego/gosec" = "2.29.0"\n' > "$tmp_mise"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a local gosec pin that differs from CI" >&2
  exit 1
fi
printf '[tools]\n' > "$tmp_mise"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a missing local gosec pin" >&2
  exit 1
fi
printf '[tools]\n"aqua:securego/gosec" = "2.30.0"\n' > "$tmp_mise"
printf '          go install github.com/securego/gosec/v2/cmd/gosec@latest\n' > "$tmp_ci"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted an unpinned gosec install (@latest)" >&2
  exit 1
fi
printf '          go install github.com/securego/gosec/v2/cmd/gosec@v2\n' > "$tmp_ci"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a partial gosec version (@v2)" >&2
  exit 1
fi
printf '          gosec ./...\n' > "$tmp_ci"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a workflow with no gosec install line" >&2
  exit 1
fi
printf '%s\n%s\n' "$gosec_line" '          go install github.com/securego/gosec/v2/cmd/gosec@latest' > "$tmp_ci"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted a second, unpinned gosec install" >&2
  exit 1
fi
printf '%s\n' "$gosec_line" > "$tmp_ci"
printf 'installed with go install github.com/securego/gosec/v2/cmd/gosec@latest\n' > "$tmp_docs"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted an audit doc that does not name the pinned gosec" >&2
  exit 1
fi
printf '%s\n%s\n' 'installed with go install github.com/securego/gosec/v2/cmd/gosec@v2.30.0' \
  'formerly go install github.com/securego/gosec/v2/cmd/gosec@v2.22.0' > "$tmp_docs"
if check_gosec "$tmp_ci" "$tmp_mise" "$tmp_docs"; then
  echo "test-go-toolchain: accepted an audit doc that also names a stale gosec" >&2
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
if ! check_gosec "$ROOT/.github/workflows/ci.yml" "$ROOT/mise.toml" "$ROOT/docs/troubleshooting/security-audit.md"; then
  echo "test-go-toolchain: the gosec pin in ci.yml, mise.toml and security-audit.md disagree or is not exact" >&2
  exit 1
fi
echo "test-go-toolchain: OK under $TGT_SHELL ($count Go workflows)"
