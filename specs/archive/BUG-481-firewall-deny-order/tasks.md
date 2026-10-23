---
tags: [spec, tasks]
created: "2026-10-22"
---

# Tasks - BUG-481-firewall-deny-order

## Setup

- [x] Branch `fix/issue-481-firewall-deny-order`, stacked on `fix/issue-483-iptables-portless-allow` (#493)
- [x] `proposal.md` complete; option A (unscoped deny only, reported `manual`) agreed on the issue

## Implementation

- [x] [AC1] [AC2] [AC3] [AC4] Failing `TestFirewallDecideOrder`: one table, both backends, verbatim syntax
- [x] [AC1] [AC3] Failing `TestIptablesDecideOnly`: blanket and `tcp-reset` denies decide; conntrack, negated-scope, loopback and jump rules do not
- [x] [AC1] Failing setup/check cases: a deny first means `manual` and no command
- [x] [AC1] [AC3] `iptablesDecide` and `ufwDecide` walk rules in order; `iptablesCovers`/`ufwAllows` become wrappers
- [x] [AC1] `ruleBlocked` state and `firewallBlockedStep` (`manual`, names the rule) in setup and check

## Closing

- [x] Every acceptance criterion covered by a named test
- [x] `features.json` entries with executable verification
- [x] `go vet` on linux/windows/darwin, `golangci-lint run` clean
- [x] End-to-end run in `ubuntu:24.04`
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder
