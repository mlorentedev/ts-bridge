---
tags: [spec, verification, templates]
created: "2026-10-31"
---

# Verification - CI-514-hygiene-wiring-guard

## Evidence

- [x] Criterion 1 -> `scripts/tests/test-hygiene-wiring.sh` fixtures `guard-coe`, `suite-coe`,
      `job-coe`, `other-step-coe-expression` (`${{ ... }}` value) and `guard-coe-false`, all exit 1
      with `forbidden: continue-on-error`.
- [x] Criterion 2 -> fixtures `guard-if`, `suite-if`, `job-if`, `job-needs`, `guard-missing`,
      `suite-missing`, `guard-under-env` (`run:` nested under `env:`) and `guard-in-comment`, all
      exit 1 with the reason named.
- [x] Criterion 3 -> `valid` (a copy of the real workflow, including the AI-trailer `if:`),
      `comment-mentions-coe` and `crlf-valid` exit 0; `crlf-guard-coe` exits 1. The real
      `.github/workflows/repo-hygiene.yml` is accepted.
- [x] Criterion 4 -> negative probes: a copy of the real workflow with `continue-on-error: true`
      under `run: bash scripts/check-conflict-markers.sh` -> exit 1; under
      `run: bash scripts/tests/test-conflict-markers.sh` -> exit 1 (`features.json` f4 runs both).
- [x] CI wiring -> `.github/workflows/repo-hygiene.yml` runs `bash scripts/tests/test-hygiene-wiring.sh`
      as its own step (the f5 grep passes); `actionlint -shellcheck=shellcheck` exits 0, the check
      accepts the edited workflow, and the f4 probes still exit non-zero on it.

## Test status

- RED: with a stub check that only echoes `OK`, every red fixture exits 0 and the suite fails.
- GREEN: `bash scripts/tests/test-hygiene-wiring.sh` -> `OK (17 fixtures + .github/workflows/repo-hygiene.yml)`; 18 after the CodeRabbit fix below.
- Mutants of the check, each reddening the suite: the `continue-on-error` rule removed; the
  guarded `run:` matched as a substring instead of an exact step key; the step `if:` rule
  removed; the job `if:` rule removed. One equivalent mutant: dropping the CR strip changes
  nothing, because the shared awk readers already treat `\r` as trailing whitespace; the strip
  stays as a defence, and `crlf-valid` pins the accepting side.
- Refactor: `test-site-pr-workflow.sh` sources the extracted helper -> `OK (26 fixtures + .github/workflows/site-pr.yml)`.
- Every hygiene guard on the real tree: `check-lessons` OK (48), `check-conflict-markers` OK (565),
  `check-actions-pinned` OK (9), `check-workflow-permissions` OK, `check-doc-authkey` OK (91).
- `shellcheck -x` on the new test, the refactored sibling and the helper -> exit 0.
- No Go code changed.

## Decisions made during implementation

- A separate step instead of folding the check into `test-conflict-markers.sh`: folded, one
  `continue-on-error` on that step would disable the suite and the wiring check together.
- `continue-on-error` is forbidden file-wide, by key: the workflow has none today, and a
  value-based rule would have to enumerate `true`, expressions and quoted spellings.
- The `if:` rule covers only the guarded steps and the job: the AI-trailer step needs its `if:`.
- `strip_comments` and `job_steps` moved to `scripts/tests/helpers/workflow_yaml.bash` rather
  than being copied, so both workflow guards share one reader.

## Review dispositions (`review.md`, PASS, nan/deepseek-v4-flash at `c7e0ae0`)

1. **Spaced or quoted `continue-on-error` key passes (Minor, theoretical). Declined.** These
   spellings take a deliberate edit, and the proposal's threat model is a regression lint for
   workflows maintainers write, not a defence against evasion (CI-348). Matching every YAML key
   spelling is the enumeration that lesson 036 warns against.
2. **`valid.yml` drifted from the real workflow (Minor, real). Applied.** `valid.yml` was
   re-copied from `repo-hygiene.yml` after the new step landed, and the suite stays green.
   Correctness never depended on it: the real file is checked separately at the end of the
   suite.
3. **Nothing asserts that the check's own step stays wired (Minor, theoretical). Accepted.**
   Every guard in this repo has the same self-reference, and the proposal records disabling
   this step as the accepted residual risk.
4. **Bash-only, while the sibling suite runs under two shells (Minor, speculative). Declined.**
   The reviewer ran it under zsh and it exits 0. The check reads a YAML file on the CI runner
   and does not test shell portability, which is why the proposal declares it bash-only, like
   `test-site-pr-workflow.sh`.
5. **Proposal omits the helper extraction, and the AC boxes are unticked (Minor, real).
   Recorded here, with no contract edit.** `strip_comments` and `job_steps` moved into
   `scripts/tests/helpers/workflow_yaml.bash`, and `test-site-pr-workflow.sh` sources it (26
   fixtures, still green). Tasks and the PR body record this. All four acceptance criteria are
   met, according to the Evidence section above and to the review's own table.
6. **Is `hygiene` a required check? (Question).** Yes. The live branch-protection read of
   2026-10-30 lists `test`, `lint`, `security` and `hygiene` as required on `master` (#507,
   `docs/troubleshooting/security-audit.md`). Protection as code is tracked in #351.

## CodeRabbit dispositions (review at `226b457`)

- **`job_steps` takes the first list in the job as the step list. Applied.** A
  `strategy.matrix.include` placed before `steps:` turned the matrix items into "steps", and the
  real steps read as job-level keys. The `valid` workflow with a matrix added then failed with
  `forbidden: if condition on job 'hygiene'`, because the AI-trailer step's `if:` was read as the
  job's. Now only list items under the job's own `steps:` key start a step. The new fixture
  `matrix-before-steps` was red first and is now green, the suite reports
  `OK (18 fixtures + …)`, the sibling `test-site-pr-workflow.sh` is still `OK (26 fixtures + …)`,
  and `shellcheck -x` exits 0. The contract set is untouched, so the PASS above stays fresh.
- **Quoted or spaced keys (`"if":`, `continue-on-error :`). Declined.** This is the same as
  review finding 1 above.
- **f4 passes vacuously if awk inserts no probe line. Declined.** That happens only when a
  guarded `run:` line is gone, and then the real-workflow run of the check, in this suite and
  in CI, already fails with `missing step`. Fixing it means editing `features.json`, which
  invalidates the PASS for a case that is already red.
- **f5's grep is not scoped to the `hygiene` job. Declined.** `repo-hygiene.yml` has one job,
  and the same contract cost applies.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the one durable point (a check folded into the step it guards is disabled by the same edit) is recorded in the test header and proposal; lesson 036 already covers structure over spelling
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: a test following the existing workflow-guard pattern
- [x] New pattern candidate for `00_meta/patterns/`? no: one project so far

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/CI-514-hygiene-wiring-guard/` -> `specs/archive/CI-514-hygiene-wiring-guard/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
