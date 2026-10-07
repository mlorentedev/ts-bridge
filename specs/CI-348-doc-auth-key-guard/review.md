---
spec: "CI-348-doc-auth-key-guard"
verdict: "FAIL"
reviewed_sha: "c5cceef8e023246b3bd6da40b3ac8149e3ff257c"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-10-06"
---
## Adversarial review

**Scope**: CI-348-doc-auth-key-guard
**Sources**: `specs/CI-348-doc-auth-key-guard/{proposal,tasks,verification}.md`, PR diff `0b23df6e9b8e1298b66f84d3c21d4116b420770d...HEAD`

### Spec and task alignment
- `check-doc-authkey.sh` was introduced and wired into the GitHub Actions workflow successfully.
- Tests exercise multiple shell syntaxes, but miss edge cases where commands are split at the binary name or executed via wrappers.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker  | REAL    | regex | `is_command` expects a trailing space after `ts-bridge`, so a line split immediately after the command (`ts-bridge \`) drops the space and bypasses the guard entirely. | `ts-bridge \ \n connect --auth-key tskey...` returns `OK`. | UNTESTED | code + tests |
| Major    | THEORETICAL | wrapper | `strip_prefixes` fails to strip environment variables whose values contain spaces (e.g., `VAR="a b" ts-bridge`) because its regex `[^[:space:]]*` stops at the first space. | `VAR="value with spaces" ts-bridge ...` returns `OK`. | UNTESTED | code + tests |
| Major    | THEORETICAL | wrapper | Execution via runners like `docker run` or `go run` bypasses `is_command` because the regex anchors `ts-bridge` to the start of the segment. A future doc using a runner will silently bypass the guard. | `docker run mlorentedev/ts-bridge connect ...` returns `OK`. | UNTESTED | code + tests |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | D | `ts-bridge \` parsing defect completely bypasses the guard for a common multi-line pattern. |
| Verification       | B | Evidence covers criteria but misses several edge cases in command prefixes and splits. |
| Scope              | A | Diff matches proposal exactly; no scope creep. |
| Reliability        | B | Awk parser is mostly reliable but brittle around shell edge cases. |
| Maintainability    | B | Clear naming and structure, but `strip_prefixes` regexes are hard to maintain. |
| Handoff-readiness  | A | Spec updates and verification artifacts are present and well-documented. |

### Verdict
FAIL

### Recommended next steps
- Update `is_command` regex to allow `ts-bridge` at the end of the segment: `^([^[:space:]]*[\/\\])?ts-bridge(\.exe)?([[:space:]]|$)`
- Update `strip_prefixes` to properly handle quoted strings in environment variables.
- Update `is_command` or `contains_command` to detect `ts-bridge` when executed via common wrappers like `docker run` or `go run`.
- Add fixtures for `ts-bridge \` (split), `VAR="with spaces"`, and `docker run` to `scripts/tests/fixtures/doc-authkey/` and ensure they fail without the fixes.
- Do not run `/spec archive` until the code is fixed and a re-review passes.
