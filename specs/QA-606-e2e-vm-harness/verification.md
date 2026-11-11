---
tags: [spec, verification]
created: "2026-11-04"
id: "QA-606-e2e-vm-harness"
type: spec-verification
status: active
owner: manu
---

# Verification - QA-606-e2e-vm-harness

## Evidence

- [x] AC1: `make e2e-linux` on 03e077c (the harness merged with master 82b1c69), Ubuntu 24.04 arm64: 37 pass, 0 fail, 2 xfail (#608, #557), 0 xpass, exit 0. Log: `scripts/e2e/out/linux-20261105T224406Z.log`. (The unmarked `ufw inactive` case now requires the `Firewall configuration` step, so a dropped step fails instead of reading as the #608 XFAIL.) An earlier run on a base without #612 reported `FAIL serve: auth_key_file under /tmp refused` and exited 2, so the gate does fail a run.
- [x] AC2: `make e2e-windows` on candidate 03e077c (the drift negatives now require the service in the check document), Windows 11 Pro Arm64: 39 pass, 0 fail, 1 xfail (#557), 0 xpass, exit 0. Log: `scripts/e2e/out/windows-20261105T224649Z.log`. This covers the real PowerShell reads and writes: registry, TermService, firewall rule, network profile and powercfg.
- [x] AC3: after every run, `dotf vm linux status` and `dotf vm windows status` list no `tsb-e2e-*` session. Running `make e2e-clean` twice prints "no session left" both times. The EXIT trap also deleted the guest when a run failed.
- [x] AC4: both logs have `PASS setup #2: nothing changed`. Every drift case is detected by `check` and repaired by `setup`. On Linux the cases are a removed ufw rule and a stopped or disabled xrdp. On Windows they are a rule switched to UDP, a disabled rule, a deleted rule, RDP denied, and power reset to 30 min.
- [x] AC5: `bash scripts/tests/test-e2e-report.sh` exits 0, with fixtures where XFAIL passes and FAIL or XPASS fails. In practice, once #595, #605 and #613 landed, their XFAIL markers reported XPASS and failed the run until they were removed (commits ef2406a and 967ee8e).
- [x] AC6: `docs/runbooks/e2e-vm.md`, linked from item 5 of the release section in `guide-development-workflow.md`.

## Test status

- `bash scripts/check-shellcheck.sh`: OK (21 scripts).
- `bash scripts/tests/test-e2e-report.sh`: exit 0.
- Not run in CI by design: GitHub runners cannot nest a VM (proposal, Non-goals).

## Review disposition (round 1, FAIL)

`review.md` (nan/deepseek-v4-flash) returned FAIL on one REAL Major. Each finding:

- f4's verification asserted 9 Windows drift lines; the script prints 12 (Major, REAL): applied. f4 now
  requires `PASS setup #2: nothing changed`, at least one `PASS drift` and no `FAIL drift` in the
  newest log of each OS, so it no longer pins a count that every new drift case breaks. Replayed at
  bbd8685 against the two runs below: exit 0.
- Evidence cites 37/39 pass at 03e077c (Minor, REAL): applied. Both runs are repeated at the
  reviewed scripts' head bbd8685: `make e2e-linux` 77 pass, 0 fail, 0 xfail, 0 xpass, exit 0
  (`linux-20261110T060009Z.log`); `make e2e-windows` 50 pass, 0 fail, 0 xfail, 0 xpass, exit 0
  (`windows-20261110T060033Z.log`). The XFAILs of the first runs are gone because #608 and #557
  were fixed since. AC1/AC2 above keep the first run; f1/f2 carry both.
- `scripts/e2e/*.sh` outside shellcheck (Minor, REAL): applied. `scripts/check-shellcheck.sh` now
  globs `scripts/e2e/*.sh` (red on the two `run.sh` warnings first, then fixed: the unused loop
  variable and the `local` that masked `hs_host_ip`'s status). `scripts/tests/test-shellcheck.sh`
  pins the new glob and an empty `scripts/e2e` failing closed. OK (30 scripts).
- The `test-e2e-*.sh` suites run in no workflow (Minor, REAL): ticketed as #702. The wiring is in
  `.github/`, which only the owner changes.
- The primary path's teardown trap has no test (Minor, REAL): ticketed as #702, with the CI wiring.
- Preflight probes only `up --help` and hardcodes dotf's state path (Minor, THEORETICAL): declined. A
  missing `exec`/`cp` fails the run on its first use, with dotf's own message, and the state path is
  only a hint for a missing clean base: a wrong guess costs a clearer error, never a false pass.
- f3 is fail-open without `dotf` (Minor, REAL): applied. f3 now captures `dotf vm <os> status` and
  exits 1 when the command fails, before it greps: with no `dotf` the status read exits 127 and f3
  fails. Replayed at bbd8685 with `dotf` present: exit 0, and `dotf vm linux|windows status` list no `tsb-e2e-` session.
- `report()` has no case-count floor (Minor, SPECULATIVE): declined. A floor would be a second
  count to keep in step with the scripts, the drift f4 just removed; the summary line prints the
  totals, and a lost block shows in the diff of the scenario script that lost it.

## Review disposition (round 3, PASS-WITH-GAPS)

Reviewer `nan/mimo-v2.6-flash`, at ea0781a (`review.md`; round 2 kept as `review-round2.md`). This round re-read the contract set after the f4 replay edit, so its digests match this branch.

- **Minor REAL: the round-2 dispositions cite #705, which is not on this branch.** This is accurate: they are applied there, not here. #705 carries the log header, `test-e2e-run-teardown.sh` and the CI wiring, and it is open and green.
- **Minor REAL: AC3 has no test for `main()` at this sha.** Same as the line above: covered by `test-e2e-run-teardown.sh` in #705 (pass, fail, interrupt, failed teardown, leftover session).
- **Minor THEORETICAL (preflight) and Minor SPECULATIVE (portability).** Declined, as in rounds 1 and 2.

## Review disposition (round 2, PASS-WITH-GAPS)

Reviewer `nan/deepseek-v4-flash`, at a357428 (`review.md`). No Blocker or Major.

- **Minor REAL, evidence provenance.** The log does not name its candidate commit or previous release: `run.sh` echoes that header before the `tee`. Applied in #705: the header goes through `tee` into the log, and `test-e2e-run-teardown.sh` asserts that the log names the candidate.
- **Minor REAL, AC3 untested for `main()`.** Applied in #705 (closes #702): `test-e2e-run-teardown.sh` runs the real `run.sh linux` against shims. It covers pass, fail, interrupt, a failed teardown and the pre-run deletion of a leftover session.
- **Minor THEORETICAL, preflight checks only `vm up`.** Declined, as in round 1. A missing verb fails at first use with dotf's own message, so no run reads green.
- **Minor SPECULATIVE, `unzip`/`tar`/`shasum` not preflighted.** Declined. They ship with the only supported host (macOS on Apple Silicon, per the runbook). A missing one fails closed in `die "could not verify"`.

PR-Agent on #703 (head 5c0bee8):
- **f4 replay weakened (REAL).** Applied. The replay requires at least 4 `PASS drift` lines on Linux and 12 on Windows. Deleting one Linux drift line from the log makes it exit 1 (mutant), and the restored log exits 0.
- **Dead `|| exit 1` (THEORETICAL).** Applied on the same line, since it costs nothing: `[ -n "$l" ] || exit 1`.

## Decisions made during implementation

- `run.sh` keeps the temp dir in a global variable. The EXIT trap fires after `main` returns, and with a `local` the trap hit `dir: unbound variable` and left the guest running.
- The harness found three of its own case bugs, and each is fixed to test the product:
  - The `status` case asserted a non-zero exit, but the contract is exit 0 (`smoke.bats:258`).
  - The Windows `state_dir` case pointed at an absent directory, which `serve` correctly creates protected. The case now creates a loose directory first.
  - A weak #605 case is replaced by the disabled-xrdp pair.
- The scenario scripts are test code, so they fall outside the 300-line executable cap. The product code this PR touches is limited to the Makefile targets.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the one surprise, a `local` variable read by an EXIT trap, is a generic shell fact and is commented in `run.sh`
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: running the harness locally only, on breaking releases, is a process rule that lives in the runbook
- [x] New pattern candidate for `00_meta/patterns/`? no: one project so far

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/QA-606-e2e-vm-harness/` -> `specs/archive/QA-606-e2e-vm-harness/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
