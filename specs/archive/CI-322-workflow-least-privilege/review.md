---
spec: "CI-322-workflow-least-privilege"
verdict: "PASS"
reviewed_sha: "3c1291330a6e0a803fb29c4c1cd99c336f15e274"
reviewer: "nan/mimo-v2.5"
date: "2026-09-22"
---
## Adversarial review

**Scope**: CI-322-workflow-least-privilege
**Sources**: `specs/CI-322-workflow-least-privilege/{proposal,tasks,verification,features.json}`; `git diff b0b1ec177e6938c6524b92bc6f8e15605b3580c3...HEAD`; commits `27172fe` (#336), `67297da`, `3c12913` (the round-2 fix).

### Spec and task alignment

**What the change is.** The reviewed range carries seven CI-322-attributable commits: the initial PR (`27172fe`, #336), the post-round-1 parser fix (`67297da`), and the round-2 fix that installed zsh, made the suite fail-closed on a missing shell, fixed false reds on CRLF/block-scalar/string/flow-style, refused anchored/tagged `write-all`, and split the awk scanner (`3c12913`). The remaining commits in the range (`00c71b9`, `faaae70`, `26d3ba6`, `6fc72a8`, `a40ffe2`) belong to other specs and are out of scope for this review's findings.

**Verified independently in this session:**

- **AC1** — `bash scripts/check-workflow-permissions.sh` → `OK (8 workflows, 10 checkouts, 10 credential-less, 0 opted out with a reason)`, exit 0. Independently counted: 10 `uses: actions/checkout@` lines and 10 `persist-credentials: false` lines. All eight workflows declare a top-level `permissions:` key; none declares `write-all`.
- **AC2** — Mutation tests on a copy of the real tree: dropping one `persist-credentials: false` from `release.yml` → exit 1 (`no-except release.yml line 37`); removing `ci.yml`'s top-level key → exit 1 (`no-perms`); adding a new workflow with `permissions: write-all` → exit 1 (`write-all`). All three refusals fire on the real tree.
- **AC3** — `bash scripts/tests/test-workflow-permissions.sh` → 53 assertions (26 fixtures × 2 shells + missing-shell self-test), all green, exit 0. Self-test: with a required shell absent, the suite exits 1 (`FAIL [zsh] not installed`). CI installs zsh (`sudo apt-get install -y -qq zsh` in `repo-hygiene.yml`).
- **AC4** — `Repo hygiene` job runs the guard and its suite. `hygiene` is a required status check on `master` (`["test","lint","security","hygiene"]`, `strict: true`).
- **AC5 / features.json** — all five verification commands executed verbatim, all exit 0: `go build ./...`, `go vet ./...`, `go test ./...` (all packages green), `check-lessons.sh` → `OK (32 lessons)`, `check-actions-pinned.sh` → `OK (8 workflow files)`. `shellcheck -s bash` clean on both new scripts.

**What the change gets right.** Every acceptance criterion is met at `3c12913`. The guard catches all four refusal classes on the real tree. The two-shell contract is now enforced: CI installs zsh, the suite self-tests the missing-shell path, and the exact `set -- $sum` regression that lesson-031 was written about would now fail the gate. The awk scanner was split into named functions (`scan_permissions`, `scan_step`, `begin_step`, `report_wa`, `keycol`, `flush`). The proposal's wording now accurately describes what the guard handles and what it does not (YAML aliases as the documented limit, not flow-style steps). The `hygiene` check is a required status check, so a red run blocks the merge.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location |
|----------|---------|------|---------|----------|---------------------------|--------------|
| Minor | REAL | Maintainability | `scan_step` in `check-workflow-permissions.sh` is 65 executable lines — over the repo's 40-line function threshold. The complexity is inherent to a line-based step scanner that must track dash boundaries, step state, checkout detection, persist-credentials matching, and opt-out extraction in a single pass. The function was split out of the monolithic `scan()` in the round-2 fix; further splitting would fragment tightly coupled awk state across functions with no clear seam. | `awk '/^    function scan_step/' scripts/check-workflow-permissions.sh` → 65 lines | Covered by 26 fixtures × 2 shells (all pass) | code or document exemption — the awk line-scanner domain justifies the threshold exceedance |
| Minor | SPECULATIVE | Harness state | All five `features.json` entries have `"state": "pending"` despite their verification commands passing. The harness has not transitioned them to `"action_required"` or `"passing"`. This does not block archiving (the archive gate checks `review.md` freshness and verdict, not `features.json` state), but it leaves the machine-readable record incomplete. | `cat specs/CI-322-workflow-least-privilege/features.json` → all `"state": "pending"` | n/a (harness-side) | harness — or document that `features.json` state is maintained outside the review cycle |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All five ACs verified at `3c12913` with independent evidence; all four refusal classes fire on the real tree; minor false-red gaps are the documented limits of a line-based parser, not defects. |
| Verification       | B | Every features.json command reproduces; 26 fixtures × 2 shells + self-test all green; guard fires on the real tree; missing-shell self-test works; but `features.json` state is stale and verification.md has minor count drift. |
| Scope              | A | CI-322 commits contain nothing outside the proposal; unrelated merges in the range are documented and out of scope. |
| Reliability        | B | Fail-closed on parse errors (exit 2), on missing shells (exit 1), on all decay mutations, and on YAML aliases (documented limit); the guard ran green in CI with the zsh-installed hygiene job. |
| Maintainability    | B | Clear naming, well-documented awk functions, no new dependency, 26 fixtures; `scan_step` at 65 lines exceeds the 40-line threshold, justified by the awk line-scanner domain. |
| Handoff-readiness  | A | Three lessons captured in-PR with index entries, both prior reviews fully dispositioned in `verification.md`, next steps explicit, proposal wording corrected. |

### Verdict
PASS

No blockers, no majors, no D grades. The two prior FAILs (round 1: parser gap on `- name:`-before-`uses:`; round 2: zsh not installed in CI) have been fixed with fixture-backed evidence and mutation-tested on the real tree. The remaining minors are a maintainability threshold exceedance in awk and a harness-state gap in `features.json` — neither gates archiving.

**Archive is advisable.** `dotf spec archive CI-322-workflow-least-privilege` should accept this review: the verdict is PASS, `reviewed_sha` matches `3c12913` (current HEAD), and no contract files (`proposal.md`, `tasks.md`, `features.json`) have been modified by this review. The one unticked box in `tasks.md` ("Independent adversarial review before archive") should be recorded in `verification.md`'s archive checklist, not ticked in `tasks.md` — ticking it would invalidate the staleness check.

### Recommended next steps

All items below are outside the contract set and can be dispositioned in `verification.md` without invalidating this review:

1. **Disposition finding 1 (Minor):** Decline as a documented trade-off. The awk line-scanner domain justifies the 65-line `scan_step` function; further splitting would fragment tightly coupled state across awk functions with no clear seam. Record the exemption in `verification.md`.
2. **Disposition finding 2 (Minor):** Acknowledge as a harness limitation. The `features.json` state is maintained by the harness, not by the spec author, and does not gate archiving. No action required.
3. **Record this review's completion** in `verification.md`'s archive checklist (the `[ ] Independent adversarial review` box), citing this review's SHA and verdict. Do not tick the corresponding box in `tasks.md`.
