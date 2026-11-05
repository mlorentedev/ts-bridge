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
