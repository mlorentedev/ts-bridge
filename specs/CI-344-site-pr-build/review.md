---
spec: "CI-344-site-pr-build"
verdict: "FAIL"
reviewed_sha: "0e4cdac93abf7cb55aca3b924f785d7ab2da3a38"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-10-06"
---

## Adversarial review

**Scope**: CI-344-site-pr-build
**Sources**: `specs/CI-344-site-pr-build/`, `.github/workflows/site-pr.yml`, `scripts/tests/test-site-pr-workflow.sh`

### Spec and task alignment
- **Setup**: PR branch created, proposal and tasks complete.
- **Implementation**: The PR workflow is added and tested with a bash-based contract guard.
- **Security Posture**: The implementation asserts a read-only token, but the regex-based validation in the guard is incomplete and permits bypasses for elevated privileges.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker  | REAL    | authz | The contract guard fails to enforce the "only `contents: read`" constraint (AC2). It asserts the presence of `contents: read` but does not deny elevated permissions (e.g., `contents: write`, `pull-requests: write`). Since `check-workflow-permissions.sh` only requires *a* `permissions:` block and no `write-all`, a PR can grant itself `write` permissions and pass all CI checks. | Fixture with job-level `contents: write` passes the guard test suite | UNTESTED (needs fixture like `elevated-permissions.yml`) | tests |
| Major    | REAL    | authz | The explicit denylist for `pages: write` and `id-token: write` can be bypassed using standard YAML string quoting (e.g., `"pages": write` or `pages: 'write'`). The literal `grep` checks miss these, allowing a PR to grant itself OIDC/Pages capabilities, violating AC3. | Fixture with `"id-token": write` bypasses the `grep` check and exits 0 | UNTESTED (needs fixture like `quoted-permissions.yml`) | tests |
| Major    | THEORETICAL | trigger | The guard checks for the path filters `- 'site/**'` and `- '.github/workflows/site-pr.yml'` anywhere in the file, not strictly under `on.pull_request.paths`. A PR author could move these filters to a dummy `env` array to bypass the PR trigger entirely while still passing the contract test. | Moving paths to `env:` passes the guard | UNTESTED (needs fixture for misplaced paths) | tests |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | D | AC2 and AC3 are not securely met due to regex bypasses allowing permission escalation |
| Verification       | B | Test fixtures exist but the bash-based structural assertions have logical gaps |
| Scope              | A | Implementation is focused entirely on the site PR build workflow |
| Reliability        | B | CI job handles caching and builds reliably, but guard script relies on brittle regex |
| Maintainability    | B | Clear fixture-driven test design, though reaching the limits of what grep/awk can parse |
| Handoff-readiness  | A | Spec is fully updated, verification records dispositions from previous rounds |

### Verdict
FAIL

### Recommended next steps
- **tests**: Update `scripts/tests/test-site-pr-workflow.sh` to forbid any occurrence of `write` (e.g., `! grep -qiE 'write'` outside benign contexts like comments) or enforce strict structural bounds for the `permissions:` block.
- **tests**: Update the regex for forbidden capabilities to allow optional quotes: `['"]?(pages|id-token)['"]?:[[:space:]]*['"]?write['"]?`.
- **tests**: Tighten the path filter assertions to ensure they appear under the `on:` block, rather than using a global `grep` across the whole file.
- **tests**: Add named fixtures for these bypass vectors (`elevated-permissions.yml`, `quoted-permissions.yml`) and ensure they fail.
