---
tags: [spec, verification, templates]
created: "2026-11-09"
id: "BUG-526-tailscaled-firewall-modes"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-526-tailscaled-firewall-modes

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] AC1 (nodivert `unverified`, no write) -> a264c19 / `TestReadIptablesCapturedModes/netfilter-mode_nodivert`, built on a live capture (`capture-tsfw.sh`, eb43f7d).
- [x] AC2 (nftables mode on nf_tables reads as iptables mode) -> a264c19 / `TestReadIptablesCapturedModes/nftables_mode,_read_through_iptables-nft`, built on a live capture.
- [x] AC3 (unscoped v6 allow `manual` in check and setup, `ip6tables -D` fix, `firewall_ok` false, nothing deleted) -> edd29a3 / `TestIP6Exposure/an_unscoped_v6_allow`. Mutants, re-run after the master merge 6bc1fba: dropping `len(fw.v6Foreign) == 0` from `linuxFirewallOK`, or `withIP6` from the setup or the check step, each fail `TestIP6Exposure`.
- [x] AC4 (failed v6 read noted, no `ip6tables` unchanged) -> edd29a3 / `TestIP6Exposure/v6_read_fails`, `/no_ip6tables`.
- [x] AC5 (e2e) -> 65994ad / `make e2e-linux` at cf2366c: `linux-20261111T071101Z.log`, 89 pass, 0 fail, 0 xfail, 0 xpass. It includes `PASS unscoped v6 allow: check says manual, naming the ip6tables fix`, `... check does not report firewall_ok`, `... setup reports it and leaves it` and `PASS v6 allow removed by the operator: check is ok again`.

## Test status

- Test suite: `go test -race ./...` -> all packages ok at cf2366c. `golangci-lint run ./...` and `GOOS=linux` / `GOOS=windows` on `internal/host` -> 0 issues. `gosec ./...` -> no findings.
- Real VM: `make e2e-linux` -> 89 pass, 0 fail (above).
- No regressions: yes. The merge of #704 (SEC-631 off-tailnet allows) was resolved by keeping both terms in `linuxFirewallOK`, and `TestOffTailnetOnlyAllow` and `TestIP6Exposure` both pass.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- The v6 read is an exposure scan, not a second reachability decision (owner fork, 2026-11-10). An unscoped v6 allow is reported `manual` and never deleted, because ts-bridge does not own v6 rules.
- No Linux host runbook exists, so the operator-facing note went to `docs/design/host-client-parity.md`, the per-OS host state. That edit also corrected its nftables-mode line.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-055-2026-11-04.md (corrected with the live capture, b04a956)
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: the v6 exposure scan follows ADR-017 Amendment B, where an allow setup did not write is reported `manual` and left to the operator
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: specific to tailscaled's Linux firewall modes

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-526-tailscaled-firewall-modes/` -> `specs/archive/BUG-526-tailscaled-firewall-modes/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
