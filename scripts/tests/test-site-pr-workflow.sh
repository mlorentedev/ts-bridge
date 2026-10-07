#!/usr/bin/env bash
# Contract guard for .github/workflows/site-pr.yml, the pull-request build of the docs site.
#
# Why structure and not text: the first version grepped the whole file, so it proved only that
# the strings existed somewhere. The CI-344 adversarial review found three mutations it passed —
# the job renamed, `npm ci` kept alive only in a comment, and the checkout moved into another job
# — and two spellings it failed for no reason: a comment that names a forbidden capability, and
# `node-version: 22` without quotes. So this version strips comments, cuts out the `site-build`
# job, and asserts on each step of it. Round 2 found the same flaw in the two blocks still
# grepped (permissions, path filters): permissions are now an allow-list and the filters must sit
# under on.pull_request.paths. Round 3 found step lines still matched as text (a `run:` parked
# under `env:` passed), an `if:` that switches the build off, a negated path filter and CRLF; each
# step line is now keyed by its parent, and `if:` and extra filters are findings.
#
# Why fixtures: a guard pointed at one real tree only proves "today it is quiet". Each fixture
# under scripts/tests/fixtures/site-pr/ is one way the contract can break (expected red) or one
# harmless spelling it must accept (expected green); `valid.yml` is the control.
#
# Run: bash scripts/tests/test-site-pr-workflow.sh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/site-pr.yml"
FIXTURES="$ROOT/scripts/tests/fixtures/site-pr"
PAGES_ACTION_RE='actions/(upload|deploy)-pages([-@]|$)'
JOB='site-build'

# Comments are prose, never contract: drop full-line comments and trailing ` # ...`.
strip_comments() {
  sed -E -e '/^[[:space:]]*#/d' -e 's/[[:space:]]+#.*$//' "$1"
}

# Print the children of the block at a /-separated key path (e.g. on/pull_request/paths),
# indentation trimmed; a value on the key's own line is printed first. Nothing if absent.
yaml_block() {
  awk -v path="$1" '
    BEGIN { n = split(path, keys, "/"); depth = 1; want = 0; parent = -1 }
    /^[[:space:]]*$/ { next }
    {
      match($0, /^[[:space:]]*/); ind = RLENGTH; line = substr($0, ind + 1)
      if (inside) { if (ind > base) { print line; next } exit }
      if (ind <= parent) exit
      if (want < 0) want = ind
      if (ind != want || index(line, keys[depth] ":") != 1) next
      if (depth == n) {
        inside = 1; base = ind
        rest = substr(line, length(keys[depth]) + 2); sub(/^[[:space:]]+/, "", rest)
        if (rest != "") print rest
        next
      }
      depth++; parent = ind; want = -1
    }
  '
}

# Print the body of job $JOB, one step per record: every step begins with a line `@@STEP`.
# Lines are keyed by where they sit, not by what they say: a step key prints bare (`run: npm ci`),
# anything nested under one prints with its parent (`with.persist-credentials: false`,
# `env.run: npm ci`), and a job-level key prints as `job.<key>`. So `^run:` matches only a
# step's own command, never a look-alike parked under `env:` or `with:`.
job_steps() {
  awk -v job="$JOB" '
    /^jobs:[[:space:]]*$/ { in_jobs = 1; next }
    in_jobs && /^[^[:space:]]/ { in_jobs = 0; in_job = 0 }
    in_jobs && /^  [^[:space:]][^:]*:[[:space:]]*$/ {
      key = $0; sub(/^  /, "", key); sub(/:[[:space:]]*$/, "", key)
      in_job = (key == job); step_indent = ""; next
    }
    in_job && !/^[[:space:]]*$/ {
      match($0, /^[[:space:]]*/); ind = RLENGTH; body = substr($0, ind + 1)
      if (body ~ /^- / && (step_indent == "" || ind == step_indent)) {
        step_indent = ind; print "@@STEP"
        body = substr(body, 3); sub(/^[[:space:]]+/, "", body); ind += 2
      }
      if (step_indent == "" || ind <= step_indent) { print "job." body; next }
      if (ind == step_indent + 2) { parent = body; sub(/:.*/, "", parent); print body; next }
      print parent "." body
    }
  '
}

# Exit 0 when a single step of the job has a line matching each ERE given.
step_has() {
  awk '
    BEGIN { for (i = 1; i < ARGC; i++) re[i] = ARGV[i]; n = ARGC - 1; ARGC = 1 }
    function flush(  i, j, hit) {
      if (!lines) return
      for (i = 1; i <= n; i++) {
        hit = 0
        for (j = 1; j <= lines; j++) if (line[j] ~ re[i]) { hit = 1; break }
        if (!hit) return
      }
      found = 1
    }
    /^@@STEP$/ { flush(); lines = 0; next }
    { line[++lines] = $0 }
    END { flush(); exit(found ? 0 : 1) }
  ' "$@"
}

check_workflow() {
  local file="$1" text steps raw_steps action
  [ -f "$file" ] || { echo "workflow not found at $file"; return 1; }
  # Quotes are spelling, not structure: `"id-token": write` must read as `id-token: write`.
  # CR is line-ending spelling too: a CRLF checkout must read the same as LF.
  text="$(strip_comments "$file" | tr -d "\"'\r")"
  steps="$(printf '%s\n' "$text" | job_steps)"
  raw_steps="$(tr -d '\r' <"$file" | job_steps)"

  paths="$(yaml_block on/pull_request/paths <<<"$text")"
  [ -n "$(yaml_block on/pull_request <<<"$text")" ] || { echo "missing contract: pull_request trigger"; return 1; }
  grep -qxE -- '- site/\*\*' <<<"$paths" || { echo "missing contract: site path filter under on.pull_request.paths"; return 1; }
  grep -qxE -- '- \.github/workflows/site-pr\.yml' <<<"$paths" || { echo "missing contract: self path filter under on.pull_request.paths"; return 1; }
  # Allow-list: any third entry (a `!site/**` negation above all) can only narrow the trigger.
  [ "$(grep -c . <<<"$paths")" -eq 2 ] || { echo "missing contract: on.pull_request.paths must hold exactly the two filters"; return 1; }

  ! grep -qE 'pages:[[:space:]]*write' <<<"$text" || { echo "forbidden PR capability: Pages write permission"; return 1; }
  ! grep -qE 'id-token:[[:space:]]*write' <<<"$text" || { echo "forbidden PR capability: OIDC write permission"; return 1; }
  ! grep -qE "$PAGES_ACTION_RE" <<<"$text" || { echo "forbidden PR capability: Pages upload or deployment action"; return 1; }

  # Allow-list, not presence: the workflow grants exactly `contents: read`, and no job widens it.
  [ "$(yaml_block permissions <<<"$text")" = "contents: read" ] ||
    { echo "missing contract: top-level permissions must be exactly contents: read"; return 1; }
  ! yaml_block jobs <<<"$text" | grep -qE '^permissions:' ||
    { echo "forbidden PR capability: job-level permissions"; return 1; }

  [ -n "$steps" ] || { echo "missing contract: job '$JOB'"; return 1; }
  # A condition on the job or on any step can switch the build off while every line stays put.
  ! grep -qE '^(job\.)?if:' <<<"$steps" || { echo "forbidden PR capability: if condition in job '$JOB'"; return 1; }
  # continue-on-error turns a failed build into a green one, on a step or on the whole job.
  ! grep -qE '^(job\.)?continue-on-error:' <<<"$steps" ||
    { echo "forbidden PR capability: continue-on-error in job '$JOB'"; return 1; }
  step_has '^uses:[[:space:]]*actions/checkout@[0-9a-f]{40}' '^with\.persist-credentials:[[:space:]]*false[[:space:]]*$' <<<"$steps" ||
    { echo "missing contract: credential-less checkout in job '$JOB'"; return 1; }
  step_has '^uses:[[:space:]]*actions/setup-node@[0-9a-f]{40}' "^with\\.node-version:[[:space:]]*['\"]?22['\"]?[[:space:]]*$" <<<"$steps" ||
    { echo "missing contract: Node 22 setup in job '$JOB'"; return 1; }
  step_has '^run:[[:space:]]*npm ci[[:space:]]*$' '^working-directory:[[:space:]]*site[[:space:]]*$' <<<"$steps" ||
    { echo "missing contract: npm ci under site/ in job '$JOB'"; return 1; }
  step_has '^run:[[:space:]]*npm run build[[:space:]]*$' '^working-directory:[[:space:]]*site[[:space:]]*$' <<<"$steps" ||
    { echo "missing contract: site build under site/ in job '$JOB'"; return 1; }

  # The version comment is the one comment that is contract: it names what the SHA pins.
  for action in checkout setup-node; do
    grep -qE "actions/$action@[0-9a-f]{40}[[:space:]]+# v[0-9]+" <<<"$raw_steps" ||
      { echo "missing contract: SHA-pinned $action with a version comment in job '$JOB'"; return 1; }
  done
  echo "OK"
}

for action in 'actions/upload-pages-artifact@sha' 'actions/deploy-pages@sha'; do
  grep -qE -- "$PAGES_ACTION_RE" <<<"$action" ||
    { echo "test-site-pr-workflow: Pages action matcher does not recognize $action" >&2; exit 1; }
done

# fixture:expected_exit:expected_substring
SPECS="valid:0:OK
node-unquoted:0:OK
comment-mentions-forbidden:0:OK
renamed-job:1:job 'site-build'
npm-ci-in-comment:1:npm ci under site/
checkout-other-job:1:credential-less checkout
npm-ci-outside-site:1:npm ci under site/
persists-credentials:1:credential-less checkout
pages-write:1:Pages write permission
elevated-permissions:1:exactly contents: read
read-all-permissions:1:exactly contents: read
job-permissions:1:job-level permissions
quoted-permissions:1:OIDC write permission
paths-misplaced:1:site path filter under on.pull_request.paths
paths-negated:1:exactly the two filters
run-under-env:1:npm ci under site/
persist-under-env:1:credential-less checkout
step-if:1:if condition
job-if:1:if condition
step-continue-on-error:1:continue-on-error
job-continue-on-error:1:continue-on-error
crlf:0:OK"

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
[ "$failed" -eq 0 ] || { echo "test-site-pr-workflow: fixture suite failed" >&2; exit 1; }

got="$(check_workflow "$WORKFLOW")" || { echo "test-site-pr-workflow: $got" >&2; exit 1; }
echo "test-site-pr-workflow: OK ($count fixtures + .github/workflows/site-pr.yml)"
