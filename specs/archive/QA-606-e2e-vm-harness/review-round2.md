---
spec: "QA-606-e2e-vm-harness"
verdict: "PASS-WITH-GAPS"
reviewed_sha: "5c0bee8ae0dd85e716b791e9ad73fb907740cbdf"
reviewer: "nan/deepseek-v4-flash"
date: "2026-11-10"
---

## Adversarial review

**Scope**: `QA-606-e2e-vm-harness` — `git diff 82b1c6918c3b101a2bd58e048f17bb1b5829ff8b...HEAD`
(198 files). The QA-606 slice is 22 files: `scripts/e2e/{run.sh,linux.sh,windows.ps1}` (1047 lines),
`scripts/check-shellcheck.sh` + `scripts/tests/test-shellcheck.sh` (the `scripts/e2e/*.sh` glob),
`scripts/tests/test-e2e-report.sh`, the five `make e2e*` targets, `docs/runbooks/e2e-vm.md` and its
link, the `.gitignore` entry, the `AGENTS.md` row and `specs/QA-606-e2e-vm-harness/`. The remaining
~176 files are sibling specs' work that arrived on this branch (the range holds the branch's own 40
commits up to `bbd8685`, plus a merge of `origin/master` at `a357428`, plus three later docs/lint
commits); they are judged only where they touch the harness — `headscale.sh`/`mesh_linux.sh` reach
`run.sh`'s `clean` and `mesh` verbs, `windows.ps1`/`linux.sh` were edited by five master commits
(#660, #661, #663, #681, #689), and `release.yml` decides the asset names the upgrade case downloads.
**Sources**: `specs/QA-606-e2e-vm-harness/{proposal,tasks,verification}.md`, `features.json`,
`review-request.json`, `review-request-round1.json`; the scripts above; `Makefile`;
`docs/runbooks/{e2e-vm,guide-development-workflow}.md`; `.github/workflows/release.yml`;
`.gitignore`; `AGENTS.md`.

### Spec and task alignment

- **Contract digests are current.** `dotf spec digest QA-606-e2e-vm-harness` returns
  `features.json 38268cc4…`, `proposal.md 5a57dc14…`, `tasks.md a6327121…` — byte-identical to
  `review-request.json`'s `contract_digests`, and `reviewed_sha` equals `git rev-parse HEAD`. The
  round-2 review is of the state the launcher asked for.
- **Every ticked task has a diff artefact.** `[AC5]` → `report()` + `test-e2e-report.sh`;
  `[AC3]` → `preflight`/`build`/`down`/EXIT trap/`clean`; `[AC1][AC4]` → `linux.sh` (77 cases);
  `[AC2][AC4]` → `windows.ps1` (42 static cases + 4 drift keys × 2 = 50 runtime lines);
  Makefile targets, `scripts/e2e/out/` ignored (`.gitignore:76`), `docs/runbooks/e2e-vm.md` linked
  at item 5 of `guide-development-workflow.md:94`. No `[AGENT-DRAFT]` / `[AGENT-SUGGESTION]` tags.
- **AC5 verified by running, red-green.** `bash scripts/tests/test-e2e-report.sh` → 9/9 ok, exit 0.
  Mutating `report()` (dropping the `[ "$xpass" -eq 0 ]` clause) in a `mktemp` copy of the tree turns
  `FAIL one-xpass: want fail, got pass` and exit 1, while the unmutated control prints
  `all cases passed`. The fixture's `clean` and `xfail-only` controls stop a report that fails
  everything.
- **AC1/AC2/AC4 are evidenced, and the evidence is traceable to this tree.** The cited logs exist
  (`scripts/e2e/out/linux-20261110T060009Z.log`, `windows-20261110T060033Z.log`, both git-ignored),
  and their contents confirm the recorded 77/50 pass, 0 fail, 0 xfail, 0 xpass. Beyond the counts,
  I mapped the case names: the Linux log's 77 result lines match `linux.sh`'s 77 `ok "…"` calls with
  an empty `diff`; the Windows log's 50 lines are the 42 static `Case '…'` declarations plus the
  `$drifts` loop's 4 keys × 2 (after CRLF normalisation, empty `diff`). So the recorded runs describe
  the scripts at `HEAD`, not an older revision — which is what round 1 could not establish.
- **AC4 is verified by replay, not by reading.** `features.json` f4's `verification` command, run
  verbatim, exits 0 against both newest logs, and it no longer pins a line count.
- **AC3's post-run and `clean` halves hold; its interrupt half is untested** (finding R1).
- **AC6 verified by reading**: preconditions, trigger (before a breaking/host-facing Release PR),
  the four result lines, and cleanup are all present, and the release runbook links it.
- **Round 1's dispositions are recorded, not verbal.** Each is applied, declined with a reason, or
  ticketed: `gh issue view 702` → **OPEN**, "test(e2e): wire the e2e unit suites into CI and cover the
  primary teardown", created 2026-11-10T06:00:22Z — so the teardown gap and the CI wiring have a
  durable home. `scripts/check-shellcheck.sh` now globs `scripts/e2e/*.sh` and reports
  `OK (30 scripts)`; `test-shellcheck.sh` pins the new depth and the fail-closed empty scope.
  The two declined findings (preflight depth, `report()` case floor) state defensible reasons.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | REAL | verification (evidence provenance) | The log the runbook calls "the full log" cannot identify the build it describes. `run.sh:205` echoes `candidate <sha>, previous <tag>` to stdout **before** the `tee` pipeline starts, so neither the commit nor the previous release reaches `scripts/e2e/out/<os>-<ts>.log`; every log begins with the first case. `features.json` f1/f2 cite "rerun at bbd8685" and `verification.md` repeats it, but the artifact itself carries no sha — an archived spec's evidence inherits a claim only the operator's memory backs. | `head -5` of both cited logs → first line is `PASS version prints`; `grep -c 'candidate\|previous'` → 0 in both. Mitigation I applied: case-name set equals `HEAD`'s exactly (77/77, 50/50), so the logs *are* from this tree — but that check needs the checkout, not the log. | UNTESTED (no test asserts the log identifies its candidate) | code — `scripts/e2e/run.sh` (send the header through `tee` too, one line) + `verification.md` noting it |
| Minor | REAL | tests (AC3) | "Every run deletes a leftover session first" (`run.sh:197`, `down "$OS"` before `up`) and "whether it passes, fails or **is interrupted**" (the EXIT trap) have no test. `verification.md`'s AC3 box proves the *post*-run state, `make e2e-clean` twice, and one failed run — not the pre-run cleanup, not a SIGINT, and not "a second run gives the same result". The only interrupt test in the tree (`test-e2e-mesh-preflight.sh`'s case) covers `main_mesh`'s trap, not `main`'s. | code read of `run.sh:193-212`; `grep -rn 'interrupt\|SIGINT' scripts/tests/` finds only the mesh case; the two recorded runs are one per OS, not repeated | UNTESTED for `main()` — `test-e2e-mesh-preflight.sh:interrupt-tore-down-the-guest` covers the mesh path only | tests — tracked by **#702** (OPEN); reuse the mesh suite's shim technique |
| Minor | THEORETICAL | preflight | `preflight()` gates on `dotf vm "$OS" up --help` only (not `exec --script` / `cp`) and hardcodes `$HOME/.local/state/dotfiles/vm/windows/clean-base.qcow2` (`run.sh:42`), duplicating dotf's state layout. The declined rationale in round 1 is sound for the *false-pass* direction (a missing verb fails at first use with dotf's own message, so no run reads green), but the *false-refusal* direction survives: a dotf that honours `XDG_STATE_HOME` has its base image elsewhere and the harness stops with "run 'dotf vm windows build' once" on a machine where it already was. | code read; the proposal's risk note claims a stronger gate ("checks both and stops with the fix") than the code performs | UNTESTED | code — `run.sh:preflight` (ask dotf for the path; it prints one) |
| Minor | SPECULATIVE | portability | The previous-release path needs `unzip`, GNU/BSD `tar` and `shasum -a 256 -c`. None is preflighted, and the failure is funnelled into `die "could not verify $asset"` (`run.sh:56-57`), which names the wrong cause. The runbook's prerequisite list says only `dotf`, `gh`, Go and an Arm64 host, while its own troubleshooting section contemplates a non-Apple-Silicon host failing with `exec format error`. | code read of `run.sh:52-62` against the runbook's Prerequisites; not reproduced (no Linux host here) | UNTESTED | code — `run.sh:preflight` (add the three tools) + `verification.md` |
| Question / assumption | — | operator ergonomics | `make e2e` is two recipe lines, so a Linux failure stops the run before Windows. That is defensible (and probably wanted), but the runbook says "both, one after the other", which reads as "Windows runs regardless". Confirm the intended semantics and say so in the runbook row. | `Makefile:82-84`; `docs/runbooks/e2e-vm.md` Run section | n/a (documentation) | spec — `verification.md` disposition, or a runbook wording fix |

**What I tried to break and could not** (recorded because each refuted a plausible Blocker):

- **Upgrade path asset names.** `run.sh` builds `ts-bridge-$tag-linux-arm64.tar.gz` /
  `…-windows-arm64.zip` from `gh release view --json tagName`, greps the checksum line for exactly
  that asset, and copies out of `ts-bridge-linux-arm64/` / `ts-bridge-windows-arm64/`.
  `release.yml:70-86` stages exactly those directory names and names the archives
  `ts-bridge-${VERSION}-${OS}-${ARCH}` from `needs.release-please.outputs.tag_name`, and emits
  `checksums.txt` with `sha256sum -- *` (two-space form, which `shasum -c` accepts). The upgrade case
  is not relying on a guessed layout.
- **`report()` cannot be fooled by noise.** `test-e2e-report.sh`'s `noise-only` and
  `fail-word-mid` fixtures prove the `^(PASS|FAIL|…) ` anchoring; `empty`, `no-result` and
  `RC=1 aborted` prove a guest that dies early fails the run; the `tee` guard is real because
  `set -o pipefail` makes a failed log write abort rather than pass.
- **A log write failure, a failed teardown and a missing `dotf` all fail closed.** `run.sh:202`
  re-raises a failed `down` (`[ "$rc" != 0 ] || rc=1`), and f3's `out=$(dotf vm $os status) || exit 1`
  turns the round-1 fail-open into a failure (confirmed by reading the recorded command).
- **`make e2e-clean` on a docker-less machine.** `headscale.sh:78-86` guards `hs_down` with
  `command -v docker`, so AC3's cleanup does not depend on QA-607's container being installed.
- **A dropped `--` in `linux.sh`'s `ok` helper.** I suspected a silent-PASS footgun (the helper
  shifts unconditionally), so I ran the real `result`/`ok` block from `linux.sh` in a scratch
  directory: `ok "x" 'false'` reports `XPASS #false x`, which `report()` fails — the misuse is
  visible, not silent. All 77 call sites carry `--` anyway.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | AC1/2/4/5 verified (logs, replay, red-green); AC3's interrupt and pre-run-cleanup halves are untested, and no criterion is observably broken. |
| Verification       | B | Every criterion has a runnable command and I replayed f4 plus both test suites; the cited logs omit the candidate sha, so the evidence identifies its build only through the case-name diff I had to perform. |
| Scope              | B | The QA-606 slice matches the proposal; the range carries ~176 sibling files (plus `run.sh`'s mesh verbs from QA-607) that the proposal does not name — documented in the branch history, judged only where it touches the harness. |
| Reliability        | B | Fails closed on an empty/aborted scenario, a failed teardown, a failed log write and a missing `dotf`; the interrupt path rests on code reading rather than a test. |
| Maintainability    | B | Small functions, comments carry the *why* (`run.sh:21`, `linux.sh:14-35`, `windows.ps1:126-135`), shellcheck now covers `scripts/e2e/*.sh`; the verdict primitive's argument contract (`NAME [ISSUE] -- EXPR`) is a sharp edge documented only in a comment. |
| Handoff-readiness  | B | Spec artifacts complete, round-1 findings dispositioned in `verification.md`, #702 open for the tracked gaps, runbook linked; the archived log's missing provenance is the one thing a future reader cannot recover. |

### Verdict

**PASS WITH GAPS** — minors only, no Blocker and no REAL Major; rubric has no C and no D.
Nothing here blocks `dotf spec archive`.

**UNVERIFIED**: the archive gate itself. `dotf spec archive` offers no dry run, and running it
would move the folder, so whether it accepts `verdict: PASS-WITH-GAPS` as a passing verdict rests
on its documented contract (`--help`: "refuses without a fresh, passing review.md") and on the
adversarial-review skill's enum, not on my execution of it. The contract digests and `reviewed_sha`
are current, which is the freshness half of that check and the half I could observe
(`dotf spec digest` matches `review-request.json` exactly).

### Recommended next steps

Contract set (`proposal.md`, `tasks.md`, `features.json`) is **closed**: this verdict permits the
archive, and editing any of them now would invalidate it. The items below belong in
`verification.md` (free to edit, excluded from the staleness check) or in a follow-up ticket.

- **R1 (Minor, REAL)** — record in `verification.md` that the cited logs carry no candidate sha, and
  that the link to `bbd8685` was re-established by matching case sets (77/77, 50/50) against `HEAD`.
  Optional one-line code fix next time the harness runs: `echo … | tee -a "$log"` for the header
  line, which needs a fresh VM run to demonstrate — not something this review can produce.
- **R2 (Minor, REAL)** — already tracked by **#702**; no new ticket needed. Disposition it in
  `verification.md` as covered there, naming the pre-run `down` and the SIGINT path explicitly so
  #702's scope is not read as teardown-only.
- **R3 (Minor, THEORETICAL)** — accept as debt in `verification.md`, or fold the `XDG_STATE_HOME`
  half into #702 if the owner wants the false refusal gone; the false-pass argument for declining is
  correct and need not be revisited.
- **R4 (Minor, SPECULATIVE)** — surface only: either add `unzip`/`tar`/`shasum` to `preflight()` and
  the runbook's prerequisites, or leave it and note in the runbook that a Linux host will fail at the
  first download with a misleading message. Do not gate on it.
- **Q1 (Question)** — confirm in the runbook whether `make e2e` is meant to continue to Windows after
  a Linux failure. The current behaviour (stop) is the safer one; the wording should say which.
- **Process note for the archive**: `tasks.md`'s "shellcheck passes" is now true of the change itself
  (`OK (30 scripts)`, and `test-shellcheck.sh` proves the glob cannot silently empty), and
  `verification.md`'s "Not run in CI by design" is consistent with the proposal's non-goals. Both are
  sound; no action.
