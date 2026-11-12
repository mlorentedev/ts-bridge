---
tags: [spec, verification, templates]
created: "2026-11-12"
id: "BUG-739-foreign-firewall"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-739-foreign-firewall

## Evidence

- [x] AC1 (firewalld running: check `unverified`, setup `manual`, no write, no policy note, `firewall_ok` false) -> `TestForeignFirewall/firewalld_running` and `.../firewalld_running_wins_over_its_nft_table`. Also probed by hand on an Ubuntu 24.04 guest with firewalld 2.1 and the built binary: check `unverified` "Cannot verify TCP 3389: firewalld is running, which ts-bridge does not read", `firewall_ok` False; setup `manual`, `iptables -S INPUT` unchanged. With firewalld stopped (`not running`, exit 252) the read returned to the usual path.
- [x] AC2 (native nft input table; iptables-nft, tailscaled and hookless tables ignored) -> `TestForeignFirewall/native_inet_filter`, `.../firewalld_table,_firewall-cmd_absent`, `.../iptables-nft_and_tailscaled_tables_only`, `.../table_without_an_input_hook`; parser cases in `TestNFTForeignInput`.
- [x] AC3 (a failed probe is unknown) -> `TestForeignFirewall/firewalld_state_fails`, `.../nft_listing_fails`: check `unverified`, setup `failed`, no write.
- [x] AC4 (iptables and ufw unchanged) -> the existing host suite passes. The only fixture change is that the call-sequence filter (`withoutIP6Probe`, now `withoutProbes`) also drops the probe's two lookups, and `TestIP6Exposure` pins them at the head of the read.
- [x] AC5 (e2e native nft table) -> `scripts/e2e/linux.sh` "native nft table" block. `make e2e-linux` on 999c0a7: 107 pass, 0 fail, including the four native nft checks (check `unverified` naming `table inet tsbe2e`, no `firewall_ok`, setup `manual` with `iptables -S INPUT` unchanged, `ok` again once the table is deleted). Re-run on 67fe2b2, after the round 1 fix to the firewalld probe (144d34f): 107 pass, 0 fail.

## Test status

- `go test -race ./...`: ok. `golangci-lint run` (host and `GOOS=linux`): 0 issues.
- Manual: the firewalld probe above (guest `probe739`, since deleted). Ubuntu ships `nft` (`/usr/sbin/nft`), and `nft list chains` prints `table <family> <name> {` / `chain <name> {` / `type ... hook <hook> ...`, the shape the parser reads.
- No regressions: yes.

## Decisions made during implementation

- **setup is `manual`, not `unverified`:** ADR-017 gives setup no `unverified` outcome, and `manual` is "the operator acts; the fix names the action". check is `unverified` as the issue asks: ts-bridge cannot read the other firewall.
- **The probe runs before the backend choice and returns early:** neither `ufw` nor `iptables` is read, so neither policy note nor the v6 exposure scan can appear. The backend stays empty, so `policyRead` is false. The step builders return `foreignStep` before `withPolicy`/`withIP6`.
- **Fixture with no answer:** `recordingRunner` returns empty stdout for a command with no `output` key, and an empty `nft list chains` is the true "no tables" answer, so existing fixtures with `nft` on PATH stay valid. A listing that fails is an unknown read, never "nothing found".
- **firewalld fix:** `--change-interface`, not `--add-interface`. Probed: `--add-interface` fails with `ZONE_CONFLICT` (exit 18) for an interface another zone holds, while `--change-interface` moves it and also binds an unbound one. `--new-zone` fails with `NAME_CONFLICT` when the zone exists, hence "(once)".
- **No firewalld e2e:** it needs `apt-get update` and a package install on the guest; the hand probe stands in, as the proposal states.

## Review dispositions

Round 1 (nan/deepseek-v4-flash, FAIL, at 0342f1d; kept as `review-round1.md`):

- Major REAL, AC5 unrun: applied. `make e2e-linux` ran on 999c0a7 (the code the review read) and passed, 107/0, see AC5. The review read the branch before the run was recorded.
- Major THEORETICAL, no `nft list chains` capture from the tool: applied by the same run. The e2e block creates an `inet` table on the guest, and the real `nft` output then drives check to `unverified` naming `table inet tsbe2e`. A parser that misread the real format would have left check `ok`, failing the e2e. The unit fixtures follow the probe output byte for byte: tab indentation, `type filter hook input priority filter; policy accept;`, and firewalld's `priority filter + 10`.
- Minor THEORETICAL, `not running` with exit 0: applied. The words decide and exit 252 is the convention; the new row is `TestForeignFirewall/firewalld_not_running,_exit_0`, red first.
- Minor REAL, `mangle`/`raw` untested: applied. Two `TestNFTForeignInput` rows now pin them. They passed on arrival, since the code already excluded both, and they guard the mutation the review ran.
- Minor REAL, bookkeeping: applied (ACs and closing tasks ticked, `features.json` ends in a newline).
- Question, a failed `nft list chains` outranks a readable ufw: declined, decided in the proposal. An unusable `nft` on root means the ruleset that filters the host is unknown. ufw's answer would be a guess the read refuses elsewhere (#532), and setup still writes nothing.

Round 2 (nan/deepseek-v4-flash, PASS, at 67fe2b2; this `review.md`):

- Minor REAL, provenance: applied. AC5 now records the re-run on 67fe2b2, which carries 144d34f. The guest has no `firewall-cmd`, so that change never runs there.
- Minor REAL, a fixture with no answer: the proposal's sentence ("shows up as a failing test") overstates it, and the code does what this file says under "Fixture with no answer". An unanswered `nft list chains` is empty stdout, which is the true "no tables" answer, not a guess. The proposal prose is left as reviewed; this line corrects it.
- Minor REAL, no `checkFor` coverage: applied. A `TestCheckForLinux` row runs firewalld through `checkFor`, asserting `unverified`, `firewall_ok` false and no ufw read. Mutation: reading `readLinuxFirewallV4` in `checkLinux` turns the row red. It passed on arrival, being a guard.
- Minor THEORETICAL, runbook "false alarm" rows: applied. The #691 and #733 rows (written in #734 and #746, not this diff) now say firewalld and native nftables are reported by name since #739.
- Minor REAL, parity full stop: applied.
- Minor SPECULATIVE, no `nft` binary on a host with a native ruleset: accepted limit, as the proposal declares. Without `nft` nothing on the host can list the ruleset; reading `/proc` or netlink directly is out of scope.
- Question, probe precedence: unchanged from round 1.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the gap (a read that assumes it is the only firewall) is recorded in the issue, ADR-017 and the runbook rows; no new mechanism to teach.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 Amendment B gained the #739 line; no new decision beyond "report, never write, a firewall ts-bridge does not own".
- [x] New pattern candidate for `00_meta/patterns/`? no: single project.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-739-foreign-firewall/` -> `specs/archive/BUG-739-foreign-firewall/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
