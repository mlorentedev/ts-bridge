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

- [x] AC1 (nodivert `unverified`, no write) -> a264c19 / `TestReadIptablesCapturedModes/netfilter-mode_nodivert` (the read, built on a live capture from `capture-tsfw.sh`, eb43f7d) and `TestFirewallStepsTailscaledModes/nodivert` (no `run` call on setup; check `unverified`).
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

## Review disposition (round 1, PASS-WITH-GAPS)

`review.md` (nan/deepseek-v4-flash, reviewed `8d832f2`) returned PASS-WITH-GAPS. No contract file changed after it.

- **The v6 scan reuses the rule-shape test, so a negated scope or `::/0` is invisible** (Major, THEORETICAL): applied. `readIP6` now uses `ip6Exposed`. An allow counts as narrowed only with a plain `-i`, `-s` or `-d` whose value is not `::/0`; a negated one narrows nothing. Three new `TestIP6Exposure` rows (`! -i tailscale0`, `-s ::/0`, `-d ::/0`) failed before the change and pass after. A fourth row, a single LAN prefix, pins that an operator-scoped allow stays `ok`. The v4 read is unchanged: its off-tailnet handling came with #704.
- **AC3's "manual" is narrower than the code** (Minor, REAL): declined as a code change and recorded here. When the v4 step is already `unverified` or `failed`, `withIP6` keeps that outcome and appends the v6 rule to the message, with `firewall_ok` false. Overwriting it with `manual` would hide the v4 problem, which the operator has to fix first. So AC3's `manual` applies when v4 has converged; otherwise the v6 rule is still named.
- **A failed v6 read gives no reason** (Minor, REAL): applied. The note now quotes the tool's first stderr line (`toolSaid`). `TestReadIP6NoteQuotesTheTool` failed before the change and passes after.
- **`withoutIP6Probe` hides the probe's placement, and the check path's sequence is unasserted** (Minor, SPECULATIVE): applied in part. `TestIP6Exposure` now also asserts the check path's exact call sequence. The filter stays in the IPv4 tables, which pin the IPv4 sequence; the v6 probe's place is pinned here on both paths.
- **The AC1 citation is incomplete** (Minor, REAL): applied. AC1 now also cites `TestFirewallStepsTailscaledModes/nodivert`.

## PR-Agent disposition (#714, head `bb62c81`)

- *Vacuous assertion* (REAL): applied. `TestIP6Exposure` now fails a row with no expected v6 text if the message mentions `ip6tables` or IPv6. A mutant that returns a note when `ip6tables` is missing fails the `no ip6tables` row.
- *Contradictory doc claim* (REAL): applied. Lesson 055 now says the v6 `INPUT` chain is scanned for an off-tailnet allow and plays no part in reachability.
- *Hardcoded guest arch* (THEORETICAL): declined. The e2e harness is Apple Silicon only (QA-606 topology); `run.sh` builds the guest for arm64 too.

## PR-Agent disposition (#714, head `347b581`, partial run)

- *Vacuous verification* (REAL): applied. `f1` now replays `TestFirewallStepsTailscaledModes/nodivert` (setup writes nothing) beside the read test. This changes `features.json`, so the spec review runs again as round 2 before archive.
- *v6 scan skips jumped chains* (THEORETICAL): ticketed as #720. tailscaled's own v6 `ts-input` rules are interface-scoped; the gap needs a rule someone added inside a jumped chain.

## Review disposition (round 2, PASS-WITH-GAPS at the merged #714 head)

| Finding | Disposition |
|---|---|
| F1 Major THEORETICAL: a negated destination port (`! --dport 22`) is invisible to the v6 scan, and to the v4 unscoped read too | Ticketed #736. This predates the PR: the v4 read shares `iptPortCovers`. |
| F2 Minor REAL: `chainMissing`'s doc comment sits on `toolSaid` | Applied in the archive PR |
| F3 Minor REAL: lesson 055 says "off the tailnet"; the code reports unscoped allows only | Applied in the archive PR. The lesson now says "unscoped", and that an allow scoped to another interface or LAN prefix is not reported |
| F4 Minor REAL: `capture-tsfw.sh` has no shim test | Ticketed #737 |
| F5 Minor REAL: `f5` replays only where the gitignored e2e log exists | Recorded. The log is deliberately not committed (`scripts/e2e/out/` is gitignored); a reviewer reproduces it with `make e2e-linux` |
| F6 Minor THEORETICAL: the v6 scan ignores rule order | Applied as a comment beside `ip6Exposed`. Over-reporting a dead rule is the chosen direction, the same as `iptUnscoped` |
| Q7 SPECULATIVE: the captured `ts-input` has no magicsock UDP rule | Answered from the capture log `tsfw-linux-20261110T054239Z.log`. Lines 17–22 (iptables `ts-input`) and 28–31 (ip6tables) are the verbatim chain, with no `-p udp` rule. That run published no magicsock port when the chain was read. The fixtures copy those lines. A UDP rule cannot change the read in either case, because both reads skip non-TCP rules. |
