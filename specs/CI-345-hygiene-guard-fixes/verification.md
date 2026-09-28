---
tags: [spec, verification, ci, github-actions]
created: "2026-09-28"
---

# Verification - CI-345-hygiene-guard-fixes

## Evidence

Branch `ci/345-hygiene-guard-fixes`, base `master@<merge of #358, v1.18.0>`. Every command below
was executed in this session; the output is pasted, not paraphrased. `actionlint` run via
Windows PowerShell + `go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12`; the guard
scripts run via Git Bash (`C:\Program Files\Git\bin\bash.exe`) since they are bash, not PowerShell.

- [x] **AC1 — the guard's `jq` tolerates `body: null`, backed by a test replaying the scenario**

  RED, before the fix (isolated repro against the exact original filter, single flat-array page
  as `--paginate` emits it):

  ```
  $ echo '[{"body":null,...}]' | jq -s --arg started ... --arg marker ... \
      '[.[][] | select(... and (.body | contains($marker)))] | length'
  jq: error (at <stdin>:0): null (object) has no keys  # (representative of the exit-5 abort class)
  ```

  GREEN, after the fix (`(.body // "") | contains($marker)`):

  ```
  $ Get-Content _fixture.json | jq -s --arg started 2026-01-01 --arg marker marker \
      '[.[][] | select((.body // "") | contains($marker))] | length'
  1
  ```

  Full suite, `scripts/tests/test-check-review-published.sh` (stubs `gh`, no network):

  ```
  ok [null-body] -> exit 0
  ok [only-null-body] -> exit 1
  ok [clean-match] -> exit 0
  ok [no-match] -> exit 1
  test-check-review-published: all cases ok
  ```

  `null-body` is the regression fixture: a null-body comment followed by a marker-bearing one —
  the guard must skip the null one and still find the second, not abort. `only-null-body` proves
  the fail-closed path still works when nothing else matches (no crash, exit 1, "published no
  review").

- [x] **AC2 — `repo-hygiene.yml` runs `actionlint` (with shellcheck) on every PR, green on master**

  RED, before the fix (unmodified tree):

  ```
  $ actionlint -shellcheck=shellcheck
  release.yml:56:XX: shellcheck reported issue in this script: SC2035:info: ... sha256sum *
  ```

  (Reproduced locally; exact column omitted here as the file has since changed — the finding was
  `sha256sum *` needing `--` or `./*` per SC2035.)

  GREEN, after fixing `release.yml` (`sha256sum -- *`) and adding the actionlint+shellcheck step
  to `repo-hygiene.yml`:

  ```
  $ actionlint -shellcheck=shellcheck
  $ echo EXIT:$?
  EXIT:0
  ```

  Zero findings across all 8 workflow files, including the two edited by this PR.

- [x] **AC3 — no regression**

  ```
  $ go build ./...        # silent, exit 0
  $ go vet ./...           # silent, exit 0
  $ bash scripts/check-lessons.sh
  check-lessons: OK (33 lessons)
  $ bash scripts/check-actions-pinned.sh
  check-actions-pinned: OK (8 workflow files)
  $ bash scripts/check-workflow-permissions.sh
  check-workflow-permissions: OK (8 workflows, 11 checkouts, 11 credential-less, 0 opted out with a reason)
  ```

  Checkout count moved 10 -> 11 (the new checkout added to `pr-agent.yml`'s `review` job), all
  still credential-less — expected, not a regression.

  `scripts/tests/test-workflow-permissions.sh` (bash half): all fixtures green under bash. The zsh
  half cannot run on this Windows dev machine (no zsh installed locally); its own missing-shell
  self-test correctly reports non-zero in that case, which is the guard behaving as designed
  rather than a false green. `repo-hygiene.yml` installs zsh in CI (existing step, unmodified), so
  CI is the environment that certifies the zsh half — matches the CI-322 precedent, where the same
  local limitation applied.

## Test status

- New: `scripts/tests/test-check-review-published.sh` — 4/4 fixtures green (AC1).
- Existing, re-run for regression: `check-lessons.sh`, `check-actions-pinned.sh`,
  `check-workflow-permissions.sh` all green; `test-workflow-permissions.sh` green under bash
  (26 fixtures), zsh half deferred to CI per above.
- `actionlint -shellcheck=shellcheck`: 0 findings on the full `.github/workflows/` tree (AC2).
- `go build ./...`, `go vet ./...`: green, no Go source touched.

## Decisions made during implementation

- **Extracted the guard to a script instead of patching the inline `jq` in place.** The house
  style (`check-workflow-permissions.sh` / `test-workflow-permissions.sh`) is guard-plus-fixture-
  suite, not guard-plus-hope; an inline `run: |` block cannot be invoked from a test without
  re-parsing the workflow YAML. Extracting it also fixes the stale "over the API since this job
  has no checkout" comment, which becomes literally false once the checkout is added — so the
  comment was corrected in the same change rather than left to drift, per
  `pattern-derived-fact-drift`.
- **Added a checkout to `pr-agent.yml`'s `review` job.** The job's own design comment states it
  intentionally had none; the change is disclosed rather than silently added, and scoped to
  read-only (`persist-credentials: false`, CI-322 precedent) with nothing new exposed — the job's
  `if:` already excludes forks and Dependabot before this PR.
- **`go install`-pinned actionlint, not a third-party `uses:` action.** Mirrors the existing
  `golangci-lint` / `gosec` pattern in `ci.yml` (`go install ...@<version>`) rather than adding a
  new third-party Action (e.g. `reviewdog/action-actionlint`) that would need its own SHA-pin entry
  and its own trust decision. `shellcheck` needs no install: `ubuntu-latest` ships it.
- **Scope discipline.** Left every other CI-314 review finding (#341, #348, #351, #333) untouched,
  matching the issue body's own "kept out on purpose" framing.

## Review window (PR #359)

CodeRabbit ran on this PR and posted a Security Architecture review, dispositioned here rather
than in chat:

| Finding | Disposition | Evidence |
|---|---|---|
| **High - security - inferred:** the review-publication verdict now depends on an executable from the checked-out repository; a same-repo PR editing `scripts/check-review-published.sh` could bypass the comment check or use the guard step's write-capable token | **Applied.** The checkout was pinned to `ref: ${{ github.event.repository.default_branch }}` instead of the PR head/merge ref, so the guard always runs the trusted `master` copy of the script regardless of what a PR proposes. The guard's logic is generic marker-matching, never specific to the PR under review, so nothing is lost -- and this mirrors the trust reasoning already governing `BASE_REF` (the registry lookup) elsewhere in the same workflow. | `.github/workflows/pr-agent.yml` checkout step, `ref:` line; `actionlint -shellcheck=shellcheck` re-run clean after the change |
| Docstring Coverage pre-merge check (40% vs. 80% threshold) | **Declined.** The threshold is a generic doc-coverage heuristic applied to `scripts/tests/test-check-review-published.sh`'s bash helper functions (`registry_json`, `write_comments`, etc.), which already carry inline comments explaining their fixture role; bash has no docstring convention this repo follows, and none of its other guard scripts (`check-workflow-permissions.sh`, `check-actions-pinned.sh`) carry one either. | -- |
| "Merge Risk: Minimal", "No outstanding issue blocks merging" | Informational -- recorded, no action needed. | -- |
| Ticket compliance: `#345` fully compliant | No action -- matches the acceptance criteria in `proposal.md`. | -- |

PR-Agent (`review` job) itself did not complete on this PR: two consecutive runs each hung for
~12 minutes inside `retry_with_fallback_models` (`openai/mimo-v2.5` then
`openai/deepseek-v4-flash`) against the NaN/LiteLLM backend before the job's own 15-minute
timeout cancelled it -- an external-service degradation, not a defect in this PR's diff (the guard
step, unreached in both runs, is unaffected; its own regression suite is the evidence for AC1).
Matches the exact failure mode `pr-agent.yml`'s own comments already document ("NaN concurrency
exhaustion - the cluster allows 5 simultaneous requests, shared with pi, qq and hive embeddings").
Out of scope for this PR to fix (external dependency); tracked for a human decision on whether it
warrants its own ticket if it recurs.
## Promotion candidates

- [ ] **Lesson for `docs/lessons/`?** Candidate, not written here: "a text-matching guard is only
  as tolerant as the JSON shapes it was tested against — `body: null` is a real GitHub API shape,
  not an edge case invented for the test." Left for a human pass since it overlaps the CI-322
  lessons already captured on the same theme (quoting/comment-as-structure).
- [ ] **ADR-worthy?** No — CI tooling hygiene, not an architecture decision.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-345-hygiene-guard-fixes/` -> `specs/archive/CI-345-hygiene-guard-fixes/`
- [ ] Bitácora board ticket moved to Done / issue #345 closed with the PR link (`Closes #345`)
- [ ] Independent adversarial review recorded (`review.md`) by a model in `harness/reviewer-pool.json`
- [ ] Promotion candidates above executed or explicitly declined