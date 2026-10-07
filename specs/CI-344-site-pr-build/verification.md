---
tags: [spec, verification, templates]
created: "2026-09-29"
---

# Verification - CI-344-site-pr-build

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] Criterion 1 -> `scripts/tests/test-site-pr-workflow.sh` failed when the workflow was absent, then passed after `.github/workflows/site-pr.yml` was added with both path filters.
- [x] Criterion 2 -> the contract test plus `check-actions-pinned.sh`, `check-workflow-permissions.sh`, and actionlint verify the pinned, credential-less Node 22 build job.
- [x] Criterion 3 -> the contract test rejects Pages/OIDC write permissions and upload/deploy actions; `.github/workflows/pages.yml` is unchanged.

## Test status

- Contract RED: `bash scripts/tests/test-site-pr-workflow.sh` -> exit 1, workflow absent.
- Contract GREEN: `bash scripts/tests/test-site-pr-workflow.sh` -> `test-site-pr-workflow: OK`.
- Workflow guards: `check-actions-pinned.sh` -> 9 workflows OK; `check-workflow-permissions.sh` -> 12/12 checkouts credential-less.
- Lint: `actionlint -shellcheck=shellcheck` -> exit 0.
- Site build: `npm ci --ignore-scripts && npm run build` -> 5 pages, Pagefind and sitemap generated; existing i18n/404 warnings only.
- Live PR proof: #381 added `site-build`; the valid workflow revision passed.
- Negative CI proof: commit `c6f548a` added an unclosed MDX element and `site-build` failed as required ([run 36612632616, job 109557629253](https://github.com/mlorentedev/ts-bridge/actions/runs/36612632616/job/109557629253)).
- Recovery proof: commit `c270a38` removed only the deliberate MDX break and the same local production build returned to green.
- CodeRabbit review: applied the deploy-action matcher fix and added a self-test for both `actions/upload-pages-artifact@…` and `actions/deploy-pages@…`.
- Two-shell permission fixtures: bash cases passed; local Windows lacks zsh, which `repo-hygiene.yml` installs before running the suite in CI.
- No regressions in existing test suite: yes for the directly affected workflow guards.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- A separate read-only PR workflow is safer than adding `pull_request` to `pages.yml`, whose Pages and OIDC write permissions are required only for deployment.
- The workflow includes its own path so the introducing PR proves the check runs instead of waiting for the first later site edit.

## Review round 1 (`nan/deepseek-v4-flash`, FAIL)

`review.md` records one REAL Major and seven Minor/Question rows. The Major is fixed in
`scripts/tests/test-site-pr-workflow.sh`; no contract file (`proposal.md`, `tasks.md`,
`features.json`) changed, so the next round reviews the same contract digests.

| Finding | Disposition | Evidence |
|---|---|---|
| Major: the guard asserts on file text, so a renamed job, `npm ci` kept only in a comment, and a checkout moved to another job all pass | **Applied.** The guard strips comments, cuts out the `site-build` job, and asserts per step (`run: npm ci` with `working-directory: site` in the same step; checkout with `persist-credentials: false` in the same step). Fixture suite under `scripts/tests/fixtures/site-pr/` in the style of `test-workflow-permissions.sh`. | RED: against the old guard, `renamed-job`, `npm-ci-in-comment` and `checkout-other-job` exited 0. GREEN: all 9 fixtures behave as specified under bash and zsh. Six mutants of the real `site-pr.yml` (job renamed, `npm ci` in a comment, build command changed, Node 20, pin comment dropped, `contents: write`) all exit 1. |
| Minor: the guard pins the quoting of `node-version` | **Applied.** Quote-tolerant match; `node-unquoted` fixture must pass. | `node-unquoted` exited 1 against the old guard, 0 now. |
| Minor: a comment naming a forbidden capability would red the guard | **Applied** (found while writing fixtures, same root cause as the Major). | `comment-mentions-forbidden` exited 1 against the old guard, 0 now. |
| Minor: the recorded local proof (`npm ci --ignore-scripts`, Node 24) does not reproduce the gate | **Declined, with the gate as evidence.** The gate is the `site-build` job itself on Node 22 without `--ignore-scripts`; its green run on PR #381 is the reproduction. The local line stays as what it is: a smoke check, labelled as such here. | PR #381 `site-build` job |
| Minor: `features.json` entries stay `pending` with empty `evidence` | **Declined for this spec.** Only the harness may set `passing` (tasks.md, "Pass-state gating"), and the review itself notes no `dotf spec archive` pre-flight executes them; that is harness scope, not this workflow. | `tasks.md` gating rule |
| Minor (theoretical): no `timeout-minutes` on `site-build` | **Applied.** `timeout-minutes: 15`, in line with `pr-agent.yml`. | `actionlint .github/workflows/site-pr.yml` clean |
| Minor (theoretical): `npm ci` runs lifecycle scripts on PR-controlled lockfiles | **Declined.** Residual only (no secrets, `contents: read`, no `id-token`, credential-less checkout), and `pages.yml` shares the posture: switching one arm alone creates the divergence the review warns about. Both arms change together or neither. | review.md row |
| Question: `site-build` is not a required status | **Informational.** Required contexts are an owner-side branch-protection setting, already tracked as hand-set state in #351. What the merge gate enforces is this guard, via the required `hygiene` job. | #351 |
| Question: launcher review base spans other specs | **Informational, harness scope.** Not a defect of this change. | — |

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-036-2026-10-06.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: this is a CI verification boundary, not a product architecture decision.
- [x] New pattern candidate for `00_meta/patterns/`? no: PR-time documentation builds are established CI practice, not a new cross-project pattern.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-344-site-pr-build/` -> `specs/archive/CI-344-site-pr-build/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
