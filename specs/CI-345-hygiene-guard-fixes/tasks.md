---
tags: [spec, tasks, ci, github-actions]
created: "2026-09-28"
---

# Tasks - CI-345-hygiene-guard-fixes

> TDD order. One task = one focused commit. Tick as you go.

## Setup

- [x] Branch created from master: `ci/345-hygiene-guard-fixes`
- [x] `proposal.md` complete and every acceptance criteria testable
- [x] Issue #345 open and picked up (self-assigned via this session)

## Implementation

- [x] [AC1] **RED first**: reproduced the null-body crash locally with a `jq` one-liner against a
      fixture comment (`{"body": null, ...}`) piped through `jq -s ... (.body | contains($marker))`
      -> `jq: error (at <stdin>:0): null (null) has no keys` equivalent abort; confirmed the
      unmodified filter cannot survive a null body
- [x] [AC1] Extract the "Fail if no review was published" step's shell logic to
      `scripts/check-review-published.sh`, fixing `(.body | contains($marker))` ->
      `((.body // "") | contains($marker))` in the process
- [x] [AC1] Add checkout (`persist-credentials: false`) to `pr-agent.yml`'s `review` job so the
      extracted script is available on disk; update the step to `run: bash scripts/check-review-published.sh`
- [x] [AC1] Write `scripts/tests/test-check-review-published.sh`: stubs `gh` on `PATH`, 4 fixtures
      (`null-body`, `only-null-body`, `clean-match`, `no-match`) — `null-body` is the regression
      case (must still find the marker on the second comment), `only-null-body` is the same shape
      with no non-null match (must fail closed, not crash)
- [x] [AC2] **RED first**: ran `actionlint -shellcheck=shellcheck` against the unmodified tree,
      found one finding — `release.yml:` `sha256sum *` (SC2035)
- [x] [AC2] Fix `release.yml`: `sha256sum *` -> `sha256sum -- *`
- [x] [AC2] Add `actionlint` (go install, pinned `v1.7.12`) + shellcheck step to
      `repo-hygiene.yml`, and wire the new `test-check-review-published.sh` suite into the same job
- [x] Refactor for clarity: header comment in `repo-hygiene.yml` updated to describe the added
      guard/lint steps rather than the stale "three guards" count

## Closing

- [x] Every acceptance criterion covered by at least one test or an executed command (mapping in `verification.md`)
- [x] Type checks pass — `go build ./...`, `go vet ./...` (no Go touched; run anyway per CI-322 precedent)
- [x] `actionlint -shellcheck=shellcheck` clean on every workflow in the repo
- [x] All hygiene guards green — `check-lessons.sh`, `check-actions-pinned.sh`,
      `check-workflow-permissions.sh`, `test-workflow-permissions.sh` (bash; zsh unavailable on the
      dev machine, confirmed CI installs it), `test-check-review-published.sh`
- [x] No unrelated changes in the diff
- [x] `verification.md` filled with executed evidence, not intentions
- [x] PR opened referencing this spec folder (Closes #345): #359
- [x] Post-review fix (CodeRabbit on #359, Security Architecture, High): the added checkout
      read the guard script from the PR head, so a same-repo PR editing
      scripts/check-review-published.sh could report a false "review published" using the
      guard step's write-capable token. Pinned the checkout to the repository's default branch
      so the guard always runs the trusted master copy; re-verified actionlint -shellcheck=shellcheck
      clean. Full disposition table in verification.md
- [x] Reviewer output dispositioned and recorded (verification.md -> "Review window (PR #359)")
- [ ] Independent adversarial review before archive (implementer cannot sign it)
