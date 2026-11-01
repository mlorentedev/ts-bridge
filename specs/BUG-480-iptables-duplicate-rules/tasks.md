---
tags: [spec, tasks]
created: "2026-11-01"
---

# Tasks - BUG-480-iptables-duplicate-rules

> TDD order. One task = one focused commit.

## Setup

- [x] Branch created from master: `fix/issue-480-iptables-duplicate-rules`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [AC1] [AC2] [AC3] [AC6] Failing `TestIptablesSetupCopies` table on the counting helper (zero, one, two, scoped plus unscoped, other port, copies behind the ts-input jump)
- [x] [AC1] [AC2] [AC3] Count the copies in `readIptables` and carry them in `firewallRead`
- [x] [AC1] [AC4] Failing `TestCheckForLinux` rows through `checkFor`: two copies (manual, `firewall_ok` true, fix names `-D` and the count), two copies with `tsInputDefault`, a DROP before two copies (blocked wins)
- [x] [AC1] [AC4] `firewallCheckStep` reports copies when the read is present
- [x] [AC5] `TestSetupForLinux` row: two copies, setup `ok`, no `iptables -D` in the call list
- [x] Mutation checks recorded in `verification.md`, each re-run through the seam

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command
- [x] `go vet` passes on linux, windows and darwin
- [x] Lint passes
- [x] No unrelated changes in the diff
- [x] `verification.md` filled in
- [x] PR opened referencing this spec folder (#534)
