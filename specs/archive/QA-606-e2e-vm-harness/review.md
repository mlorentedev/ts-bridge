---
spec: "QA-606-e2e-vm-harness"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "ea0781abf5cce5440690837d5776dd32259782b1"
reviewer: "nan/mimo-v2.6-flash"
date: "2026-11-10"
---

## Adversarial review

**Scope**: `QA-606-e2e-vm-harness` — `git diff 82b1c6918c3b101a2bd58e048f17bb1b5829ff8b...HEAD` at
`ea0781a` (45 commits, 200 files). This is review round 3 (rounds 1–2 in `review-round1.md` /
`review-round2.md`); it judges the whole range, including the ea0781a delta since round 2
(features.json f4 floors, the round-2 disposition record) and how it interacts with rounds 1–2.
**Sources**: `specs/QA-606-e2e-vm-harness/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`, `review-request-round2.json`; `scripts/e2e/{run.sh,linux.sh,windows.ps1}`,
`scripts/check-shellcheck.sh`, `scripts/tests/test-{shellcheck,e2e-report,e2e-mesh-preflight,e2e-probe,e2e-headscale,features-shape,features-selectors}.sh`;
`Makefile`, `.gitignore`, `docs/runbooks/{e2e-vm,guide-development-workflow}.md`,
`.github/workflows/release.yml`, `harness/reviewer-pool.json`; the two git-ignored run logs under
`scripts/e2e/out/`; `gh pr view {703,705}`.

### Spec and task alignment

- **Contract digests are current.** `dotf spec digest QA-606-e2e-vm-harness` returns
  `features.json d9870758…`, `proposal.md 5a57dc14…`, `tasks.md a6327121…` — byte-identical to
  `review-request.json`'s `contract_digests`; `reviewed_sha` equals `git rev-parse HEAD`
  (`ea0781a…`); `reviewer: nan/mimo-v2.6-flash` is spelled exactly as in
  `harness/reviewer-pool.json`. The features.json digest differs from round 2's
  (`38268cc4…` → `d9870758…`) because ea0781a strengthened f4 — the launcher recorded the new
  digests, so this review measures the right contract.
- **Every ticked task has a diff artefact, and I re-ran its check.** `[AC5]` → `report()` +
  `test-e2e-report.sh` (9/9 ok, exit 0, re-run by me). `[AC3]` → preflight/build/down/EXIT
  trap/clean in `run.sh` (read line-by-line; f3 replayed verbatim below). `[AC1][AC4]` → `linux.sh`
  (77 result lines; every `^PASS ` line in the cited log maps 1:1 to its `ok` call sites, empty
  diff). `[AC2][AC4]` → `windows.ps1` (50 lines = 42 static `Case` declarations + 4 `$drifts` keys
  × 2 + RDP-denied × 2 + power × 2; matches the log after CRLF). Makefile targets `e2e`,
  `e2e-linux`, `e2e-windows`, `e2e-clean` present (lines 81–109); `.gitignore:76`
  `scripts/e2e/out/`. `[AC6]` → runbook read in full, linked at
  `guide-development-workflow.md:94`. Closing: shellcheck re-run (`OK (30 scripts)`), features
  shape/selector guards re-run (56 + 64 cases, exit 0), PR #703 open with the spec folder named in
  its body. No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags.
- **AC5 is verified red-green by my own mutation.** Control `bash scripts/tests/test-e2e-report.sh`
  → exit 0. In a `mktemp` copy of the tree with `report()`'s `[ "$xpass" -eq 0 ]` clause deleted:
  `FAIL one-xpass: want fail, got pass`, exit 1. The XPASS gate is real, not decorative.
- **AC4 is verified red-green by my own mutation.** f4's verification command run verbatim against
  the newest logs → exit 0; deleting one `^PASS drift` line from the Linux log → exit 1, from the
  Windows log → exit 1 (both in a `mktemp` copy; the repo tree was not touched). The floors equal
  the true counts (Linux 4, Windows 12 — both script-derived and log-derived by me), so a lost case
  is caught while an added one still passes.
- **AC3's idempotent half is replayed; its interrupt half is not covered** (finding F2). The f3
  command from features.json run verbatim: `make e2e-clean` twice prints "no session left, no
  headscale container" both times, `dotf vm linux|windows status` list no `tsb-e2e-` session,
  exit 0.
- **AC1/AC2 evidence is internally consistent but not re-executed by me**: both cited logs exist,
  and their contents are exactly the recorded counts (77/0/0/0 and 50/0/0/0 PASS/FAIL/XFAIL/XPASS)
  with case names matching `HEAD`'s scripts. I did not re-run the VMs (~6 + ~15 min, out of this
  run's window): **AC1/AC2 full runs UNVERIFIED by re-execution**, verified by artifact inspection.
- **AC6 verified by reading**: preconditions (dotf ≥ 0.79, gh, Go, clean base, Arm64, RAM), trigger
  (release PR breaking/host-facing; after `internal/host` changes), the four result-line meanings,
  and cleanup (incl. "a run killed mid-way … `make e2e-clean`") are all present; the release
  section links it.
- **Round-2/PR-Agent dispositions are recorded, one with a provenance defect** (F1): the f4 floor +
  `[ -n "$l" ]` fix is committed in ea0781a and I proved it fallible; the log-header and
  teardown-test dispositions say "Applied in #705", which is an **open** PR not in this tree.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | verification (disposition provenance) | verification.md's round-2 dispositions claim the log-provenance header and `test-e2e-run-teardown.sh` are "**Applied in #705**", but neither exists at this reviewed sha: `run.sh:205` still echoes `candidate …` **before** the `tee` pipeline (lines 207–209), both cited logs grep `candidate` → 0 lines, and `scripts/tests/test-e2e-run-teardown.sh` is absent from the tree. `gh pr view 705` → **OPEN** (`ci/issue-702-wire-e2e-suites`, unmerged; its commits `eadeea2`/`1907da8` are reachable only via `git log --all`). Round-2's R1/R2 gaps are therefore still open **at this sha**, and the same block names round 2 "at a357428" while the recorded round-2 `review.md` says `5c0bee8`. An archived spec would carry "Applied" for fixes that only land if #705 merges. | `sed -n '203,210p' scripts/e2e/run.sh`; `grep -c candidate` on both logs → 0; `ls scripts/tests/`; `gh pr view 705 --json state` → OPEN; `git merge-base --is-ancestor 1907da8 HEAD` → no; verification.md round-2 block vs `review-round2.md` frontmatter | UNTESTED — the covering test named by the disposition is the file that does not exist at this sha; `test-e2e-mesh-preflight.sh` covers `main_mesh`'s interrupt only (re-run: all pass) | spec artifacts — `verification.md` (outside the contract set): say "#705 (open at round 3)" or merge #705 first, and correct the round-2 sha |
| Minor | REAL | tests (AC3) | AC3's "deletes a leftover session first … whether it passes, fails **or is interrupted**" for `main()` still has no test at this sha: `grep -rln 'interrupt\|SIGINT' scripts/tests/` matches only `test-e2e-mesh-preflight.sh` (the mesh path). The pre-run `down` and the EXIT-trap SIGINT path rest on code reading; the two recorded runs are one per OS, not repeated. | code read of `run.sh:193–212` (trap installed after `down`, replaced after `up`); round-2 R2, tracked as #702 → #705 (open) | UNTESTED for `main()`; mesh analog `test-e2e-mesh-preflight.sh` (ran green, covers `main_mesh` only) | tests — already ticketed (#702); lands with PR #705 |
| Minor | THEORETICAL | preflight | `preflight()` probes only `vm $OS up --help` and hardcodes `$HOME/.local/state/dotfiles/vm/windows/clean-base.qcow2` (`run.sh:37–45`), while the proposal's risk note claims a stronger gate ("checks both and stops with the fix"). A dotf honouring `XDG_STATE_HOME` gets a **false refusal** on a machine whose base exists. Declined twice (rounds 1–2) with a sound false-pass argument; only the false-refusal direction survives. | code read; proposal "Risks / open questions" vs `preflight()` | UNTESTED | code (`run.sh:preflight`) or a proposal note — accepted debt, disposition already recorded |
| Minor | SPECULATIVE | portability | `unzip`/`tar`/`shasum` are not preflighted; a missing one funnels into `die "could not verify $asset"` (`run.sh:52–57`), naming the wrong cause. Declined: the only supported host (macOS Arm64) ships all three. | code read against the runbook's Prerequisites; not reproduced | UNTESTED | code + runbook — surface only; do not gate |
| Question | — | docs | `make e2e` is two recipe lines, so a Linux failure stops the run before Windows; the runbook says "both, one after the other", which does not state failure semantics (round-2 Q1, still open). Current behaviour (stop) is the safer one. | `Makefile:82-84`; `docs/runbooks/e2e-vm.md` Run section | n/a (documentation) | docs/runbooks wording, or a verification.md disposition |

**What I tried to break and could not** (each refuted a plausible escalation):

- **f4 as a rubber stamp.** The floor could have been decorative: it is not — both one-line
  deletions (per OS) flipped it to exit 1, and the empty-`out/` case fails closed (`ls` glob
  unmatched → empty `l` → `[ -n "$l" ] || exit 1`; without that clause `grep -q "" $l` also errors,
  so the PR-Agent "dead guard" observation is true but harmless either way).
- **`report()` fooled by noise or a dead guest.** `noise-only`, `fail-word-mid`, `empty`,
  `no-result`, `RC=1 aborted` fixtures all hold; my XPASS-clause mutant was caught.
- **Upgrade-path asset names guessed.** `run.sh` builds `ts-bridge-$tag-{linux,windows}-arm64.{tar.gz,zip}`
  from `gh release view`'s tagName; `release.yml:84–94` stages exactly those names from
  `needs.release-please.outputs.tag_name` and emits `checksums.txt` via `sha256sum -- *`, which
  `shasum -c` accepts. Re-confirmed myself this round.
- **f3 fail-open.** Re-run verbatim: `dotf vm … status` failure propagates (`|| exit 1`); with a
  missing session both cleans are idempotent.
- **A test suite quietly skipped.** All five spec-adjacent suites re-run green this session:
  `test-e2e-report.sh` (9/9), `test-shellcheck.sh`, `check-shellcheck.sh` (30 scripts incl.
  `scripts/e2e/*.sh`), `test-features-shape.sh` (56), `test-features-selectors.sh` (64), plus
  `test-e2e-mesh-preflight.sh`, `test-e2e-probe.sh`, `test-e2e-headscale.sh`; `go build ./...` → exit 0.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC3–AC6 verified by replay/mutation/reading, AC1/AC2 by artifact inspection (77/50 lines map 1:1 to script cases); no criterion observably broken, but the AC1/AC2 VM runs were not re-executed by me and AC3's interrupt half is untested. |
| Verification       | B | Every AC has a runnable command and I red-greened f4 and f5 myself; the logs still lack a candidate sha (F1) and two round-2 dispositions cite an unmerged PR. |
| Scope              | B | The 22-file QA-606 slice matches the proposal exactly; the range carries ~176 sibling-work files plus `run.sh`'s mesh verbs (QA-607), judged only where they touch the harness (headscale/mesh reach `clean`; five master commits edited the scenario scripts; `release.yml` names the upgrade assets). |
| Reliability        | B | Fails closed on an empty/aborted scenario, a lost drift case, a missing log, a failed teardown and a failed `dotf` status; the main() interrupt path rests on code reading, not a test. |
| Maintainability    | B | Small functions, `why`-comments at the trap and verdict primitives, shellcheck covers `scripts/e2e/*.sh`; the `report()` argument contract (`[RC] log…`) is sharp but documented. |
| Handoff-readiness  | B | Spec artifacts current and digests recorded, rounds 1–2 dispositioned in verification.md, #702 open, runbook linked — held at B by F1's over-stated "Applied" and the wrong round-2 sha in the same block. |

### Verdict

**PASS WITH GAPS** — no Blocker, no Major; two REAL Minors, one THEORETICAL, one SPECULATIVE, one
Question; rubric all B. Per the severity × reality rule, the REAL minors are tracked gaps, not
stop-ship defects: the ACs they touch (AC3's interrupt half) were already open at round 2 under a
passing verdict, and nothing I ran or mutated broke the harness's behaviour.

**UNVERIFIED**: (1) the archive gate's execution — `dotf spec archive` has no dry run and would
move the folder; I observed its `--help` contract (refuses without a fresh, passing `review.md`;
freshness = the digests the launcher recorded) and satisfied both halves I can check (digests match,
`reviewed_sha` = HEAD, no draft tags), but did not run it. (2) Full AC1/AC2 VM re-runs — artifact
inspection only, as stated above.

### Recommended next steps

The contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed**: this verdict permits
the archive, and editing any of them would invalidate the review. These items go to
`verification.md` dispositions (free to edit) or follow-up work — do not head them "before archive":

- **F1 (Minor, REAL)** — in `verification.md`'s round-2 block, change "Applied in #705" to
  "implemented in PR #705 (open at review round 3)" or merge #705 first, and correct
  "at a357428" to `5c0bee8` (the sha the round-2 `review.md` actually records). One-line edits.
- **F2 (Minor, REAL)** — no new ticket needed: #702 is open and PR #705 carries
  `test-e2e-run-teardown.sh`; disposition it in `verification.md` naming the pre-run `down` and the
  SIGINT path so #702's scope is not read as teardown-only. Merging #705 also closes F1's substance.
- **F3/F4** — already declined with reasons; keep as recorded debt, revisit if a non-macOS host is
  ever supported.
- **Q1** — one sentence in the runbook's Run section stating whether `make e2e` stops after a Linux
  failure (current behaviour: yes).

**`dotf spec archive` is advisable in this state** (verdict passes the "fresh, passing review"
bar). Merging #705 afterwards would clear both REAL minors retroactively; waiting for it is not
required by this verdict.
