---
id: "CI-345-hygiene-guard-fixes"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-09-28"
issue: "mlorentedev/ts-bridge#345"
tags: [spec, ci, github-actions, tech-debt]
template_version: "1.0"
---

# CI-345: null-body guard crash and no workflow/shell lint

## Why

Two code-level Minors from the CI-314 adversarial review (`specs/archive/CI-314-pr-agent-reviewer/review.md`,
2026-09-21), kept out of #341 on purpose: #341 changed spec text only, and a change to the guard the
spec verifies needs its own review.

1. `pr-agent.yml`'s "Fail if no review was published" step ran `(.body | contains($marker))` in
   `jq`. A `gh api` comment with `body: null` (GitHub returns this for some edited/reaction-only
   comments) makes `jq` abort the whole filter with exit 5, so a red job that should show its own
   diagnostic instead surfaces a bare jq stack trace. It fails closed today (never a false green),
   but a reader gets the wrong remedy.
2. Nothing lints the workflows themselves or their embedded shell. `actionlint`/`shellcheck` are
   named only in a comment in `pr-agent.yml`. Running actionlint+shellcheck locally against the
   unmodified tree found one pre-existing issue: `release.yml`'s `sha256sum *` (SC2035) — a
   filename beginning with `-` would be parsed as an option.

## What

- `pr-agent.yml`'s guard tolerates a null `body` (`.body // ""`) instead of aborting.
- The guard's shell logic is extracted to `scripts/check-review-published.sh` so it is testable
  without a live PR-Agent run; a new fixture-driven suite
  (`scripts/tests/test-check-review-published.sh`) replays the null-body scenario plus three
  control cases (`only-null-body`, `clean-match`, `no-match`) by stubbing `gh` on `PATH`, needing
  no network. This requires the `review` job to check out the repo (previously it did not); the
  checkout is read-only (`persist-credentials: false`, per CI-322).
- `repo-hygiene.yml` installs `actionlint` (`go install`, pinned `v1.7.12`) and runs it with
  `-shellcheck=shellcheck` (preinstalled on `ubuntu-latest`) against every workflow, wired into the
  job whose `hygiene` check is required on `master`.
- `release.yml`'s `sha256sum *` becomes `sha256sum -- *` (SC2035), the one pre-existing finding
  that would otherwise start the new lint step red.

## Out of scope

- Extending the guard/lint pass to `.github/actions/*/action.yml` composite manifests (none exist
  in this repo today).
- Any other CI-314 review finding already dispositioned elsewhere (#341, #348, #351, #333).
- Any Go production code or CLI surface change.

## Risks / open questions

- **Adding a checkout to `pr-agent.yml`'s `review` job.** The job previously ran with no checkout
  at all (deliberate, per its own comments). Read-only (`persist-credentials: false`), and pinned
  to `ref: ${{ github.event.repository.default_branch }}` rather than the PR head/merge ref
  (CodeRabbit finding on PR #359, Security Architecture review, High): the job carries
  `pull-requests: write` / `issues: write`, so a same-repo PR that could edit the checked-out
  script would otherwise decide, with that write-capable token, whether its own review
  requirement was satisfied. Reading the guard from the base ref closes that without losing
  coverage — the guard is generic marker-matching logic, never specific to the PR under review,
  and the same trust reasoning already governs `BASE_REF` (the registry lookup) elsewhere in this
  workflow. See `verification.md` for the full disposition.
- **`go install`-based actionlint has no SHA pin the way `uses:` actions do.** Pinned to an exact
  version tag (`v1.7.12`) instead, consistent with `golangci-lint`'s and `gosec`'s existing
  `go install ...@version` / `@latest` pattern in `ci.yml`.

## Acceptance criteria

- [ ] The guard's `jq` tolerates `body: null`, backed by a test that replays the reviewer's null-body scenario.
- [ ] `repo-hygiene.yml` runs `actionlint` (with shellcheck) on every PR, and it is green on master.
- [ ] No regression: `go build ./...`, `go vet ./...`, and the pre-existing hygiene guards stay green.

## References

- Issue #345 (see `issue:` frontmatter).
- Refs #341, #314 (per the issue body).
- Existing precedent: `scripts/check-workflow-permissions.sh` + `scripts/tests/test-workflow-permissions.sh` (extracted-guard-plus-fixture-suite house style), `specs/archive/CI-322-workflow-least-privilege/` (checkout hardening pattern this PR reuses).