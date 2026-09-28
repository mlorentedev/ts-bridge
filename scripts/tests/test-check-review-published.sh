#!/usr/bin/env bash
# Tests for scripts/check-review-published.sh (CI-345) -- the guard that fails a PR
# when PR-Agent reported green but published no review (#1107).
#
# Each fixture stubs `gh` with a tiny dispatcher so the test needs no network and no
# live PR: the three `gh api` calls the guard makes (registry lookup, PR head lookup,
# paginated comment list) are answered from fixture JSON placed on `PATH` ahead of the
# real `gh`, following the same "stub the CLI, not the guard" approach the CI-314
# adversarial review used to find this bug in the first place.
#
# The load-bearing case is `null-body`: one comment in the paginated list has
# `"body": null` (GitHub returns this for some edited/reaction-only comments). Before
# the fix, `.body | contains($marker)` aborted `jq` with exit 5 on that comment and the
# step failed with a bare jq stack trace instead of its own diagnostic. The fixture
# also carries a second, later comment that DOES carry the marker, so the assertion is
# that the null comment is skipped rather than that it kills the whole filter.
#
# Fixture shape matters: `--paginate` emits ONE raw JSON array per page on stdout, and
# the guard's own `jq -s` is what slurps however many pages arrive into an
# array-of-arrays. A fixture must therefore be a single flat array (one page), never
# pre-wrapped in an extra `[...]`, or it no longer represents what `--paginate` emits.
#
# Run: bash scripts/tests/test-check-review-published.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
GUARD="$ROOT/scripts/check-review-published.sh"
[ -f "$GUARD" ] || { echo "test-check-review-published: guard not found at $GUARD" >&2; exit 2; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

MARKER='## PR Reviewer Guide'
STARTED='2026-09-23T00:00:00Z'

registry_json() {
  cat <<JSON
{"reviewers":[{"login":"github-actions","review_markers":["${MARKER}"]}]}
JSON
}

# $1 = fixture name -> writes $WORK/comments.json, exactly one page (a flat array).
write_comments() {
  case "$1" in
    null-body)
      # First comment has a null body (must not abort the filter); second carries the
      # marker and is updated after $STARTED.
      cat > "$WORK/comments.json" <<JSON
[
  {"user":{"login":"github-actions[bot]"},"updated_at":"2026-09-23T00:00:05Z","body":null},
  {"user":{"login":"github-actions[bot]"},"updated_at":"2026-09-23T00:00:10Z","body":"${MARKER}\n\nLGTM"}
]
JSON
      ;;
    only-null-body)
      # Only a null-body comment exists -- the guard must still fail closed (no
      # match), not crash.
      cat > "$WORK/comments.json" <<JSON
[
  {"user":{"login":"github-actions[bot]"},"updated_at":"2026-09-23T00:00:05Z","body":null}
]
JSON
      ;;
    clean-match)
      cat > "$WORK/comments.json" <<JSON
[
  {"user":{"login":"github-actions[bot]"},"updated_at":"2026-09-23T00:00:10Z","body":"${MARKER}\n\nLGTM"}
]
JSON
      ;;
    no-match)
      cat > "$WORK/comments.json" <<JSON
[
  {"user":{"login":"someone-else"},"updated_at":"2026-09-23T00:00:10Z","body":"${MARKER}"}
]
JSON
      ;;
  esac
}

# A minimal `gh` stub: routes on the API path suffix the guard requests. Emits
# base64(registry.json) for the registry lookup -- equivalent to what the real
# `gh api ... --jq '.content'` returns (the Contents API's base64 file body), so the
# guard's own `base64 -d` decodes it identically either way.
write_gh_stub() {
  cat > "$WORK/gh" <<'STUB'
#!/usr/bin/env bash
set -euo pipefail
if [ "$1" = "api" ]; then
  path="$2"
  case "$path" in
    */contents/harness/review-attestation.json*)
      base64 < "$FIXTURE_REGISTRY" | tr -d '\n'
      echo
      ;;
    */pulls/*)
      echo '{"head":{"sha":"deadbeef"}}'
      ;;
    */issues/*/comments)
      cat "$FIXTURE_COMMENTS"
      ;;
    *)
      echo "gh-stub: unhandled path: $path" >&2
      exit 1
      ;;
  esac
else
  echo "gh-stub: unhandled command: $*" >&2
  exit 1
fi
STUB
  chmod +x "$WORK/gh"
}

run_case() { # $1=fixture $2=expected_exit $3=expected_substring
  local fixture="$1" want_exit="$2" want_sub="$3"
  write_comments "$fixture"
  registry_json > "$WORK/registry.json"
  export FIXTURE_REGISTRY="$WORK/registry.json"
  export FIXTURE_COMMENTS="$WORK/comments.json"

  out=$(PATH="$WORK:$PATH" \
    GH_TOKEN=x GITHUB_REPOSITORY=owner/repo PR_NUMBER=1 BASE_REF=master \
    STARTED="$STARTED" HEAD_SHA=deadbeef \
    bash "$GUARD" 2>&1) && got_exit=0 || got_exit=$?

  if [ "$got_exit" != "$want_exit" ]; then
    echo "FAIL [$fixture]: exit $got_exit, want $want_exit. Output:" >&2
    echo "$out" >&2
    return 1
  fi
  if ! grep -qF -- "$want_sub" <<<"$out"; then
    echo "FAIL [$fixture]: output missing \"$want_sub\". Output:" >&2
    echo "$out" >&2
    return 1
  fi
  echo "ok [$fixture] -> exit $got_exit"
}

write_gh_stub

fail=0
run_case null-body       0 "review published"    || fail=1
run_case only-null-body  1 "published no review" || fail=1
run_case clean-match     0 "review published"    || fail=1
run_case no-match        1 "published no review" || fail=1

if [ "$fail" -ne 0 ]; then
  echo "test-check-review-published: FAILED" >&2
  exit 1
fi
echo "test-check-review-published: all cases ok"