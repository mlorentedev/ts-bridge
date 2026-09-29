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

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? no: the workflow contract and regression guard fully encode the finding.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: this is a CI verification boundary, not a product architecture decision.
- [x] New pattern candidate for `00_meta/patterns/`? no: PR-time documentation builds are established CI practice, not a new cross-project pattern.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-344-site-pr-build/` -> `specs/archive/CI-344-site-pr-build/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
