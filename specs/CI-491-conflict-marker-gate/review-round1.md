---
spec: "CI-491-conflict-marker-gate"
verdict: "FAIL"
reviewed_sha: "813d8980ee44c7b8f2985d11fa8494aaaacc8699"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-27"
---

## Adversarial review

**Scope**: `CI-491-conflict-marker-gate` — the conflict-marker guard, its two-shell fixture
suite, the hygiene wiring, lesson 048 and the spec triad. Reviewed range
`aa3e8c6..813d898` (the base the launcher resolved), i.e. #495 (`558886d`, all of this
spec's code) plus #501 (`813d898`, DEPS-312 follow-ups, not this spec's work — see Scope).
Contract digests recomputed with `dotf spec digest` match `review-request.json` exactly, so
this review is fresh against the current `proposal.md` / `tasks.md` / `features.json`.

**Sources**: `specs/CI-491-conflict-marker-gate/{proposal,tasks,verification}.md`,
`features.json`, `scripts/check-conflict-markers.sh`, `scripts/tests/test-conflict-markers.sh`,
`.github/workflows/repo-hygiene.yml`, `docs/lessons/lesson-048-2026-10-22.md`,
`docs/lessons/_index.md`, `harness/reviewer-pool.json`, `docs/troubleshooting/security-audit.md`.

### Spec and task alignment

- AC1 (tracked marker fails with `file:line`, untracked does not), AC2 (setext 3 and exactly 7
  green), AC3 (exit 2 outside a work tree, real tree clean) and AC4 (both shells, fail when one
  is missing) are each covered by a **named** case in `scripts/tests/test-conflict-markers.sh`.
  I re-ran all 13 cases under bash: 13/13 `ok`. The real tree reports
  `check-conflict-markers: OK (541 tracked files)`; `check-lessons.sh` reports
  `OK (48 lessons)`; `shellcheck` 0.11.0 on both scripts exits 0.
- The separator decision (`=======` unmatched) is argued in `proposal.md` and is correct: Git
  always emits open/close markers around a separator, and the acceptance's own negative case
  (a seven-`=` setext underline) is the false positive it would create. I did **not** reopen it.
- Fault injection confirms the guards are load-bearing, not decorative. On throwaway copies:
  separator `={7,}` back in the regex → killed (by `setext-seven`); `git grep` exit 1 not
  treated as OK → killed (5 cases); end-of-line form dropped → killed (`bare-open-marker`);
  findings branch `exit 0` → killed (5 cases); diff3 base dropped → killed (4 cases); work-tree
  check removed → killed (`bare-repo`, `not-a-repo`, `dotgit/.git`); scan narrowed to `docs/`
  → killed; re-rooting to `--show-toplevel` removed → killed (`nested/sub`). The four mutants
  claimed in `tasks.md` all red the suite as claimed. Two survivors are semantically harmless:
  dropping `-I` (stricter) and dropping the `-- .` pathspec (no-op once the guard re-roots to
  the toplevel — which is itself separately covered).
- No test was deleted or weakened: every CI-491 file in the range is pure addition
  (`--numstat`: 0 deletions), so the retention bar does not apply here.
- `tasks.md` and `proposal.md` disagree with reality in two small places (F4).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|---|---|---|---|---|---|---|
| Major | REAL | verification contract | `features.json` f1–f3 pin `TCM_SHELLS='bash zsh'`. The suite deliberately fails when a shell is absent (AC4), and the machine that consumes `features.json` is this workstation — which has no zsh (`command -v zsh` → missing; `dotf spec digest` runs locally, nothing in CI or the Makefile reads `features.json`). So all three `verification` commands exit 1 here and can never capture exit 0, leaving `state` permanently `pending`: a feature list that cannot answer. Exactly the shape lesson-030 records. | `TCM_SHELLS='bash zsh' bash scripts/tests/test-conflict-markers.sh` → `FAIL [zsh] not installed: both shells are required`, rc=1 (run this session); identical command with `TCM_SHELLS=bash` → rc=0, 13/13. Nothing in `.github/`, `Makefile` or `harness/` consumes `features.json`. | UNTESTED — no test asserts a feature's `verification` command can run in the harness's own environment | spec (`features.json`: shell-aware commands, or a recorded machine prerequisite) |
| Minor | REAL | contract non-vacuity | f4's verification is two `grep -q` calls for the script paths in the workflow. It stays green if the steps are neutered (e.g. `if: false`, moved under a job that no longer runs, or the `run:` body replaced) — the strings only prove the paths are *mentioned*. | `grep -q scripts/check-conflict-markers.sh …repo-hygiene.yml && grep -q scripts/tests/test-conflict-markers.sh …` → rc=0, and would also be rc=0 against a disabled step | UNTESTED | spec (`features.json` f4) |
| Minor | REAL | guard coverage | `git grep -I` skips files Git treats as binary, so a marker in one is **silently** missed (exit 0, "OK"). Reproduced two ways: `*.md binary` in `.gitattributes`, and a NUL byte in a `.txt`; control without them fails correctly. The guard's whole value is the silent-pass class it removes. | probes in `/tmp/cmprobe`: attribute case `OK (2 tracked files)` rc=0, NUL case `OK (1 tracked files)` rc=0, control `doc.md:2:<<<<<<< HEAD` rc=1 | UNTESTED — no fixture holds a NUL file or a `binary` attribute | code (`-a`/`--text`) + tests |
| Minor | REAL | test determinism | The `bare-repo` case's outcome depends on the ambient `safe.bareRepository`: with `explicit` (this workstation) `rev-parse` errors, with `all` it prints `false`. Both yield exit 2, so the suite is green either way — but it exercises only one branch per machine, and the "with and without that override" claim in `verification.md` is prose, not encoded. | `git config --get safe.bareRepository` → `explicit`; suite green here and (per `verification.md`) under `GIT_CONFIG_*…=all` | named case `bare-repo` — covers one branch only | tests (set both configs in the suite) |
| Minor | REAL | docs vs code | `lesson-048` says the guard "fails on any tracked line that starts with a conflict open or close marker". The guard also matches the diff3 base `\|\|\|\|\|\|\|`, and the suite has a `diff3-base` case for it. The lesson understates the contract it documents. | `check-conflict-markers.sh:19` regex vs `lesson-048`:14-16 | UNTESTED (doc accuracy; no gate) | docs (`docs/lessons/lesson-048-2026-10-22.md`) |
| Minor | REAL | spec drift | `tasks.md` keeps `[ ] PR opened referencing this spec folder` unticked although #495 and #501 are merged and `verification.md` cites them; `proposal.md` still declares `status: implementing` while the change is in verification. | `tasks.md` Closing section; `proposal.md` frontmatter | UNTESTED | spec (contract set — same re-review round as the Major) |
| Minor | THEORETICAL | portability | The diff3 base is written `\|{7,}` inside a POSIX **extended** regex, where the meaning of an escaped ordinary character is undefined; GNU and BSD both read it as a literal `|`, and `repo-hygiene.yml` runs on Linux only, so no CI run exercises it elsewhere. `[|]{7,}` expresses the same set with no escape and no interpretation at stake. | `check-conflict-markers.sh:19`; `repo-hygiene.yml` `runs-on: ubuntu-latest` | cases `diff3-base` (Linux/GNU only) | code (one character class) |
| Question | REAL | gating posture | Whether `Repo hygiene` is a required status check is not verifiable from this worktree, and the repo's own audit lists a different set (`test`, `lint`, `security`, `shellcheck` — and no `shellcheck` *job* exists any more, so that document is itself stale). If `hygiene` is not required, the guard still reds the PR and this repo's merge discipline blocks it, so the exposure is smaller than a missing gate — but the two records should be reconciled. | `docs/troubleshooting/security-audit.md:124`; `grep -n '^  …:$' ci.yml` shows `test`, `test-windows`, `test-macos`, `smoke`, `build-matrix`, `lint`, `security` | UNTESTED | docs (audit) / repo settings |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All four acceptance criteria hold and fault injection kills the guard in every way that matters; one silent-miss class (binary/NUL files) is uncovered. |
| Verification       | C | Local evidence is strong and reproducible for bash and the real tree, but the machine-readable `verification` commands fail in the harness's environment, and the zsh half rests on a cited CI run (run `40573251031`) I could not re-run here — zsh is absent, so the two-shell criterion is unverified in this session. |
| Scope              | B | #495 matches the proposal file-for-file; the reviewed range also contains #501's DEPS-312 changes, which belong to another spec (documented, not this spec's creep). |
| Reliability        | A | 35-line guard, fail-closed on `git grep` errors and outside a work tree, idempotent, no state, no writes. |
| Maintainability    | A | Comments explain *why* (the separator decision, the `git grep` exit codes, the fixture-assembly constraint); shellcheck clean; cyclomatic complexity far below 10; no dead code. |
| Handoff-readiness  | B | Lesson written and indexed, `verification.md` is unusually candid; but `features.json` is permanently `pending`, and the spec's own status and PR box lag reality. |

### Verdict

**FAIL** — one **REAL Major** in the contract set (`features.json` verification commands cannot
run where the harness runs). Per the reality rule this is the one class a SPECULATIVE finding may
not do and a REAL one cannot escape: the fix lands in `features.json`, which is measured by the
archive gate, so the next round is the mechanism rather than an inconvenience. No Blocker, no D
grade; the guard itself is sound.

### Recommended next steps

1. **Contract set (this is what forces the re-review).** Make f1–f3 runnable in the environment
   that consumes them — e.g. f1/f2 on `TCM_SHELLS=bash` (bash is always present), f3 unchanged as
   the real-tree run, and carry AC4 in its own feature whose command tolerates a missing shell
   *or* is documented as CI-only with that stated — and make f4 prove the step *runs* (assert the
   `run:` line, not just the path string). Then run `dotf spec review CI-491-conflict-marker-gate`
   again; the current verdict is invalidated by design the moment `features.json` changes.
2. **Code + tests (outside the contract set, may land in the same PR).** Close the silent-skip
   class: search binary files (`-a`) or record the limitation and a fixture holding a NUL file;
   force both `safe.bareRepository` branches inside the suite instead of inheriting the ambient
   config; prefer `[|]{7,}` over `\|{7,}`.
3. **Spec artifacts (same round as (1), or dispositioned in `verification.md`).** Tick the PR box
   in `tasks.md` and move `proposal.md` to `verifying`.
4. **Docs, ticketable.** Correct `lesson-048` to name the diff3 base marker, and reconcile
   `docs/troubleshooting/security-audit.md`'s required-check list with the workflows that exist
   today (including whether `Repo hygiene` blocks a merge).

Not asserted anywhere above: the zsh half of AC4 in this session, and the cited CI runs
(`40567222073`, `40573251031`) — no network or CI access was used, and golangci-lint was not run
locally on this workstation. Every other statement is backed by a command I ran here.
