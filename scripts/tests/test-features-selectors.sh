#!/usr/bin/env bash
# Tests for scripts/check-features-selectors.sh -- the check that a features.json `go test -run`
# selector still names a test (#575).
#
# The consumer is the harness rule "exit 0 means the feature may be set passing". `go test` exits
# 0 when `-run` selects nothing, so a renamed test turns the command into one that checks
# nothing. Each case builds a throwaway tree (specs/ plus the packages its commands name), so no
# committed spec has to be deliberately wrong. `matching` and the other controls stop a guard
# that fails everything; `lost-alternative` is the BUG-530 shape: one of several alternatives
# is renamed while the others keep the command green.
#
# Every case runs under bash and zsh: a guard that fails differently per shell gets deleted.
#
# Run: bash scripts/tests/test-features-selectors.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-features-selectors.sh"
[ -f "$GUARD" ] || { echo "test-features-selectors: guard not found at $GUARD" >&2; exit 2; }

WORK="$(mktemp -d)"
trap 'chmod -R u+rwx "$WORK" 2>/dev/null; rm -rf "$WORK"' EXIT

# put <case> <path> <content>: a file under the case's tree.
put() {
  mkdir -p "$WORK/$1/$(dirname "$2")"
  printf '%s\n' "$3" >"$WORK/$1/$2"
}

# spec <case> <path under specs/> <verification command>: a one-feature spec.
spec() {
  local cmd
  cmd="$(jq -Rn --arg c "$3" '$c')"
  put "$1" "specs/$2/features.json" "[{\"id\":\"X-f1\",\"behavior\":\"b\",\"verification\":$cmd,\"state\":\"pending\",\"evidence\":\"\"}]"
}

# gopkg <case> <dir> <test funcs...>: a package whose test file declares the named functions.
gopkg() {
  local c="$1" dir="$2" body="package p" fn
  shift 2
  for fn in "$@"; do body="$body
func $fn(t *testing.T) {}"; done
  put "$c" "$dir/p_test.go" "$body"
}

# One flat fixture table: each arm is straight-line setup with no shared branching, so it stays
# in one place rather than following the 40-line / CC-10 rule.
build() {
  local c="$1"
  mkdir -p "$WORK/$c/specs"
  case "$c" in
    matching) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run '^TestA\$' -count=1" ;;
    no-specs) ;;
    zero-match) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run 'TestRenamed' -count=1" ;;
    lost-alternative)
      gopkg "$c" pkg TestA TestB
      spec "$c" A "go test ./pkg -run 'TestA|TestGone|TestB' -count=1" ;;
    anchored-alternative)
      gopkg "$c" pkg TestA TestB
      spec "$c" A "go test ./pkg -run '^TestA\$|^TestBgone\$' -count=1" ;;
    grouped-alternation) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run '^(TestA|TestGone)\$'" ;;
    equals-form) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run=TestGone" ;;
    double-quoted) gopkg "$c" pkg TestA; spec "$c" A 'go test ./pkg -run "TestGone"' ;;
    chained-second-bad)
      gopkg "$c" pkg TestA
      spec "$c" A "go test ./pkg -run '^TestA\$' -v | grep -q -- '--- PASS: TestA ' && go test ./pkg -run '^TestGone\$' -v | grep -q -- '--- PASS: TestGone '" ;;
    pipe-in-grep-only)
      gopkg "$c" pkg TestA
      spec "$c" A "go test ./pkg -run '^TestA\$' -v | grep -Eq 'PASS|ok'" ;;
    no-run-flag) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -count=1" ;;
    empty-run) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run ''" ;;
    not-go-test) spec "$c" A "bash scripts/tests/test-x.sh" ;;
    env-prefix) gopkg "$c" pkg TestA; spec "$c" A "GOOS=windows go test ./pkg -run 'TestGone'" ;;
    guarded-os-step)
      gopkg "$c" pkg TestA
      spec "$c" A "[ \"\$(go env GOOS)\" = windows ] && go test ./pkg -run '^TestGone\$'" ;;
    subtest-ok) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run 'TestA/sub'" ;;
    subtest-bad) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run 'TestGone/sub'" ;;
    other-os-file)
      put "$c" pkg/win_windows_test.go "package p
func TestOnlyWindows(t *testing.T) {}"
      spec "$c" A "go test ./pkg -run '^TestOnlyWindows\$'" ;;
    kinds)
      gopkg "$c" pkg FuzzF ExampleE
      spec "$c" A "go test ./pkg -run 'FuzzF|ExampleE'" ;;
    benchmark-is-not-run) gopkg "$c" pkg TestA BenchmarkB; spec "$c" A "go test ./pkg -run '^BenchmarkB\$'" ;;
    multi-line)
      gopkg "$c" one TestA
      gopkg "$c" two TestB
      spec "$c" A "go test ./one -run '^TestGone\$'
go test ./two -run '^TestB\$'" ;;
    method-is-not-a-test)
      put "$c" pkg/p_test.go "package p
func (s *S) TestMethod(t *testing.T) {}"
      spec "$c" A "go test ./pkg -run 'TestMethod'" ;;
    missing-package) spec "$c" A "go test ./nope -run 'TestA'" ;;
    tree-wildcard)
      gopkg "$c" pkg/sub TestDeep
      spec "$c" A "go test ./pkg/... -run '^TestDeep\$'" ;;
    two-packages)
      gopkg "$c" one TestA
      gopkg "$c" two TestB
      spec "$c" A "go test ./one ./two -run 'TestA|TestB'" ;;
    archived-bad) gopkg "$c" pkg TestA; spec "$c" archive/A "go test ./pkg -run 'TestGone'" ;;
    only-bad-named)
      gopkg "$c" pkg TestA
      spec "$c" good "go test ./pkg -run 'TestA'"
      spec "$c" bad "go test ./pkg -run 'TestGone'" ;;
    invalid-json) put "$c" specs/A/features.json '[{"id":' ;;
    bad-regexp) gopkg "$c" pkg TestA; spec "$c" A "go test ./pkg -run 'Test(A'" ;;
    not-a-dir) rmdir "$WORK/$c/specs"; : >"$WORK/$c/specs" ;;
  esac
}

# name:expected_exit:required substring
SPECS='matching:0:OK (1 go test
no-specs:0:OK (0 go test
zero-match:1:matches no test: TestRenamed
lost-alternative:1:matches no test: TestGone
anchored-alternative:1:matches no test: ^TestBgone$
grouped-alternation:0:OK (1 go test
equals-form:1:matches no test: TestGone
double-quoted:1:matches no test: TestGone
chained-second-bad:1:matches no test: ^TestGone$
pipe-in-grep-only:0:OK (1 go test
no-run-flag:0:OK (0 go test
empty-run:0:OK (0 go test
not-go-test:0:OK (0 go test
env-prefix:1:matches no test: TestGone
guarded-os-step:1:matches no test: ^TestGone$
subtest-ok:0:OK (1 go test
subtest-bad:1:matches no test: TestGone
other-os-file:0:OK (1 go test
kinds:0:OK (1 go test
benchmark-is-not-run:1:matches no test: ^BenchmarkB$
multi-line:1:matches no test: ^TestGone$ (in ./one)
method-is-not-a-test:1:matches no test: TestMethod
missing-package:1:no such package: ./nope
tree-wildcard:0:OK (1 go test
two-packages:0:OK (1 go test
archived-bad:0:OK (0 go test
only-bad-named:1:bad/features.json
invalid-json:2:cannot read
bad-regexp:2:cannot evaluate
not-a-dir:2:not a directory'

# name:substring that must NOT appear in the output: the report names the lost alternative
# and no other, and an archived spec is not read.
ABSENT='lost-alternative:matches no test: TestA
lost-alternative:matches no test: TestB
only-bad-named:good/features.json
archived-bad:TestGone'

failed=0
count=0

run_cases() { # run_cases <shell>
  local sh_name="$1" spec_line name rest want want_sub got rc absent
  if ! command -v "$sh_name" >/dev/null 2>&1; then
    echo "  FAIL [$sh_name] not installed: both shells are required" >&2
    failed=1
    return 0
  fi
  while IFS= read -r spec_line; do
    name="${spec_line%%:*}"
    rest="${spec_line#*:}"
    want="${rest%%:*}"
    want_sub="${rest#*:}"
    [ -d "$WORK/$name" ] || build "$name"
    got="$("$sh_name" "$GUARD" "$WORK/$name/specs" 2>&1)" && rc=0 || rc=$?
    count=$((count + 1))
    if [ "$rc" -ne "$want" ]; then
      echo "  FAIL [$sh_name] $name: exit $rc, wanted $want"; echo "$got"; failed=1; continue
    fi
    case "$got" in
      *"$want_sub"*) echo "  ok   [$sh_name] $name -> exit $rc, mentions \"$want_sub\"" ;;
      *) echo "  FAIL [$sh_name] $name: output does not mention '$want_sub'"; echo "$got"; failed=1 ;;
    esac
    while IFS= read -r absent; do
      [ -n "$absent" ] || continue
      case "$got" in
        *"$absent"*) echo "  FAIL [$sh_name] $name: output mentions '$absent'"; echo "$got"; failed=1 ;;
      esac
    done <<EOF
$(printf '%s\n' "$ABSENT" | awk -F: -v n="$name" '$1 == n { print substr($0, length(n) + 2) }')
EOF
  done <<EOF
$SPECS
EOF
}

# zsh does not word-split an unquoted expansion, so the shell list is read line by line.
while IFS= read -r sh_name; do
  run_cases "$sh_name"
done <<EOF
$(printf '%s\n' "${FJS_SHELLS:-bash zsh}" | tr ' ' '\n')
EOF

# Detect only: the check reads a spec and never rewrites it.
before="$(cat "$WORK/zero-match/specs/A/features.json")"
bash "$GUARD" "$WORK/zero-match/specs" >/dev/null 2>&1 || true
after="$(cat "$WORK/zero-match/specs/A/features.json")"
if [ "$before" != "$after" ]; then echo "  FAIL features.json was modified"; failed=1; fi

# Fail closed without jq: no JSON reader must be an error, never a clean tree.
empty_path="$WORK/empty-path"
mkdir -p "$empty_path"
got="$(PATH="$empty_path" "$(command -v bash)" "$GUARD" "$WORK/matching/specs" 2>&1)" && rc=0 || rc=$?
case "$rc:$got" in
  2:*jq*) echo "  ok   [bash] without jq -> exit 2" ;;
  *) echo "  FAIL without jq: exit $rc, wanted 2 naming jq"; echo "$got"; failed=1 ;;
esac

if [ "$failed" -eq 0 ]; then echo "test-features-selectors: OK ($count cases)"; fi
exit "$failed"
