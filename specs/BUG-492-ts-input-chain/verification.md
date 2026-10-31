---
tags: [spec, verification, templates]
created: "2026-10-30"
---

# Verification - BUG-492-ts-input-chain

## Evidence

- [x] AC1 -> `TestIptablesTSInput/tailscaled default`; `TestIptablesRule/present inside ts-input`; `TestSetupForLinux/iptables, tailscaled's ts-input accepts the tailnet` (`ok`, no `run`); `TestCheckForLinux/iptables, tailscaled's ts-input accepts the tailnet` (`ok`, `firewall_ok` true).
- [x] AC2 -> `TestIptablesTSInput` (`deny before the jump decides first`, `deny after the jump is shadowed`, `tailscaled's other rules decide nothing`, `and an INPUT allow after the jump still counts`). The splice at the jump is also pinned through `readIptables`: `TestIptablesRule/a deny after the jump is shadowed by ts-input` and `TestSetupForLinux/iptables, a deny after the jump is shadowed by ts-input` (round 2, F1).
- [x] AC3 -> `TestIptablesTSInput` (`a qualified jump is not followed`, `no jump, nothing to follow`); `TestSetupForLinux` (`iptables, no ts-input jump: one read`, `iptables, a qualified ts-input jump is not followed`: the call lists hold no `iptables -S ts-input`). Every existing iptables row whose fixture jumps now expects the second read.
- [x] AC4 -> `TestIptablesRule/ts-input unreadable` (unknown); `TestSetupForLinux/iptables, ts-input unreadable: nothing appended` (`failed`, no `run`); `TestCheckForLinux/iptables, ts-input unreadable` (`unverified`). All three assert that the reason, or the step message, contains `iptables -S ts-input` (round 2, F4).
- [x] AC5 -> `tsInputDefault`/`tsInputNoAccept` in `linux_steps_test.go`, the printed form recorded on #492.

## Test status

- Red first, **corrected after review round 1** (F3). The earlier figure of "19 cases" was never measured; the reviewer counted 5 subtests and 9 FAIL lines against the original stub. Measured on the round-2 suite with the jump check forced false in `readIptables` (the pre-fix behaviour: INPUT only, no second read): 19 subtests fail, 22 `--- FAIL` lines including the parent tests. Most of them fail only on the call list, because the row now expects `output iptables -S ts-input`. The ones that fail on the decision are `present inside ts-input`, the two `accepts the tailnet` rows, the three `unreadable` rows, and the two `deny after the jump is shadowed by ts-input` rows.
- Mutation checks, one at a time, each reverted with the suite back to `ok`. The output was measured in round 2:
  - following any jump that ends in `-j ts-input` (`strings.HasSuffix`) fails `TestSetupForLinux/iptables, a qualified ts-input jump is not followed` and `TestIptablesTSInput/a qualified jump is not followed`;
  - ignoring the chain's read error fails the three `ts-input unreadable` rows (`TestIptablesRule`, `TestSetupForLinux`, `TestCheckForLinux`);
  - dropping the command from that reason (`iptables: <err>`) fails the same three rows on the new message and reason assertions (F4);
  - reading the chain but not using it (`_ = chain`) fails `present inside ts-input`, the two `accepts the tailnet` rows, and the two `deny after the jump is shadowed by ts-input` rows;
  - appending the chain to the end of INPUT instead of at the jump (`rules = rules + "\n" + string(chain)`) fails `TestIptablesRule/a deny after the jump is shadowed by ts-input` and `TestSetupForLinux/iptables, a deny after the jump is shadowed by ts-input`. **Correction (F1/F2):** round 1 claimed this mutation was caught. It was not: `TestIptablesTSInput` calls `iptInlineChain` directly, so it never exercises the call site, and the whole suite passed. The two rows above go through `readIptables`, and they were added in round 2 to close that gap.
- `go test -race -count=1 ./...` -> all packages ok.
- `GOOS=linux|windows|darwin go vet ./...` -> clean. `golangci-lint run` -> 0 issues. `gofmt -l internal/host` -> empty. The eight files `gofmt -l .` lists elsewhere predate this change (#466).
- End to end in `ubuntu:24.04` (`NET_ADMIN`), with a `GOOS=linux` binary and tailscaled's rules replayed:
  - `host setup`: "Firewall allows TCP 3389 (iptables)", and `iptables -S INPUT` is unchanged (`-P INPUT ACCEPT`, `-A INPUT -j ts-input`);
  - `host check --json`: the firewall step is `ok` and `firewall_ok` is true. The exit status is 2 because the container has no xrdp and no Tailscale;
  - with the tailnet ACCEPT deleted from `ts-input`, `host setup` appends `-A INPUT -p tcp -m tcp --dport 3389 -j ACCEPT` as before (`changed`).

## Decisions made during implementation

- `iptablesDecide` is unchanged. The chain is spliced into the INPUT text at the jump (`iptInlineChain`), so BUG-035/483/481's rules apply to its lines without a second walker. `-N ts-input` is skipped like any non-`-A` line.
- The lesson is numbered 050 because the HOST-005 A2 branch (#458, a parallel session) has lesson-049 drafted. `check-lessons.sh` allows gaps but rejects duplicates.

## Review round 1 (nan/deepseek-v4-flash, FAIL)

- F1 Major, applied: the splice position had no test through `readIptables`, so appending the chain at the end of INPUT passed the suite. Two rows now go through the call site and fail under that mutation (see Test status).
- F2 Major, applied: the false mutation claim is corrected above, and every mutation was re-run and recorded with its output.
- F3 Minor, applied: the red count is replaced with a measured one, and the old figure is marked as unmeasured.
- F4 Minor, applied: the unreadable rows assert `iptables -S ts-input` in the reason (`TestIptablesRule`) and in the step message (`TestSetupForLinux`, `TestCheckForLinux`) through `assertFirewallMessage`.
- F5 Minor, declined as an accepted risk: between the two reads, tailscaled's `ClearChain` can leave `ts-input` empty. The read then reports absent, and setup appends the unscoped ACCEPT it appended before #492. The window lasts only while tailscaled restarts, it leaves the host no worse than before this change, and closing it would take an atomic read (`iptables-save`) that the proposal does not cover.
- F6 Minor, ticketed: hosts with ufw still run the unscoped `ufw allow <port>/tcp`, because `ufw show added` does not list tailscaled's rules. Tracked as #530 (Backlog/P2/bug, `BUG-530-ufw-unscoped-allow`).

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-050-2026-10-30.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017 already says setup appends only when no allow covers the port; reading tailscaled's chain widens what "covers" reads, not the contract.
- [x] New pattern candidate for `00_meta/patterns/`? no: "follow only the indirection whose contents you know" is ts-bridge firewall detail here, and lesson 046 already carries the general allow-list point.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-492-ts-input-chain/` -> `specs/archive/BUG-492-ts-input-chain/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
