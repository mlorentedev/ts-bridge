---
tags: [spec, verification, templates]
created: "2026-10-22"
---

# Verification - BUG-483-iptables-portless-allow

## Evidence

- [x] AC1 -> `TestIptablesCovers` (`port-less blanket`, `interface`, `source`, `tcp`, `negated loopback interface`, `negated loopback source`, `tcp flags` (the `--syn` form), `comment`); `TestSetupForLinux/iptables, port-less tailnet allow` (`ok`, no `run`); `TestCheckForLinux/iptables, port-less tailnet allow` (`ok`, `firewall_ok`)
- [x] AC2 -> `TestIptablesCovers`:
  - loopback: `port-less loopback interface`, `loopback source`, `loopback destination`, `ipv6 loopback source`;
  - protocol: `udp`, `icmp`, `negated tcp`;
  - other matches: `state`, `limit`, `recent`, `drop`, and the existing conntrack case `no destination port`;
  - `fragment`;
  - TCP flags: `ack-only tcp flags`, `syn+ack tcp flags`, `other tcp flags mask`;
  - negations: `negated tailnet source`, `negated tailnet interface`.

  `TestSetupForLinux/iptables, only loopback and established allowed` and `/iptables, only an established-traffic tcp-flags allow` run one `-A` each and report `changed`. `TestCheckForLinux/iptables, only loopback and established allowed` reports `changed`.
- [x] AC3 -> every pre-existing `TestIptablesCovers` case is unchanged and passes. `loopback interface with the port` and `loopback source with the port` pin the new port-bearing loopback answer.

## Test status

- Red-green: the round-1 cases (8 `TestIptablesCovers`, 1 `TestSetupForLinux`) fail against the round-1 head `7325180` and pass now. The original setup/check cases fail against `master` and pass.
- `go test -race -count=1 ./...` -> all packages ok
- `GOOS=linux|windows|darwin go vet ./...` -> clean; `golangci-lint run` -> 0 issues
- Every `-S` line in the new cases was printed by iptables 1.8.10 (nf_tables) in `ubuntu:24.04`.
- End to end on the round-1 head: `ubuntu:24.04` with `NET_ADMIN`, a `GOOS=linux` binary, and no ufw. INPUT held a `ts-input` jump and `-i lo -j ACCEPT`. Setup was run by hand:
  - with `-i tailscale0 -j ACCEPT` added, two `host setup` runs both reported "Firewall allows TCP 3389 (iptables)", and `iptables -S INPUT` held no `--dport 3389` rule;
  - with the tailnet allow removed, the first run reported "Firewall rule added", the second "Firewall allows", and `-S INPUT` held exactly one `--dport 3389` rule.

  Round 2 only narrows what counts, so neither of these two inputs changes its answer.

## Review round 1 (nan/deepseek-v4-flash, FAIL at 7325180)

- Major, REAL: `--tcp-flags` sets other than `--syn` (ACK-only, `SYN,ACK`) read as covering. Applied. The port-less branch now accepts tokens from an allow-list, and `--tcp-flags` counts only as the exact `--syn` pair. Covered by three cases and a setup case.
- Major, REAL: `-f` and other non-module narrowing were ignored. Applied by the same allow-list: any unmodeled token reads as absent. Covered by `port-less fragment`.
- Major, REAL: a non-loopback negation (`! -s 100.64.0.0/10`, `! -i tailscale0`) was discarded. Applied: only a negated loopback scope is accepted. Covered by two cases. AC2 now lists these shapes.
- Minor, THEORETICAL: the loopback guard covered only port-less rules. Applied. `iptablesCovers` now rejects loopback rules before the port split, and two cases pin it. AC3 records the change.
- Minor, SPECULATIVE: `iptFlag` conflates absent and negated. Applied for this branch: the port-less branch no longer calls `iptFlag` and reads negation explicitly. `iptFlag` keeps its contract for the port-bearing branch, where "negated" correctly means "not this port".
- Minor, THEORETICAL: the container run is prose with no committed script. Declined. The repository has no container harness for `internal/host`, and adding one is its own change. Every `-S` line the unit tests use was printed by iptables in the container, so the parser's inputs are reproducible from the tables. The end-to-end steps are recorded above.

## Review round 2 (nan/deepseek-v4-flash, PASS)

- Minor, THEORETICAL (a bare loopback literal such as `-s 127.0.0.1` is not recognized): declined. iptables 1.8.10 always prints a prefix length (`127.0.0.1/32`, `::1/128`), as measured in round 1 and again by the reviewer, so `-S` never produces that input. A code change now would also make the PASS stale for a shape that cannot be reached.
- Minor, SPECULATIVE (a non-loopback `-d` or `! -s` counts): no change. This is the declared "operator's scope" judgment, listed in AC1 and §Risks.
- Minor, REAL (the end-to-end run has no committed script): declined, for the reason given in round 1.
- Minor, REAL (no promotion-candidates section): applied below. `features.json` states stay `pending` because only the harness may write `passing`.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-046-2026-10-22.md
- [x] ADR-worthy decision for the repo's `docs/adr/`? no: the ADR-017 Linux row already says setup appends only when no allow covers the port. This change widens what "covers" reads, not the contract.
- [x] New pattern candidate for `00_meta/patterns/`? no: The cross-project idea (prefer an allow-list over a deny-list when a false positive is silent) is already the fail-closed principle.
