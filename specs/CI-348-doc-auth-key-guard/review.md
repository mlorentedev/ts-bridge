---
spec: "CI-348-doc-auth-key-guard"
verdict: "FAIL"
reviewed_sha: "7603f59cd81f6e88f3c2f22bd07af7f6fb567a26"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-10-06"
---

## Adversarial review

**Scope**: CI-348-doc-auth-key-guard
**Sources**: `specs/CI-348-doc-auth-key-guard/` and diff against `0b23df6e9b8e1298b66f84d3c21d4116b420770d...HEAD`

### Spec and task alignment
- The implementation diverges from AC1 by failing to ignore `--auth-key <value>` in nested prose lists.
- The redesign documented in the threat model (scanning by region, not by command) causes `check-doc-authkey.sh` to correctly ignore `ts-bridge` invocation matching, but conflicts with the `proposal.md` "What" section which states the guard fails when a "ts-bridge command" passes a key.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker  | REAL    | parser | AC1 Violation: Nested lists in prose are falsely classified as indented code blocks (4 spaces). If a list item mentions `--auth-key <value>`, the guard fails it as a forbidden command. | `* Security:\n    * --auth-key <value>` returns exit 1. | UNTESTED | code + tests |
| Major    | THEORETICAL | parser | Env var assignments and subshells containing the word `init` trick the guard into applying the `init` exception to `connect` commands, bypassing the check. | `VAR=" init " ts-bridge connect --auth-key secret` returns exit 0. | UNTESTED | code + tests |
| Minor    | THEORETICAL | parser | Fenced block parser limits markers to 3 characters (`substr(..., 3)`). A 4-backtick ` ```` ` block containing a 3-backtick ` ``` ` block gets prematurely closed, treating commands as prose and bypassing the guard. | ` ```` ` wrapping ` ```bash ` returns exit 0. | UNTESTED | code |
| Minor    | THEORETICAL | spec | Spec vs code mismatch: `proposal.md` "What" section claims the guard fails when a "`ts-bridge` command" passes a key. The implementation (and the added Threat Model) intentionally checks every command in a code block without `ts-bridge` recognition. | `other-tool --auth-key secret` is flagged. | UNTESTED | spec (`proposal.md` "What" section) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Criteria partially met; substantial negative-path gaps around nested lists and `init` parsing. |
| Verification       | A | Verification artifacts provide reproducible commands and robust test cases. |
| Scope              | A | Diff matches proposal closely; redesign is well documented in the threat model. |
| Reliability        | B | Most error paths are handled appropriately; regex-based parsing has edge cases. |
| Maintainability    | B | Script is clean and maintainable, though regex complexity is increasing. |
| Handoff-readiness  | A | Spec updates and threat model additions clearly capture the new design context. |

### Verdict
FAIL

### Recommended next steps
- Fix the indented code block parser in `scripts/check-doc-authkey.sh` to distinguish between a 4-space markdown indented code block and a nested list item in prose (e.g., exclude lines starting with list markers).
- Constrain the `init` exception regex so it only matches the actual command token, not strings within environment variable assignments or subshells.
- Update the `proposal.md` "What" section to align with the Threat Model (clarifying that it checks all commands in code regions, not just `ts-bridge`).
- Add tests for the nested list prose, `init` token parsing, and 4-backtick fenced blocks.
