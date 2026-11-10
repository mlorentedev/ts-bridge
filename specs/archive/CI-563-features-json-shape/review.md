---
spec: "CI-563-features-json-shape"
verdict: "PASS"
reviewed_sha: "55cfd68ba8ef0f91d8ecfc12669895e1498e216c"
reviewer: "nan/mimo-v2.6-flash"
date: "2026-11-09"
---

## Adversarial review

**Scope**: CI-563-features-json-shape
**Sources**: `specs/CI-563-features-json-shape/{proposal,tasks,verification,features}.md|json`;
diff `7bd3fe7a4aea5d72a39e2716f7262b42c06fd0f1...HEAD` (HEAD = `55cfd68`, == `origin/master`);
commits `39ad2fc` (#568, the spec's own PR) and `188b33b` (#581, the wiring follow-up it names);
interaction commits `9b6793c` (#673), `64f9a01` (#674).

### Spec and task alignment

- **Range interpretation.** The launcher's base is the parent of #568; `git log --first-parent
  --no-merges base..HEAD` shows no merge commits — HEAD is a linear master at `55cfd68` holding
  ~78 unrelated PRs. Per the launcher instruction I judged the spec's own change first (the six
  files in `39ad2fc`: two scripts + four spec files — exactly what the proposal names, no creep)
  and reviewed only interactions: the hygiene wiring + CI-345 exemption (#581, which edits
  `check-features-shape.sh` itself), and the sibling selector guard (#673/#674).
- **Contract freshness.** `dotf spec digest CI-563-features-json-shape` reproduces all three
  `contract_digests` in `review-request.json` exactly. No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]`
  tags in any spec file.
- **Tasks vs diff.** Every `[x]` has evidence I could reproduce or locate: RED-first is historical
  (47 FAIL lines against a stub, recorded in `verification.md` — stub not retained, but my own
  mutants below serve the same proof); GREEN reproduced fresh: `bash scripts/tests/test-features-shape.sh`
  → `OK (56 cases)` and `zsh scripts/tests/test-features-shape.sh` → `OK (56 cases)`, exit 0 both;
  `shellcheck` on both scripts exit 0; `gh pr view 568` body confirms AC2's real-tree listing
  (`1 of 31`, CI-345 named, wiring explicitly deferred to #574 — the implementer correctly
  refused to add the exemption mechanism themselves).
- **Acceptance criteria (fresh runs).**
  - AC1 "names every deviating file": 20 negative fixtures per shell (top-level object/string,
    empty array, scalar/null/array element, missing/extra/renamed key, null/number value,
    second-object-bad, invalid JSON, empty file, path-with-space), plus `every-bad-named`
    (all 3 files named) and `only-bad-named` (conforming file absent from report). ✔
  - AC2 real tree: `bash scripts/check-features-shape.sh` → `OK (39 features.json files)`,
    exit 0, with CI-345 printed as `exempt, …`. Issue #563 and follow-up #574 both CLOSED. ✔
  - AC3 failing-first: four mutant probes run in a `mktemp` copy (never the repo) all fail the
    suite — findings-disabled (42 FAIL), exit-flipped (40 FAIL), string-type-check-disabled
    (**4 FAIL**, exactly `null-value`/`number-value` × both shells as `verification.md` claims),
    JSON-validity-disabled (**4 FAIL**, as claimed), missing-key-disabled (10 FAIL). ✔
  - AC4 read-only: suite's byte-identical check on `archived-bad` passes; neither `39ad2fc` nor
    `188b33b` touches `specs/archive/`. ✔
- **Interactions.** The hygiene workflow (`.github/workflows/repo-hygiene.yml`) runs both steps
  after the zsh install (verified ordering), and the latest `repo-hygiene` run on the reviewed
  sha `55cfd68` is `success`. The sibling guard `check-features-selectors.sh` (#673) passes over
  this spec's `features.json` (`OK (50 go test -run selectors)`), so nothing in the later work
  conflicts with this spec's shape contract.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL (reproduced) | coverage | A symlink named `features.json` is silently skipped by `find … -type f`, so a deviant file behind a symlink reads as a clean tree — AC1's "names every `features.json`" does not hold for symlinks | probe in `mktemp`: symlink to `{"feature_id":"evil"}` → `check-features-shape: OK (0 features.json files)`, exit 0; no symlink exists in the real tree today | UNTESTED (no symlinked-file fixture) | tests (+ optionally code: `find -L` / `-type l` handling) |
| Minor | REAL (reproduced) | error-reporting | An unreadable `features.json` (EACCES) is reported as `not valid JSON … differ from the scaffold` (exit 1) instead of `could not run` (exit 2); jq's stderr is discarded, hiding the permission error — the same discard class as the `find` defect already fixed in this script | probe: `chmod 000` a conforming file → exit 1, `…/features.json: not valid JSON (want exactly one document)`; fails safe (non-zero, file named), only the label/exit-code class is wrong | UNTESTED (suite has `unreadable-dir`, no unreadable-file fixture) | code + tests |
| Minor | REAL (observed at HEAD) | spec-vs-code drift | `proposal.md` still says "Neither runs in CI yet" and "the real-tree step stays red on CI-345 … until the owner exempts or normalises it", but #581 wired both steps and exempted CI-345; accordingly `verification.md`'s real-tree evidence ("reports 1 of 31: specs/archive/CI-345 …") no longer reproduces — HEAD reports `OK (39 features.json files)` with that file exempt | `bash scripts/check-features-shape.sh` (exit 0, exempt line) vs `verification.md` §Evidence; proposal §What/§Risks | n/a (observed) | spec artifacts: correctable in `verification.md` (outside the contract set); `proposal.md` is contract set — disposition, do not edit under this verdict |
| Minor | REAL | spec artifacts | `proposal.md`'s four acceptance criteria are all `- [ ]` and `tasks.md` Closing `Type checks pass` is `- [ ]`, while every other box is `[x]`; no archive pre-flight checks these, so they read as unfinished rather than waived | read of both files; `dotf spec archive --help` lists only draft-tag, review-freshness and promotion pre-flights | n/a | spec artifacts (record the disposition in `verification.md`; do not edit the contract set under this verdict) |

No Blockers, no Majors. The two code findings are fail-safe behaviors at the margin (both exit
non-zero over the real tree), both reproduced by me and both UNTESTED — which is why they are
tracked rather than blocking. Test deletions: none in the spec's commits (only additions).

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | All five ACs verified fresh with 56 fixtures × 2 shells + real tree; two edge paths (symlink skip, unreadable-file mislabel) found by probing, both fail-safe |
| Verification       | B | Evidence is reproducible and mutant claims reproduce exactly; the real-tree claim is stale-by-design (later #581 changed the outcome it recorded) rather than wrong |
| Scope              | A | The spec's own PR touches exactly the six files the proposal names; deferred CI wiring went through its own issue (#574/#581); unrelated range contents excluded per instruction |
| Reliability        | B | Exit contract (0/1/2) honored for clean/differs/no-jq/missing-dir/find-failure; one read-failure path lands on exit 1 with a wrong label |
| Maintainability    | B | Heavily commented, shellcheck-clean, flat fixture table with a documented deliberate exception to the 40-line rule; one hardcoded exemption path with printed reason and three tests scoping it |
| Handoff-readiness  | B | Proposal/tasks/verification/features complete and internally consistent, promotions answered with reasons; held back only by the unticked boxes and the stale real-tree line |

Aggregation: no D, no C → all B or above → PASS; severity axis finds no Blocker and no Major.

### Verdict
**PASS**

Severity × reality: no Blocker, no REAL Major, no Major at all. The four Minors are all tracked
dispositions; the two UNTESTED ones are Minor, so the UNTESTED rule does not gate a PASS.

### Recommended next steps

(Routed to `verification.md` dispositions or a follow-up ticket — the contract set
`proposal.md`/`tasks.md`/`features.json` is closed under this verdict; do not edit it, that
would stale the review.)

1. Disposition finding 1 (symlink skip): a follow-up fixture `symlinked features.json → exit 1`
   plus `find -L`/`-type l` handling in `check-features-shape.sh` — tests + code.
2. Disposition finding 2 (unreadable file): distinguish read failure (`exit 2`, surface jq's
   error) from invalid JSON, with an `unreadable-file` fixture — code + tests.
3. Record in `verification.md` that the real-tree line and the "not wired into CI" statement are
   superseded by #581 (`OK (39)`, CI-345 exempt, hygiene wiring green on `55cfd68`), and that the
   unticked proposal/Closing boxes are accepted as-is for archive.
