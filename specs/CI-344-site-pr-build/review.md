---
spec: "CI-344-site-pr-build"
verdict: "FAIL"
reviewed_sha: "de7a911db0f586e957c757cea756f7624484974d"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-10-06"
---

## Adversarial review

**Scope**: CI-344-site-pr-build
**Sources**: specs/CI-344-site-pr-build/*, .github/workflows/site-pr.yml, scripts/tests/test-site-pr-workflow.sh, .github/workflows/repo-hygiene.yml

### Spec and task alignment
- Acceptance criteria require a read-only `site-build` job that checks out the proposed revision and uses `npm ci` and `npm run build`.
- The implemented tests aim to enforce this contract structurally using `test-site-pr-workflow.sh`.
- However, the structural checks are fragile and can be easily bypassed by a malicious pull request, failing to guarantee the acceptance criteria.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker  | REAL    | validation | `step_has` grep-based assertion is bypassed if required commands are nested under `env:` or `with:`. A dummy action can declare `run: npm ci` in its `env` block and pass the guard without executing a build. | `test-site-pr-workflow.sh` uses line-by-line regex. Placing `run: npm ci` under `env` bypasses it. | UNTESTED | code + tests |
| Blocker  | REAL    | validation | The guard does not forbid `if:` conditions. A malicious PR can disable the site build entirely by appending `if: false` to the `npm ci` step, while still passing the contract test. | `step_has` does not assert the absence of `if:` conditions. | UNTESTED | code + tests |
| Blocker  | REAL    | validation | Path filters assert inclusion but allow exclusion. A malicious PR can add `- '!site/**'` to `paths`, entirely disabling the PR build for site changes. | The script checks `grep -qxE -- '- site/\*\*'` but ignores other lines in the `paths` block. | UNTESTED | code + tests |
| Major    | REAL    | reliability | Windows CRLF breaks the test suite. `yaml_block` preserves `\r`, causing the path-filter assertion `[ "$paths" = ... ]` and `grep -x` to fail on valid workflows locally. | `sed -i 's/$/\r/' .github/workflows/site-pr.yml && bash scripts/tests/test-site-pr-workflow.sh` fails. | UNTESTED | code |
| Major    | THEORETICAL | validation | `check-workflow-permissions.sh` (the repo's pre-existing guard) suffers from the same `env:` bypass. Placing `persist-credentials: false` under an `env:` block in the checkout step satisfies the guard but persists credentials. | Demonstrated locally with `bash scripts/check-workflow-permissions.sh`. | UNTESTED | code |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | D | Security gate does not actually prevent bypasses, failing its primary purpose. |
| Verification       | B | Test script and fixtures exist but lack negative tests for `env`, `if: false`, and path exclusions. |
| Scope              | A | Implementation is strictly within the boundaries of the proposal. |
| Reliability        | B | Handled edge cases except for CRLF line endings on Windows. |
| Maintainability    | C | Custom awk/grep parsing is fragile and prone to bypasses compared to proper YAML parsing. |
| Handoff-readiness  | A | Spec is complete and verification steps are logged. |

### Verdict
FAIL

### Recommended next steps
- Update `test-site-pr-workflow.sh` to forbid `if:` in the `site-build` job steps.
- Update `test-site-pr-workflow.sh` to assert that `run:` is not nested under `env:` or `with:`, perhaps by using a strict YAML parser (`yq`) or by improving the awk script to track YAML structure depth within steps.
- Update `test-site-pr-workflow.sh` to forbid `!` exclusions in the path filters.
- Strip `\r` carriage returns from the parsed text in `test-site-pr-workflow.sh` to fix Windows local execution.
- Fix the same `env:` bypass in `scripts/check-workflow-permissions.sh`.
