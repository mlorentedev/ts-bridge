#!/usr/bin/env bash
# Extracted from the "Fail if no review was published" step of
# .github/workflows/pr-agent.yml (CI-345) so the guard can be tested without a live
# PR-Agent run and a live `gh` session.
#
# A review that did not happen must not report success (#1107): PR-Agent swallows a
# failed inference into a clean exit, so this step re-derives the verdict from GitHub's
# own comment history instead of trusting the action's exit code. See pr-agent.yml for
# the full rationale (marker provenance, author/timestamp binding).
#
# Bug fixed here (#345): a `gh api` comment with `body: null` (no text, e.g. a reaction-
# only or edited-to-empty comment) made `.body | contains($marker)` abort the whole `jq`
# invocation with exit 5 instead of just failing to match that one comment -- turning a
# red job with a remedy into a red job showing a bare jq stack trace. `.body // ""`
# makes a null body fail the `contains` check instead of aborting the filter.
#
# Required env: GH_TOKEN, GITHUB_REPOSITORY, PR_NUMBER, BASE_REF, STARTED.
# Optional env: HEAD_SHA (falls back to an API call keyed on PR_NUMBER if unset).
set -euo pipefail

read_marker() { # $1 = ref; prints the marker or nothing; never aborts the step
  gh api "repos/${GITHUB_REPOSITORY}/contents/harness/review-attestation.json?ref=$1" \
    --jq '.content' 2>/dev/null | base64 -d 2>/dev/null \
    | jq -r '.reviewers[] | select(.login == "github-actions") | .review_markers[0] // empty' 2>/dev/null || true
}

marker=$(read_marker "${BASE_REF}")
if [ -z "$marker" ] && [ -z "${HEAD_SHA:-}" ]; then
  # issue_comment events carry no pull_request payload; ask the API for the head.
  HEAD_SHA=$(gh api "repos/${GITHUB_REPOSITORY}/pulls/${PR_NUMBER}" --jq '.head.sha' 2>/dev/null || true)
fi
if [ -z "$marker" ] && [ -n "${HEAD_SHA:-}" ]; then
  # The default branch has no github-actions entry yet: this is the PR that
  # introduces it. The head registry only proves the entry EXISTS; the marker
  # itself is PR-Agent's own heading, never text a PR could choose (a head-
  # supplied marker such as "#" would match any bot comment, CWE-345).
  if [ -n "$(read_marker "${HEAD_SHA}")" ]; then
    marker="PR Reviewer Guide"
    echo "::notice::reviewer registry entry found only on the PR head;" \
      "using PR-Agent's own heading as the marker"
  fi
fi
if [ -z "$marker" ] || [ "$marker" = "null" ]; then
  echo "::error::no review marker declared for github-actions in" \
    "harness/review-attestation.json (checked ${BASE_REF} and the PR head)"
  exit 1
fi

# --paginate emits one JSON array per page; `jq -s` slurps them into one array of
# arrays so the count spans every page.
found=$(gh api "repos/${GITHUB_REPOSITORY}/issues/${PR_NUMBER}/comments" --paginate \
  | jq -s --arg started "${STARTED}" --arg marker "${marker}" \
      '[.[][] | select(.user.login == "github-actions[bot]"
                       and (.updated_at >= $started)
                       and ((.body // "") | contains($marker)))] | length')
if [ "$found" -gt 0 ]; then
  echo "review published (marker: ${marker})"
  exit 0
fi
echo "::error::PR-Agent reported success but published no review" \
  "(no github-actions[bot] comment updated since ${STARTED} carries \"${marker}\")."
echo "::error::Most likely cause: NaN concurrency exhaustion - the cluster allows 5 simultaneous"
echo "::error::requests, shared with pi, qq and hive embeddings. See #1107."
exit 1