#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
WORKFLOW="$ROOT/.github/workflows/site-pr.yml"
PAGES_ACTION_RE='actions/(upload|deploy)-pages([-@]|$)'

fail() {
  echo "test-site-pr-workflow: $*" >&2
  exit 1
}

require() {
  grep -qE -- "$1" "$WORKFLOW" || fail "missing contract: $2"
}

reject() {
  ! grep -qE -- "$1" "$WORKFLOW" || fail "forbidden PR capability: $2"
}

[ -f "$WORKFLOW" ] || fail "workflow not found at $WORKFLOW"

for action in 'actions/upload-pages-artifact@sha' 'actions/deploy-pages@sha'; do
  grep -qE -- "$PAGES_ACTION_RE" <<<"$action" ||
    fail "Pages action matcher does not recognize $action"
done

require '^[[:space:]]*pull_request:' 'pull_request trigger'
require "^[[:space:]]*- 'site/\\*\\*'" 'site path filter'
require "^[[:space:]]*- '\\.github/workflows/site-pr\\.yml'" 'self path filter'
require '^[[:space:]]*contents:[[:space:]]*read' 'read-only repository permission'
require 'actions/checkout@[0-9a-f]{40}[[:space:]]+# v[0-9]+' 'SHA-pinned checkout'
require 'persist-credentials:[[:space:]]*false' 'credential-less checkout'
require 'actions/setup-node@[0-9a-f]{40}[[:space:]]+# v[0-9]+' 'SHA-pinned setup-node'
require "node-version:[[:space:]]*'22'" 'Node 22'
require 'run:[[:space:]]*npm ci' 'npm ci install'
require 'run:[[:space:]]*npm run build' 'site build'

[ "$(grep -cE 'working-directory:[[:space:]]*site' "$WORKFLOW")" -ge 2 ] ||
  fail "both npm commands must run under site/"

reject 'pages:[[:space:]]*write' 'Pages write permission'
reject 'id-token:[[:space:]]*write' 'OIDC write permission'
reject "$PAGES_ACTION_RE" 'Pages upload or deployment action'

echo "test-site-pr-workflow: OK"
