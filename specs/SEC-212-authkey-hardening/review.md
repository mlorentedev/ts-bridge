---
spec: "SEC-212-authkey-hardening"
verdict: "FAIL"
reviewed_sha: "3e71a7ec972352774c3c5806a5e8303fc24a5c6e"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-09-21"
---
## Adversarial review

**Scope**: SEC-212-authkey-hardening
**Sources**: specs/SEC-212-authkey-hardening/{proposal,tasks,verification}.md, git diff 64d8a7774368dc90b1b6aae630309fa9b438bfa0...HEAD

### Spec and task alignment
- `README.md`, `.env.example`, `docs/runbooks/guide-multi-device-operations.md`, `docs/runbooks/guide-deployment-windows.md`, and `docs/troubleshooting/security-audit.md` were updated to recommend `--auth-key-file`.
- `cmd/cli/init.go` and `cmd/cli/init_test.go` promote `--auth-key-file` and document process list and environment inheritance risks.
- AC3 mandates updating runbooks under `docs/` to use `--auth-key-file` in CLI examples. However, `docs/runbooks/guide-deployment-linux.md` was intentionally skipped and AC3 was marked as partially delivered in `tasks.md`.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | Spec vs Code | AC3 mandates updating runbooks to use `--auth-key-file`, but `docs/runbooks/guide-deployment-linux.md` was intentionally skipped and not updated. | `tasks.md` says "Not delivered: docs/runbooks/guide-deployment-linux.md" | UNTESTED | spec |
| Minor | REAL | Tests | `TestInit_SecurityGuidance` does not verify the process list warning added to the `--auth-key` flag description, only checking the child process warning in `cmd.Long`. | `cmd/cli/init_test.go` | UNTESTED | tests |
| Minor | REAL | Docs | In `docs/runbooks/guide-multi-device-operations.md`, `Get-Content` is used with `--auth-key`, which evaluates the file into a command line argument and leaks it to the process table. Though acknowledged as a risk in the doc, it remains in the quick setup command. | `docs/runbooks/guide-multi-device-operations.md` | UNTESTED | spec |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Criteria partially met; AC3 is incomplete as the Linux runbook update was skipped. |
| Verification       | B | Evidence covers criteria but missing negative tests for flag help text. |
| Scope              | B | Diff mostly matches the proposal with well-documented side-changes. |
| Reliability        | B | Security warnings correctly documented for all edge cases (environment/process list exposure). |
| Maintainability    | B | Code and documentation changes are clear and well-structured. |
| Handoff-readiness  | B | Spec updates included and skipped portions are properly tracked in new issues (#307). |

### Verdict
FAIL

### Recommended next steps
- Update `proposal.md` AC3 to explicitly exclude `docs/runbooks/guide-deployment-linux.md` so the contract matches the delivered implementation, or implement the change in the Linux runbook.
- Add an assertion to `TestInit_SecurityGuidance` (or create a new test) to verify that the `--auth-key` flag description contains the warning about process list exposure.
