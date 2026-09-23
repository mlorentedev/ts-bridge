---
spec: "CI-314-pr-agent-reviewer"
verdict: "PASS"
reviewed_sha: "39dd339361f411ccada961d03014687ab43c0806"
reviewer: "nan/mimo-v2.5"
date: "2026-09-22"
---
## Adversarial review

**Scope**: CI-314-pr-agent-reviewer
**Sources**: `specs/CI-314-pr-agent-reviewer/{proposal,tasks,verification,features}.md`, `.github/workflows/pr-agent.yml`, `.pr_agent.toml`, `harness/review-attestation.json`, git diff `ebb544d2530f1504f366ebe2655579f045195e7d...HEAD`

### Spec and task alignment

- **proposal.md** now correctly states PR-Agent reviews "every non-draft, non-dependabot, non-release PR opened from a branch of this repository (not from a fork)" — matching the workflow `if:` condition exactly. The fork exclusion was the prior round's Major finding; commit `39dd339` fixed the contract to match the code, with the "pwn request" security rationale documented in the Out of Scope section.
- **tasks.md** AC1–AC3 are ticked with run IDs; AC4 (coverage) is correctly left open at 11/15.
- **verification.md** carries post-merge evidence with specific run IDs and timestamps for AC1–AC3 and AC5. AC4 is tracked with a counting method the re-reviewer can reproduce.
- **features.json** f1 and f2 both pass when verified (step ordering and registry parse). Both remain `"state": "pending"` — the harness has not yet set `"state": "passing"`, which is correct per the gating rule.
- actionlint produced zero errors on `pr-agent.yml`.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | THEORETICAL | Reliability | `cancel-in-progress: true` in the concurrency group means a second push within the Docker build window (~20s) cancels the first run's review. If PR-Agent had already posted a comment but not updated `updated_at`, the guard would fail the cancelled run. This is by design (documented in the workflow comments) and acceptable for a review tool — the next push triggers a fresh review. | workflow comments, concurrency group config | UNTESTED (post-merge observation only; AC3 confirms push re-review works) | — (by design, no fix needed) |
| Minor | THEORETICAL | Security | The bootstrap fallback hardcodes `marker="PR Reviewer Guide"` when the default branch has no registry entry yet. A malicious PR could craft a head-registry entry with a different marker to bypass the base-branch check, but the guard's comment-match still requires `github-actions[bot]` authorship and `updated_at >= $started`, so spoofing requires posting a bot-authored comment before the run starts — which requires workflow dispatch access. The hardcoded string is actually safer than a head-supplied one (CWE-345, documented in the comment). | workflow guard, bootstrap fallback block | UNTESTED | — (documented, safe by construction) |
| Minor | N/A | Spec | AC4 (coverage ≥ 90% over 15 PRs) remains open at 11/15. This is by design — the spec explicitly requires 15 PRs of observation. 11/11 reviewed so far; 100% hit rate. | verification.md | N/A | — (waiting on data, no action needed) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All five acceptance criteria verified or in progress; fork exclusion now matches contract. |
| Verification       | A | Post-merge evidence with specific run IDs, timestamps, and a reproducible counting method for AC4. |
| Scope              | A | Diff is CI/config/spec only; no Go code, no new dependencies, no unrelated changes. |
| Reliability        | B | Guard script handles edge cases (null body, missing registry, credential absence) with clear error messages; `cancel-in-progress` is the documented trade-off. |
| Maintainability    | A | Every decision in workflow and toml is annotated with measured evidence and issue references; Cyclomatic complexity is not applicable to YAML/TOML. |
| Handoff-readiness  | A | Spec files updated, lesson candidate recorded, fork exclusion documented with rationale. |

### Verdict
PASS

All five acceptance criteria are verified or in progress with a reproducible counting method. The prior round's Major (spec vs code mismatch on fork exclusion) was fixed in `39dd339`, the HEAD commit. The remaining Minor findings are THEORETICAL or by-design trade-offs that do not gate archive.

### Recommended next steps
- **AC4 (coverage)**: Continue counting. The window needs 4 more eligible PRs to close at ≥ 90%. Current rate is 11/11 (100%). When 15 PRs are reached, tick AC4 in `proposal.md` and `tasks.md`.
- **features.json**: The harness should run f1 and f2 verification commands, capture exit codes, and set `"state": "passing"` with evidence. This is a harness action, not a spec-file edit.
- **Archive**: Once AC4 closes and the harness sets features.json to passing, `dotf spec archive CI-314-pr-agent-reviewer` should succeed. The review.md is fresh (this SHA) and the contract set has not changed since it was written.
