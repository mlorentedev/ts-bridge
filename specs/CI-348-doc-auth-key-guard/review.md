---
spec: "CI-348-doc-auth-key-guard"
verdict: "FAIL"
reviewed_sha: "177355cea297e6026a38234935dec463ae7d10bc"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-10-06"
---

## Adversarial review

**Scope**: CI-348-doc-auth-key-guard
**Sources**: `specs/CI-348-doc-auth-key-guard/{proposal,tasks,verification}.md` + `git diff 0b23df6e9b8e1298b66f84d3c21d4116b420770d...HEAD`

### Spec and task alignment
- The diff cleanly matches the proposed file additions (guard script, test suite, and workflow wiring) without scope creep.
- However, critical parsing flaws in the awk state machine mean the guard silently ignores common valid documentation forms, violating the primary acceptance criterion.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Blocker | REAL | parsing | Shell prefix bypass: `is_command` requires the segment to start exactly with `ts-bridge` or `.\ts-bridge`. A valid command prefixed with `sudo`, an environment variable (`VAR=1 ts-bridge`), or an absolute path is silently ignored. | Demonstrated locally | UNTESTED | code + tests |
| Blocker | REAL | parsing | PowerShell continuation conflicts with Markdown inline code spans. Any line ending in a backtick is treated as continued, stripping the trailing backtick. For a whole-line inline code span (`` `ts-bridge...` ``), this breaks the inline-span regex in `normalize` and appends the next line. The leading backtick remains, failing `is_command`. | Demonstrated locally | UNTESTED | code + tests |
| Major | REAL | parsing | Pipe splitting gap: The command splitting regex `/(;\|&&\|\|\|)/` does not split on single pipes. A piped command like `yes \| ts-bridge connect --auth-key 123` is evaluated as starting with `yes`, failing `is_command` and bypassing the guard. | Demonstrated locally | UNTESTED | code + tests |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | D | The guard silently fails to detect `--auth-key` in very common documentation forms (sudo, pipes, inline code spans), breaking its primary invariant. |
| Verification       | C | Existing tests pass, but negative-path tests for these common shell and markdown syntax structures are missing. |
| Scope              | A | Diff perfectly matches the proposal without creep; zero unrelated changes. |
| Reliability        | C | The awk parsing state machine is brittle against standard shell prefixes and markdown backticks. |
| Maintainability    | C | A 100-line awk regex parser for shell/markdown is fragile; future syntax will likely require more regex tweaks. |
| Handoff-readiness  | B | Spec is updated and artifacts captured, but the implementation has significant holes requiring fixes. |

### Verdict
FAIL

### Recommended next steps
- Fix `is_command` to tolerate common command prefixes (`sudo `, env var assignments) and absolute paths. Add named fixture tests for these.
- Refactor the PowerShell continuation logic to distinguish between a trailing backtick used for continuation and a Markdown closing backtick. Add a named fixture test for a whole-line inline code span.
- Extend the command splitting regex to handle single pipes (`|`). Add a named fixture test for piped commands.
- Do not run `/spec archive` or `dotf spec archive` in the current state. Fix the bugs and run another adversarial review first.
