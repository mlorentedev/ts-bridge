#!/usr/bin/env bash
# Contract guard for the wiring of the conflict-marker gate in .github/workflows/repo-hygiene.yml.
#
# The gate blocks a merge only while its guard and fixture suite run as plain steps of the
# required `hygiene` job. `continue-on-error` (on a step or on the job), an `if:` or a `needs:`
# keeps every line in place while a failure stops failing the job (#514). This check runs as
# its own step, so one edit cannot switch off both the gate and the check of its wiring.
#
# Structure, not spelling (lesson 036): comments are stripped, step lines are keyed by their
# parent, and `continue-on-error` is matched as a key whatever its value. The AI-trailer step
# carries a legitimate `if:`, so that rule covers only the guarded steps and the job.
#
# Run: bash scripts/tests/test-hygiene-wiring.sh [workflow]  (default: the real repo-hygiene.yml)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="${1:-$ROOT/.github/workflows/repo-hygiene.yml}"
FIXTURES="$ROOT/scripts/tests/fixtures/hygiene-wiring"
JOB='hygiene'
GUARDED='bash scripts/check-conflict-markers.sh
bash scripts/tests/test-conflict-markers.sh'
# shellcheck source-path=SCRIPTDIR source=helpers/workflow_yaml.bash
. "$ROOT/scripts/tests/helpers/workflow_yaml.bash"

# Print the step record (one line per key, as job_steps emits it) whose own `run:` is exactly $1.
guarded_step() {
  awk -v cmd="run: $1" '
    function flush() { if (hit) { printf "%s", rec; found = 1; exit } rec = ""; hit = 0 }
    /^@@STEP$/ { flush(); next }
    { sub(/[[:space:]]+$/, ""); rec = rec $0 "\n"; if ($0 == cmd) hit = 1 }
    END { if (!found) flush() }
  '
}

check_workflow() {
  local file="$1" text steps step cmd
  [ -f "$file" ] || { echo "workflow not found at $file"; return 1; }
  # CR is line-ending spelling: a CRLF checkout must read the same as LF.
  text="$(tr -d '\r' <"$file" | strip_comments /dev/stdin)"
  steps="$(printf '%s\n' "$text" | job_steps "$JOB")"
  [ -n "$steps" ] || { echo "missing contract: job '$JOB'"; return 1; }

  # Matched as a key, never by value: `false` today is one edit from `true`, and an expression
  # can be true on exactly the runs that matter. A step's first key follows its `- `.
  ! grep -qE '^[[:space:]]*(-[[:space:]]+)?continue-on-error:' <<<"$text" ||
    { echo "forbidden: continue-on-error in $(basename "$file")"; return 1; }
  ! grep -qE '^job\.if:' <<<"$steps" || { echo "forbidden: if condition on job '$JOB'"; return 1; }
  ! grep -qE '^job\.needs:' <<<"$steps" || { echo "forbidden: needs on job '$JOB'"; return 1; }

  while IFS= read -r cmd; do
    step="$(printf '%s\n' "$steps" | guarded_step "$cmd")"
    [ -n "$step" ] || { echo "missing step '$cmd' in job '$JOB'"; return 1; }
    ! grep -qE '^if:' <<<"$step" || { echo "forbidden: if condition on '$cmd'"; return 1; }
  done <<<"$GUARDED"
  echo "OK"
}

# fixture:expected_exit:expected_substring
SPECS="valid:0:OK
comment-mentions-coe:0:OK
crlf-valid:0:OK
matrix-before-steps:0:OK
guard-coe:1:continue-on-error
suite-coe:1:continue-on-error
job-coe:1:continue-on-error
other-step-coe-expression:1:continue-on-error
guard-coe-false:1:continue-on-error
crlf-guard-coe:1:continue-on-error
guard-if:1:if condition on 'bash scripts/check-conflict-markers.sh'
suite-if:1:if condition on 'bash scripts/tests/test-conflict-markers.sh'
job-if:1:if condition on job 'hygiene'
job-needs:1:needs on job 'hygiene'
guard-missing:1:missing step 'bash scripts/check-conflict-markers.sh'
suite-missing:1:missing step 'bash scripts/tests/test-conflict-markers.sh'
guard-under-env:1:missing step 'bash scripts/check-conflict-markers.sh'
guard-in-comment:1:missing step 'bash scripts/check-conflict-markers.sh'"

failed=0
count=0
while IFS= read -r spec; do
  [ -n "$spec" ] || continue
  name="${spec%%:*}"; rest="${spec#*:}"; want="${rest%%:*}"; want_sub="${rest#*:}"
  got="$(check_workflow "$FIXTURES/$name.yml")" && rc=0 || rc=$?
  count=$((count + 1))
  if [ "$rc" -ne "$want" ] || [[ "$got" != *"$want_sub"* ]]; then
    echo "  FAIL $name: exit $rc (wanted $want), output: $got" >&2
    failed=1
  fi
done <<<"$SPECS"
[ "$failed" -eq 0 ] || { echo "test-hygiene-wiring: fixture suite failed" >&2; exit 1; }

got="$(check_workflow "$WORKFLOW")" || { echo "test-hygiene-wiring: $got" >&2; exit 1; }
echo "test-hygiene-wiring: OK ($count fixtures + ${WORKFLOW#"$ROOT"/})"
