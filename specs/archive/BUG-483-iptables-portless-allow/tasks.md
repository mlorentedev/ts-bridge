---
tags: [spec, tasks, templates]
created: "2026-10-22"
---

# Tasks - BUG-483-iptables-portless-allow

> TDD order. One task = one focused commit.

## Setup

- [x] Branch created from master: `fix/issue-483-iptables-portless-allow`
- [x] `proposal.md` is complete and its acceptance criteria are testable
- [x] Open questions measured in `ubuntu:24.04` (iptables 1.8.10, nf_tables)

## Implementation

- [x] [AC1] [AC2] [AC3] Failing `TestIptablesCovers` cases built from the measured `-S` lines: blanket, interface, source, `! -i lo`, `! -s 127.0.0.0/8`, `-p tcp`, `--tcp-flags` and comment cover; `-i lo`, `-s 127.0.0.0/8`, `-d 127.0.0.1/32`, `-s ::1/128`, udp, icmp, `! -p tcp`, conntrack, state, limit and recent do not
- [x] [AC1] [AC2] Implement the port-less branch of `iptablesCovers`
- [x] [AC1] `TestSetupForLinux` and `TestCheckForLinux` cases: an `-i tailscale0 -j ACCEPT` host is `ok`, and setup runs no `-A`
- [x] [AC2] `TestSetupForLinux` case: an `-i lo -j ACCEPT` host still gets one `-A`
- [x] Container run: `host setup` twice on a host with `-i tailscale0 -j ACCEPT`, then on one with only `-i lo -j ACCEPT`
- [x] Lesson and parity doc updated
- [x] Review round 1 (FAIL): port-less branch rebuilt as a token allow-list (`--syn` only, `-f`, non-loopback negations absent), loopback guard hoisted above the port split, regression cases added

## Closing

- [x] Every acceptance criterion is covered by at least one test
- [x] `go vet` for linux, windows and darwin
- [x] `golangci-lint run` passes
- [x] No unrelated changes in the diff
- [x] `verification.md` filled in
- [x] PR opened referencing this spec folder
