---
spec: "CI-491-conflict-marker-gate"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "7bc98be4a537ad98b4c4a3c77475d9edb746eb4d"
reviewer: "nan/deepseek-v4-flash"
date: "2026-10-29"
---

## Adversarial review

**Scope**: `CI-491-conflict-marker-gate` — the conflict-marker guard, its two-shell fixture
suite, the `Repo hygiene` wiring, lesson 048 and the spec triad. Round 2 (round 1 is kept as
`review-round1.md`). Reviewed at `7bc98be` (the base the launcher resolved, `aa3e8c6`, carries
#509/#510/#512 as well; this spec's work is `558886d` = #495 and `7bc98be` = #511, the round-1
fixes). Contract digests recomputed with `dotf spec digest` match `review-request.json` exactly,
so this verdict is fresh against the current `proposal.md` / `tasks.md` / `features.json`.

**Sources**: `specs/CI-491-conflict-marker-gate/{proposal,tasks,verification,features,review-round1}.{md,json}`,
`scripts/check-conflict-markers.sh`, `scripts/tests/test-conflict-markers.sh`,
`.github/workflows/repo-hygiene.yml`, `docs/lessons/lesson-048-2026-10-22.md`,
`harness/reviewer-pool.json`. Throwaway copies under `/tmp` for every mutation; the reviewed
worktree was left as found (only `AGENTS.override.md` and `review-request.json` untracked).

### Spec and task alignment

- AC1–AC4 are each covered by a **named** case: `tracked-conflict`, `bare-open-marker`,
  `untracked-conflict`, `binary-attribute`, `nul-byte` (AC1); `setext-short`, `setext-seven`,
  `indented-example` (AC2); `not-a-repo`, `bare-repo[explicit]`, `bare-repo[all]`, `dotgit/.git`
  (AC3); missing-shell detection plus the zsh half (AC4). I re-ran the suite here:
  `TCM_SHELLS=bash bash scripts/tests/test-conflict-markers.sh` → 16/16 `ok`, exit 0.
- `features.json` now answers. Round 1's Major (f1–f3 pinned `TCM_SHELLS='bash zsh'`, unrunnable on
  a workstation without zsh) is gone: f1–f5 all exit 0 in this environment — verified by running
  each `verification` string under bash, not by reading it.
- The round-1 dispositions are load-bearing, not prose. Mutation battery on throwaway copies
  (`TCM_SHELLS=bash`, each mutant reverted after the run): `-a`→`-I` killed by 2 cases;
  diff3 base dropped, killed by 1; separator `={7,}` put back, killed by 1 (`setext-seven`);
  end-of-line form dropped, killed by 1; findings branch `exit 0`, killed by 7;
  work-tree check removed, killed by 4; re-root to `--show-toplevel` removed, killed by 1;
  `git grep` 0/1 swapped, killed by 12. So `tasks.md`'s mutation claims hold and the round-1
  fixes (`[|]{7,}`, `-a`, both `safe.bareRepository` branches, per-step `if:` reading,
  `TCM_SHELLS` env scan) are enforced.
- f4/f5 are no longer vacuous for the classes round 1 named. `if:` placed *after* `run:` on the
  guard step → f4 exit 1; `|| true` appended to either `run:` line → f4 exit 1; `env:
  TCM_SHELLS: bash` on the suite step → f5 exit 1; zsh install removed → f5 exit 1.
- Real tree: `check-conflict-markers: OK (553 tracked files)` (also when invoked from `docs/`);
  `check-lessons: OK (48 lessons)`; `shellcheck` 0.11.0 on both scripts, exit 0.
- No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags remain in the spec's `.md` files.
- The separator decision (`=======` unmatched) is argued and correct — Git always wraps a
  separator in open/close markers, and matching it would red the acceptance's own
  seven-`=` negative case. Not reopened, per `proposal.md`'s instruction.
- No test was deleted or weakened: both CI-491 commits are net-additive for this spec's files
  (`7bc98be` numstat: 3/1, 42/13 on the two scripts — edits inside the suite, no removed case).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|---|---|---|---|---|---|---|
| Major | THEORETICAL | tests | The fail-closed `git grep` error branch (`*)` → exit 2), which `proposal.md` names as a risk ("anything else is an error that must fail the check rather than read as clean"), is **never exercised**: no case drives `git grep` above exit 1, so a regression that reads an error as clean stays green. I instrumented the branch in a throwaway copy and ran the whole suite: zero hits. | Branch behaves correctly today — corrupt index → `fatal: index file corrupt`, `git grep` 128, guard `git grep failed (exit 128)` exit 2. But `grep -c REACHED` over the instrumented suite log → 0; the three exit-2 cases (`not-a-repo`, `bare-repo[*]`, `dotgit/.git`) all leave through the `--is-inside-work-tree` check, not through this branch. | UNTESTED — no named case; a `corrupt-index` fixture (or any `git grep` ≥2 trigger) is the missing regression test | tests |
| Minor | REAL | guard coverage | A marker line with **no label** — exactly the form case `bare-open-marker` requires — is silently missed when the file is read with CRLF line endings, because the pattern anchors on `( \|$)`: `git grep` sees `<<<<<<<\r`, so neither alternation matches and the guard prints `OK`. Labelled markers (`<<<<<<< HEAD`) are unaffected, the space matches. Reproduced on a throwaway repo `*.txt text eol=crlf` with working-tree bytes `a\r\n<<<<<<<\r\nb\r\n`: guard → `check-conflict-markers: OK (3 tracked files)`, exit 0; the same repo's twin with LF exits 1. Pattern with `\r?` before `$` matches the CRLF line (`n.txt:2:<<<<<<<`, exit 0 from `git grep`). This workstation's Git has `core.autocrlf=true` (every checkout here is CRLF; the suite's own runs print "LF will be replaced by CRLF"), and the guard reads the working tree, not the blob — so the local half of this repo's environment cannot enforce this AC. | Throwaway repos under `/tmp/cmprobe2`: `lf` → exit 1, `crlf` → exit 0. `od -c n.txt` → `a \r \n < < < < < < < \r \n b \r \n`; `git cat-file -p :n.txt` → LF blob, confirming `git grep` read CRLF worktree bytes. | UNTESTED — no CRLF fixture; `bare-open-marker` is LF-only | code (`( \|\r?$)`) + tests (a CRLF case) |
| Minor | REAL | verification contract | f4's step assertion stays green when a guarded step carries `continue-on-error: true` — a step that fails without failing the job, i.e. the gate disabled while the check still reports the wiring is present. Round 1 closed the `if:` half of this class; the failure-suppression half is open. | On a throwaway copy with `continue-on-error: true` added to **both** guarded steps: f4 → exit 0. Control: `|| true` appended to the guard's `run:` → exit 1 (f4 caught that one). Current tree has neither, so nothing is disabled today. | f4 itself is the check; no test asserts it rejects failure suppression | spec (contract set — `features.json` f4) |
| Minor | REAL | code quality | `build()` in the suite is 69 lines with 14 `case` arms (CC ≈ 15), over the repo's own law in `AGENTS.md` (`Function length < 40 lines`, `Cyclomatic complexity < 10`). Low risk — it is a flat fixture table, not branching logic — but the rule is stated as non-negotiable and the exemption is nowhere recorded. | `awk '/^build\(\) \{/,/^\}/' … \| wc -l` → 69; case arms → 14. | n/a (structure, not behavior) | tests (split into helpers, or record the table exemption) |
| Minor | REAL | board / spec state | `dotf spec audit` reports this spec as a **zombie**: `mlorentedev/ts-bridge#491 is CLOSED but the spec is still active`. | `dotf spec audit` → `[FAIL] CI-491-conflict-marker-gate: zombie: …#491 is CLOSED but the spec is still active` (3 fail / 5 active). `gh issue view 491` → `"state":"CLOSED"`. | n/a (state, not code) | board + archive — cleared by the archive this verdict permits; nothing to edit |
| Minor | SPECULATIVE | guard coverage | The same `( \|$)` anchor also misses `<<<<<<<\tHEAD` (tab separator) and `<<<<<<<HEAD` (no separator, e.g. a hand-edited or foreign-tool marker). Git never writes either form, so this is surfaced, not gated. | Probes `tab` and `nosep` → exit 0. | UNTESTED | — (surface only; fold into the CRLF fix if it is touched) |
| Question | REAL | verification | The zsh half of AC4 is not verifiable in this session: `command -v zsh` → absent, so `TCM_SHELLS=bash zsh` exits 1 with `FAIL [zsh] not installed` (which is itself the correct AC4 behaviour), and the two-shell proof rests on the CI run `40573251031` cited in `verification.md`. I did not re-run that CI job; no network or CI access was used. golangci-lint was not run (prohibited on this workstation); it is not implicated by a shell-only change. | `command -v zsh` → missing; `bash scripts/tests/test-conflict-markers.sh` (default `bash zsh`) → exit 1 with the missing-shell FAIL, as designed. | `TCM_SHELLS=bash` run covers 16/16 locally only | — (recorded limit; CI certifies zsh) |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All four ACs hold and every mutant dies, but the unlabelled-marker form silently passes on a CRLF worktree. |
| Verification       | B | f1–f5 all execute and exit 0 here, up from round 1's unrunnable features; the fail-closed branch has no named test and the zsh half is CI-only. |
| Scope              | A | `558886d` and `7bc98be` are pure CI-491 (guard, suite, workflow, lesson, spec); the rest of the reviewed range belongs to other specs and is documented as such. |
| Reliability        | A | 37-line guard, read-only and idempotent, fail-closed outside a work tree and on `git grep` errors (both verified by hand). |
| Maintainability    | B | Guard comments explain *why*; but `build()` is 69 lines / CC ≈ 15 against a stated 40-line / CC-10 law. |
| Handoff-readiness  | B | Lesson 048 written and indexed, `verification.md` candid; `dotf spec audit` still calls the spec a zombie, and lesson 048's diff3 omission lives in open #507. |

### Verdict

**PASS WITH GAPS** — no Blocker, no **REAL Major**, no D grade. The one Major is a
**THEORETICAL** coverage gap (an implemented, correct-by-hand-verification fail-closed branch with
no named test), which under the `severity × reality` rule tracks rather than blocks. The remaining
findings are Minors: one reproduced **REAL** coverage hole (unlabelled markers under CRLF), one
**REAL** f4 vacuity class (`continue-on-error`), one quality-rule excess, and one board state that
the archive itself clears.

`dotf spec archive CI-491-conflict-marker-gate` is **advisable** in this state. The contract set is
closed by this verdict: editing `proposal.md`, `tasks.md` or `features.json` now would invalidate
it, so the f4 finding must be dispositioned, not patched. `specs/CI-491-conflict-marker-gate/` has
no `review.md` besides this one; the contract digests still match `review-request.json`.

### Recommended next steps

1. **Tests (outside the contract set — may land before or after the archive).** Add two named
   cases: a `git grep`-error case (a corrupt index is the cheapest trigger; today it is UNTESTED)
   and a `crlf` case (`*.txt text eol=crlf` plus a bare marker) — the second one currently fails,
   so fix the pattern to `( |\r?$)` in the same PR. (Minor 1 + Major 1 + the SPECULATIVE tab
   form, closed together.)
2. **Disposition the f4 finding in `verification.md`** — applied before this review, ticketed for a
   later contract edit, or declined with a reason. Do **not** edit `features.json` before
   archiving: that stales this verdict by design. Suggested follow-up: make f4 also reject
   `continue-on-error` on the guarded steps (Minor 3).
3. **Code quality (tests).** Split `build()` or record the flat-table exemption in a comment, so the
   40-line / CC-10 rule in `AGENTS.md` and the file agree (Minor 4).
4. **Archive.** `dotf spec archive CI-491-conflict-marker-gate` then closes the `dotf spec audit`
   zombie for #491 (Minor 5) and executes the `verification.md` Archive checklist; #507 (lesson-048
   markers + security-audit required-check list) stays open on its own as the round-1 docs debt.

Not asserted above: the zsh half of AC4 and the CI runs `40567222073` / `40573251031` — no network
or CI access was used in this session, and golangci-lint was not run (prohibited here). Every other
statement is backed by a command run in this session.
