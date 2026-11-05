---
tags: [spec, tasks, templates]
created: "2026-11-04"
id: "QA-606-e2e-vm-harness"
type: spec-tasks
status: active
owner: manu
---

# Tasks - QA-606-e2e-vm-harness

## Setup

- [x] Branch created from master: `test/issue-606-e2e-vm-harness`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [AC5] `scripts/tests/test-e2e-report.sh`: fixture cases for the verdict (XFAIL passes, FAIL and XPASS fail), then `report()` in `scripts/e2e/run.sh`
- [x] [AC3] `run.sh`: preflight, build candidate + verified previous release, delete leftover session, EXIT trap deletes the session, `clean`
- [x] [AC1] [AC4] `scripts/e2e/linux.sh`: init, setup twice, drift (ufw, xrdp), refusals, upgrade from `.env`, serve preflight, client
- [x] [AC2] [AC4] `scripts/e2e/windows.ps1`: same on Windows with the real PowerShell reads and writes (runs after #611 lands)
- [x] [AC1] [AC2] [AC3] Makefile targets `e2e`, `e2e-linux`, `e2e-windows`, `e2e-clean`; `scripts/e2e/out/` git-ignored
- [x] [AC6] `docs/runbooks/e2e-vm.md`, linked from the release section of `guide-development-workflow.md`

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by a case or a fixture test
- [x] Every acceptance criterion has a matching entry in `features.json`
- [x] shellcheck passes (`scripts/check-shellcheck.sh`)
- [x] `verification.md` filled in with the run summaries
- [x] PR opened referencing this spec folder
