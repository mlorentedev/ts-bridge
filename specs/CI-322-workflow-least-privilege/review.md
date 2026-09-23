---
spec: "CI-322-workflow-least-privilege"
verdict: "FAIL"
reviewed_sha: "00c71b92a904b4eaff0a717705d94afa2222ff05"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-09-22"
---
## Adversarial review

**Scope**: CI-322-workflow-least-privilege
**Sources**: `specs/CI-322-workflow-least-privilege/{proposal,tasks,verification}.md`, git diff `b0b1ec177e6938c6524b92bc6f8e15605b3580c3...HEAD`

### Spec and task alignment
- `check-workflow-permissions.sh` correctly enforces top-level `permissions:` and `persist-credentials: false`.
- The test suite correctly covers the expected happy paths and known refusals across both `bash` and `zsh`.
- AC3 mandates that the guard refuses each widening. However, the guard fails to parse valid alternative YAML spellings, causing both a false positive (false red on common valid formatting) and a false negative (security bypass).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker | REAL | YAML Parsing | The awk parser assumes `uses:` is the first key in a step (and thus dictates the step's indentation). If `uses:` is preceded by another key like `name:` or `if:`, `uses:` shares the same indentation as `with:`, causing `n > pindent` to evaluate to false. The guard prematurely flushes the step and reports a false red (`no-except checkout persists credentials`). This breaks CI on perfectly valid and common GitHub Actions syntax. | Reproduced via local manual test: a step with `- name: Checkout` on line 1 and `uses: actions/checkout@v4` on line 2 fails the check. | UNTESTED | code + tests |
| Major | THEORETICAL | Security Bypass | The `write-all` refusal regex (`:[ \t]*" QO "write-all"`) only matches values on the same line. A malicious actor could bypass the guard using a YAML block scalar (e.g., `permissions: >- \n write-all`), bypassing the guard while still granting ambient write scope. | Reproduced via local manual test: `permissions: >- \n  write-all` returns `OK`. | UNTESTED | code + tests |
| Minor | THEORETICAL | YAML Parsing | YAML booleans are case-insensitive (`False`, `FALSE`), but the script checks strictly for `"false"`. `persist-credentials: False` will be incorrectly reported as `enabled` (a false red). | Code read: `else if (persist != "false")` in `check-workflow-permissions.sh`. | UNTESTED | code + tests |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Criteria met on happy path, but substantial negative-path gaps (false reds on valid YAML, block scalar bypass). |
| Verification       | B | Evidence covers criteria and edge cases discovered, but misses tests for valid step property ordering (`name:` before `uses:`). |
| Scope              | A | Diff matches proposal exactly; no scope creep. |
| Reliability        | C | Brittle awk parser fails on common YAML structures, making the guard unreliable in practice. |
| Maintainability    | B | Clear script structure and exhaustive test suite for the handled cases. |
| Handoff-readiness  | A | Spec updates included, lessons captured in `docs/lessons`, and execution decisions clearly documented. |

### Verdict
FAIL

### Recommended next steps
- **Fix the parser indentation logic:** Update `check-workflow-permissions.sh` to track indentation of the step block (e.g., the `-`) rather than relying on the `uses:` key to dictate the `pindent`. Add a regression test fixture (`checkout-not-first`) to `test-workflow-permissions.sh` where `name:` precedes `uses:`.
- **Close the block scalar bypass:** Update the script state machine or regex to detect `write-all` on subsequent lines if block scalars (`>` or `>-`) are used. Add a regression test fixture (`block-scalar-write-all`).
- **Make boolean check case-insensitive:** Convert the `persist` variable to lowercase before checking `!= "false"` to prevent false reds on `False` or `FALSE`. Add a regression test fixture for case insensitivity.
