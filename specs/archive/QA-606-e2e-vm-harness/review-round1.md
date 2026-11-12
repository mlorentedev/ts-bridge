---
spec: "QA-606-e2e-vm-harness"
verdict: "FAIL"
reviewed_sha: "55cfd68ba8ef0f91d8ecfc12669895e1498e216c"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-09"
---

## Adversarial review

**Scope**: `QA-606-e2e-vm-harness` — `git diff 82b1c6918c3b101a2bd58e048f17bb1b5829ff8b...HEAD` (172 files; the
QA-606 slice is `scripts/e2e/`, the four `make e2e*` targets, `docs/runbooks/e2e-vm.md`, the
`.gitignore` entry and the `AGENTS.md` row; the rest of the range is sibling specs' work merged into
the same files, judged only where it interacts with the harness)
**Sources**: `specs/QA-606-e2e-vm-harness/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`; `scripts/e2e/{run,linux,headscale,mesh_linux}.sh`, `scripts/e2e/windows.ps1`,
`scripts/e2e/probe.py`; `scripts/tests/{test-e2e-report,test-e2e-probe,test-e2e-mesh-preflight}.sh`;
`Makefile`; `docs/runbooks/e2e-vm.md`; `scripts/check-shellcheck.sh`; `.github/workflows/*`

### Spec and task alignment

- Contract digests in `review-request.json` match `dotf spec digest QA-606-e2e-vm-harness` exactly
  (`proposal.md`, `tasks.md`, `features.json`) — the review is of the requested state.
- Every ticked task in `tasks.md` has a diff artefact: `report()` and its fixture suite exist
  (`test-e2e-report.sh`, 9 cases, exit 0), `run.sh` has preflight / checksum-verified previous
  release / leftover-session teardown / `clean`, `linux.sh` and `windows.ps1` exist, the four
  Makefile targets exist, `scripts/e2e/out/` is git-ignored, and `docs/runbooks/e2e-vm.md` is linked
  from item 5 of the release section of `guide-development-workflow.md`. No `[AGENT-DRAFT]` /
  `[AGENT-SUGGESTION]` tags remain.
- AC6 verified by reading (preconditions, trigger, result table, cleanup all present).
- AC5 verified by running: `bash scripts/tests/test-e2e-report.sh` → 9/9 ok, exit 0; red-green by
  mutation (see findings E3).
- AC1/AC2/AC4 are **not** reproducible in this session: the harness needs `dotf vm` guests with a
  built Windows base, and the cited logs (`scripts/e2e/out/*.log`) are git-ignored, so no run of the
  reviewed tree is reachable from the repository. The recorded evidence is real but external; what
  can be checked in-tree is checked below.
- The harness scripts at `HEAD` are byte-identical to the states two sibling specs recorded green
  runs on (`git diff 66f28d6 HEAD -- scripts/e2e/linux.sh` empty → 77 pass/0 fail per
  `SEC-631/verification.md`; `git diff 310bfc6 HEAD -- scripts/e2e/windows.ps1` empty → 50 pass/0
  fail). That mitigates, without replacing, QA-606's own stale numbers (finding V2).

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Major | REAL | spec evidence (contract set) | `features.json` f4's recorded `verification` command asserts **9** `^PASS drift` lines for Windows; the current `windows.ps1` emits **12** (4 drifts × 2 "seen/repaired" + `drift: RDP denied seen by check` + `… repaired by setup` + `drift: power reset seen by check` + `drift: power reset is changed, then ok`). AC4's stated proof therefore cannot pass on the reviewed tree. | Ran the real `Result`/`Case`/`$drifts` block under `pwsh` (verbatim from `windows.ps1`) → 12 `PASS drift:` lines; `git show 03e077c:scripts/e2e/windows.ps1` shows 3 drift keys → exactly 9, i.e. the number was correct only at the commit the evidence cites. SEC-631 later added the 4th drift and the "power reset seen by check" case to the same file. Linux's half (`-eq 4`) still matches. | The command itself is the test; nothing guards `features.json`'s shell verifications (`scripts/check-features-selectors.sh` only checks `go test -run` selectors) → **UNTESTED** | spec — `features.json` f4 (contract set: fix now, then re-review) |
| Minor | REAL | verification | `verification.md` and `features.json` f1/f2 evidence cite "37 pass" (Linux) and "39 pass" (Windows) at `03e077c`; the reviewed scripts emit far more cases, and the newest recorded runs live in sibling specs' `verification.md` (77 and 50 pass). A reader of the archived spec cannot tell which run describes the archived code. | `git diff 66f28d6 HEAD -- scripts/e2e/linux.sh` empty; `git diff 310bfc6 HEAD -- scripts/e2e/windows.ps1` empty; `specs/SEC-631-scoped-firewall-rules/verification.md` records 77/50 pass on those contents | UNTESTED (documentation) | spec — `verification.md` (outside the contract set, free to edit) |
| Minor | REAL | quality / lint scope | The new harness scripts are outside `scripts/check-shellcheck.sh`'s deliberate scope (`scripts/*.sh`, `scripts/tests/*.sh`, one level); `scripts/e2e/run.sh` is therefore never linted, and shellcheck reports 2 live warnings on it: SC2034 (line 120, unused loop variable `i`) and SC2155 (line 129, `local dir="$(...)"` masks `hs_host_ip`'s status). `tasks.md`'s "shellcheck passes" is true only because the glob excludes the change. | `bash scripts/check-shellcheck.sh` → "OK (26 scripts)"; `shellcheck -x scripts/e2e/*.sh` → SC2034, SC2155, exit 1 | UNTESTED for `scripts/e2e/` (`test-shellcheck.sh` fixtures cover only the two documented depths) | code + tests — `scripts/check-shellcheck.sh` glob, `scripts/e2e/run.sh`, `test-shellcheck.sh` fixture |
| Minor | REAL | CI wiring | None of the four new `scripts/tests/test-e2e-*.sh` run in any workflow — including `test-e2e-report.sh`, the only automated guard for AC5's verdict semantics (`report()`). Every other repo guard is a step of the `hygiene` job. | `grep -rn e2e .github/workflows/` → no match; `repo-hygiene.yml` lists each guard by name | `scripts/tests/test-e2e-report.sh` (runs locally, exit 0; unwired in CI) | code — `.github/workflows/repo-hygiene.yml` (owner-only per the proposal's non-goals → ticket) |
| Minor | REAL | tests (AC3) | The teardown trap of the **primary** path (`main()` for `linux`/`windows`) is exercised by no test; the only interrupt case covers `main_mesh`'s `teardown_mesh`. AC3 ("whether it passes, fails or is interrupted") rests on one manual log plus an analogue. | `scripts/tests/test-e2e-mesh-preflight.sh` case `interrupt-tore-down-the-guest` covers the mesh trap only; no shim-level suite for `run.sh linux`; I confirmed the underlying bash behaviour separately (EXIT trap runs when SIGINT reaches the process group) | `test-e2e-mesh-preflight.sh:interrupt-tore-down-the-guest` (mesh only) → UNTESTED for `main()` | tests — reuse the `test-e2e-mesh-preflight.sh` shim technique for `run.sh linux` |
| Minor | THEORETICAL | preflight assumptions | Preflight gates on `dotf vm <os> up --help` only, not on `exec --script` / `cp`, which the run depends on; and it hardcodes `$HOME/.local/state/dotfiles/vm/windows/clean-base.qcow2`, duplicating dotf's state layout — wrong in both directions under `XDG_STATE_HOME` or a dotf layout change (false refusal, or a false pass that fails later and opaquely). | code read of `preflight()`; the proposal's own risk note ("checks both and stops with the fix") claims a stronger gate than the code performs | UNTESTED | code — `scripts/e2e/run.sh:preflight` |
| Minor | REAL | spec evidence (contract set) | f3's verification is fail-open: with `dotf` absent, `dotf vm … 2>/dev/null` exits non-zero, so `! … | grep -q '^tsb-e2e-'` succeeds and the criterion "no session left" reads as proven when nothing was queried. | demonstrated: `env PATH=/usr/bin:/bin` → f3's third clause passes with no `dotf` installed | the command itself; UNTESTED (nothing asserts it can fail) | spec — `features.json` f3 (contract set) |
| Minor | SPECULATIVE | reliability | `report()` has no lower bound on case count beyond "≥1 result line", so a scenario that loses a whole block of cases while still printing one `PASS` exits 0; a deleted case is invisible unless a human compares the summary. | fixture case `clean` (2 PASS lines) passes; `report()`'s only floor is `pass + xfail > 0` | `scripts/tests/test-e2e-report.sh:empty` / `no-result` prove the floor exists, not that it is adequate | code or tests — the count belongs in the scenario's own assertions, not in `report()` |

**Positive control, stated only because it mitigates a listed risk**: the verdict logic is genuinely
guarded — mutating `[ "$xpass" -eq 0 ]` out of `report()` turns `one-xpass` red, and removing the
`rc` check turns `aborted` red, so E3's red-green claim is not a single passing run.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Every AC has a named artefact; negative paths are unusually thorough in the guest scenarios; AC3's non-mesh interrupt path and AC4's recorded proof are the gaps. |
| Verification       | C | AC4's recorded verification command is arithmetically wrong on the reviewed tree, the cited logs are git-ignored, and AC1/AC2's numbers describe a two-generation-old script. |
| Scope              | B | The QA-606 slice is exactly the harness, its tests, the Makefile targets and the runbook; the mesh machinery in the same files belongs to QA-607 and is separable. |
| Reliability        | B | Fail-closed design throughout (checksum grep, `tee` write check, teardown failure fails the run, `--timeout` on every guest exec, safe `clean`); preflight's assumptions are the soft spot. |
| Maintainability    | B | Functions are short and commented with *why*; the harness is outside the linter, and case-name-count coupling to `features.json` is the smell the Major exposes. |
| Handoff-readiness  | B | Runbook gives preconditions, trigger, result table and cleanup, and is linked from the release runbook; the archived evidence needs reconciling with the current scripts. |

### Verdict
**FAIL** — one REAL Major in the contract set: `features.json` f4's recorded verification cannot pass
on the reviewed tree, so AC4's proof is not reproducible from the artefacts being archived. Rubric
below-bar is Verification (C), no D. Everything else the review found is Minor and tracked.

`dotf spec archive` is **not advisable** in this state: it would record an acceptance proof that
fails when run.

### Recommended next steps

Contract set (must be fixed before a re-review; the re-review is the mechanism):

1. `features.json` f4 — replace the hard-coded drift counts with assertions that cannot drift:
   grep the specific case names (`PASS drift: removed ufw rule seen by check`, …) or assert one
   pair per drift introduced by name. If a count stays, it must be the count on the reviewed tree
   (Linux 4, Windows 12) and the reason must be in `verification.md`. After the edit, replay the
   f4 command against the most recent green logs.
2. `features.json` f3 — make the criterion fallible: fail when `dotf vm <os> status` itself fails
   (e.g. `out=$(dotf vm linux status) || exit 1` then `! grep -q '^tsb-e2e-' <<<"$out"`), so a
   broken `dotf` cannot read as "no session left".

Note that (1) and (2) are contract-set edits: they invalidate this review, so `dotf spec review
QA-606-e2e-vm-harness` must run again after them. Applying them is cheaper than the round costs.

Outside the contract set (apply in the same PR, or ticket, and record the disposition in
`verification.md`):

3. `verification.md` — restate AC1/AC2/AC4 evidence against the reviewed content (the 77-pass Linux
   and 50-pass Windows runs on these exact file contents) and say the logs are git-ignored by design,
   rather than leaving 37/39 as if they described `HEAD`.
4. `scripts/check-shellcheck.sh` — add `scripts/e2e/*.sh` to the covered globs (and a
   `test-shellcheck.sh` fixture for the third depth), then fix the two live warnings in `run.sh`
   (unused `i`; declare `dir` separately from `hs_host_ip`).
5. Ticket for the owner (`.github/**` is owner-only per the proposal): add
   `bash scripts/tests/test-e2e-report.sh` (and the cheap `test-e2e-probe.sh`,
   `test-e2e-mesh-preflight.sh`, `test-e2e-headscale.sh`) as `hygiene` steps, so AC5's guard cannot
   rot.
6. Add a shim-level case for `run.sh linux` mirroring `test-e2e-mesh-preflight.sh`'s interrupt case,
   so AC3's teardown is proven for the path `make e2e` actually takes.
7. `preflight()` — gate on the verbs the run uses (`exec --script`, `cp`) and ask `dotf` for the
   Windows base path instead of hard-coding `$HOME/.local/state/dotfiles/…`.

**Minimum to flip to PASS**: items 1 and 2 (contract set) plus a fresh passing
`dotf spec review QA-606-e2e-vm-harness`; with those, Verification can move off C and the remaining
items stand as tracked minors.

### UNVERIFIED in this session

- AC1/AC2/AC4 live behaviour: no `dotf vm` guest was booted and the cited logs are absent from the
  checkout. The scenario scripts were reviewed line by line, syntax-checked (`bash -n`, `sh -n`,
  `py_compile`), and their verdict machinery was exercised by fixtures and mutations, but "the
  scenarios pass on a real guest" is taken from recorded evidence, not re-run here.
- The Windows scenario as executed by PowerShell on a real Windows 11 Arm64 guest: only its case
  structure was executed (under `pwsh`), not its OS reads and writes.
- `dotf` session-verb semantics (`status`/`up`/`down`/`cp`/`exec --script`) and the `dotf >= 0.79`
  claim; the headscale/docker mesh path (QA-607's scope).
