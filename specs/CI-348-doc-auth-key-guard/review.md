---
spec: "CI-348-doc-auth-key-guard"
verdict: "FAIL"
reviewed_sha: "fe7f821646a17c20291a2bb44ccceca96ddb0d9e"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-10-06"
---

## Adversarial review

**Scope**: CI-348-doc-auth-key-guard
**Sources**: `specs/CI-348-doc-auth-key-guard/`, `git diff 0b23df6e9b8e1298b66f84d3c21d4116b420770d...HEAD`

### Spec and task alignment
- The guard successfully implements the core requirements: scanning `README.md` and `docs/`, flagging inline `--auth-key` usages, and providing an exception for warned `init` commands.
- The `repo-hygiene.yml` integration ensures it runs automatically.
- However, the `awk` regex-based command parser misses several common shell syntax patterns, allowing significant bypasses where an inline key would go unnoticed.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker  | REAL    | parser | Quoted executable paths and names (`"C:\Program Files\ts-bridge.exe"`, `"ts-bridge"`) bypass `is_command`. Spaces break `[^[:space:]]*`, and the closing quote breaks the `([[:space:]]\|$)` suffix check. | Manually verified: `"ts-bridge" connect --auth-key X` and `& "C:\Program Files\ts-bridge\ts-bridge.exe" connect --auth-key X` return exit 0 | UNTESTED | code + tests |
| Blocker  | REAL    | parser | Root shell prompts (`# `) and alternative prompts (`% `) are not stripped by `normalize` (only `$ ` and `> ` are). This bypasses `is_command` completely, and also silently ignores commented-out examples. | Manually verified: `# ts-bridge connect --auth-key X` returns exit 0 | UNTESTED | code + tests |
| Major    | REAL    | parser | Subshells `(...)` and brace groups `{ ... }` bypass the guard because `split` on `;`, `&&`, `\|\|`, `\|` leaves the opening bracket attached to the command (e.g., `(ts-bridge`), failing `is_command`. | Manually verified: `(ts-bridge connect --auth-key X)` returns exit 0 | UNTESTED | code + tests |
| Major    | THEORETICAL | parser | `is_command` is case-sensitive, so Windows documentation using `.EXE` (e.g., `ts-bridge.EXE`) bypasses the guard. | Manually verified: `ts-bridge.EXE connect --auth-key X` returns exit 0 | UNTESTED | code + tests |
| Minor    | THEORETICAL | parser | False positive: `ts-bridge --verbose init ...` with a nearby warning is flagged as forbidden because `scan_command` rigidly expects `init` to immediately follow the binary name. | Manually verified: `ts-bridge --verbose init --auth-key X` fails despite nearby warning | UNTESTED | code + tests |
| Minor    | REAL    | parser | Command substitutions with spaces in env-var assignments (`FOO=$(echo val) ts-bridge`) bypass `strip_prefixes` because `[^[:space:]"\047]*` halts at the space, corrupting the segment. | Manually verified: `FOO=$(echo val) ts-bridge connect --auth-key X` returns exit 0 | UNTESTED | code + tests |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | C | Criteria partially met; substantial negative-path gaps (Windows paths, root prompts, subshells). |
| Verification       | B | Evidence covers criteria and relies on a strong fixture suite, but misses several edge cases. |
| Scope              | A | Diff matches proposal exactly; no scope creep. |
| Reliability        | B | Most error paths handled; the bash script is resilient to missing files. |
| Maintainability    | B | Acceptable structure for awk; logic is reasonably decomposed into functions. |
| Handoff-readiness  | B | Spec updates and verification are clear. |

### Verdict
FAIL

### Recommended next steps
- Fix `scripts/check-doc-authkey.sh` to correctly strip root prompts (`# `) and brackets (`(`, `{`).
- Update `is_command` to handle quoted paths and spaces in paths, and make `.exe` matching case-insensitive.
- Add tests to `scripts/tests/test-doc-authkey.sh` and corresponding fixtures for each of the UNTESTED findings.
- Re-run `bash scripts/tests/test-doc-authkey.sh` to prove the gaps are closed, then re-review.
