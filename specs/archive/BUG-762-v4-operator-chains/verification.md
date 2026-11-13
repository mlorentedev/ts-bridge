---
tags: [spec, verification]
created: "2026-11-13"
id: "BUG-762-v4-operator-chains"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-762-v4-operator-chains

## Acceptance criteria

- **AC1.** Covered by these `TestCheckForLinux` rows:
  - "an unscoped allow in an operator chain: manual";
  - "a port-less accept in an operator chain: manual, SSH caution";
  - "an allow two jumps deep, through a loop: manual".

  It is also covered by the `TestSetupForLinux` row "an unscoped allow in an operator chain: scoped rule added, nothing deleted, manual". In that row the recorded calls hold no `-D`.
- **AC2.**
  - The rows "a scoped allow in an operator chain: ok" and "an operator chain behind a narrowed jump is not read: ok" cover it. In the second, no `iptables -S fw-open` call is recorded.
  - `ts-input` is not walked: every existing iptables row records exactly one `iptables -S ts-input`, and removing the skip turns 85 tests red.
  - ufw's chains are not walked: the ufw host rows record no `iptables -S ufw-*`, and removing the skip turns 22 tests red.
  - The loop row ends, through `fw-open` → `fw-deep` → `fw-open`.
- **AC3.** Covered by the rows "an operator chain that cannot be read: unverified" (check), "the scoped rule in place and a chain that cannot be read: unverified" (setup) and "a jump to a missing chain: ok". Each of these is red if `chainUnread` is dropped from `linuxFirewallOK` or `withChains` is short-circuited. On a ufw host, the rows "ufw host, an operator chain that cannot be read: unverified (#762 review)" (check) and "ufw, the scoped allow in place and an operator chain that cannot be read: unverified (#762 review)" (setup) cover it. Both are red if `withUFWIptables` drops `chainUnread`.
- **AC5.** "an allow two jumps deep, through a loop: manual" covers both the nesting and the loop. "a jump from any address is a jump for every packet: manual (#762 review)" records the `iptables -S fw-open` read and the exposure, and it is red if `iptNarrowed` loses `0.0.0.0/0`.
- **AC4.** `make e2e-linux` on 51d753f, the round 1 fixes, gave 135 pass, 0 fail. The first run, on c4d8077, gave the same. The code is the same as in 9414418; the commits after it change docs and the spec only. The v4 jumped-chain cases all passed:
  - a tailnet accept is ok;
  - an unscoped allow is `manual`, its message names `-A tsbe2e4 -p tcp -m tcp --dport 3389 -j ACCEPT`, and `firewall_ok` is false;
  - setup reports it and leaves it;
  - check is ok again once it is removed.

## Commands

- `go test -race ./...`: ok.
- `golangci-lint run` (darwin, linux and windows): 0 issues.
- `make e2e-linux` on 51d753f: 135 pass, 0 fail. `make e2e-clean` reported no session left.

## Decision side

`iptablesDecide` is unchanged. The row "a tailnet accept in an operator chain is no rule for the port: changed (#762)" pins today's behaviour, as the proposal states.

## Review dispositions

Round 1 (nan/deepseek-v4-flash, FAIL, at a4e0f41; kept as `review-round1.md`):

- **Blocker REAL, `chainUnread` lost on a ufw host: applied.** `withUFWIptables` now carries it. The ufw-host check and setup rows were red first.
- **Major REAL, a `-s 0.0.0.0/0` jump not followed: applied.** `iptNarrowed` reads `0.0.0.0/0` as it reads `::/0`. The check row was red first. No v4 row was added to `TestIP6Jumps`: that test drives the v6 read, and the check row already records the chain read.
- **Minor REAL, v6 names on shared helpers: applied.** They are renamed `iptVerdicts`, `iptNarrowed`, `iptJumpMatch`, `iptJumpDropped`, `iptJumpCovers` and `iptJumpScopeFlags`. `ip6Exposed` stays, since only v6 uses it.
- **Minor REAL, Closing boxes stale: applied.** gosec is not installed locally, and the CI `security` job runs it. The task now says so instead of claiming a local run.
- **Minor REAL, a duplicate `[AC2]`: applied.** The nested and loop criterion is now AC5, and it also covers the any-address jump.
- **e2e not re-runnable on a dirty guest: applied.** `iptables -N tsbe2e4` falls back to flushing the chain when it exists.

Round 2 (nan/deepseek-v4-flash, PASS, at afea4e9; this `review.md`):

- **Minor REAL, 83 should be 85:** applied. The count was re-measured at 85.
- **Minor THEORETICAL, `withChains` drops an `ok` step's fix:** applied. It now prepends `step.Fix` as `withIP6` does. No `ok` step carries a fix today, so no row can reach it.
- **Minor THEORETICAL, `-s 0.0.0.0/0` on an allow inside a chain is still read as a scope:** this is the declared predicate (`iptScopedAllow`, see Risks). #743 now carries the exposure side for both literals, as commented there. `iptNarrowed` errs toward reading more chains, so it does not outrun that fix: a chain read never hides an allow that the predicate would otherwise have reported.
- **Minor REAL, no `-d 0.0.0.0/0` row:** applied. The row is "a jump to any address is a jump for every packet: manual (#762 review round 2)".
- **Minor REAL, the Closing box "features.json replays…" is unticked:** recorded here instead, as the review advises, since `tasks.md` is in the contract set. Both `features.json` commands replay and pass, `verification.md` is filled, and the review is committed.
- **Minor SPECULATIVE, one `iptables -S` per chain with no cap:** declined. It is the trade named in Risks: each chain is read once, and the cost is bounded by the host's own rule set.

After archive, PR #764 review (PR-Agent):

- **REAL, a `-m conntrack --ctstate NEW` jump is not followed, so the read is clear:** applied. `iptJumpMatch` now drops a conntrack or state match that admits NEW (`iptStateDropped`, gated on `!iptRepliesOnly`), because such a match takes every new connection. The fix applies to both walks. The new rows are in `TestIP6Jumps` and `TestCheckForLinux`, and they were red first. A jump held by a qualifier the parser still does not model (a mark, a limit) is not followed, as before. That doctrine question is filed as #766.
- **THEORETICAL, the e2e case leaves a jump behind on a rerun:** applied. Leftover jumps are deleted before the chain is created.

After archive, PR #764 review (CodeRabbit), with each row red first and red again under a mutation:

- **Major, a `changed` setup with an unreadable chain still claims "allowed on tailscale0 only":** applied. `withChains` now turns `changed` into `unverified` as well. Row: "a legacy copy replaced while an operator chain cannot be read: unverified (#764 review)".
- **Major, a deny in INPUT hides an allow in a chain INPUT jumps to first:** applied. `firewallBlockedStep` now names the `foreign` allows, and its fix includes `foreignFix`. Row: "a deny after an operator chain that admits the port: the chain's allow is named (#764 review)". The decision still does not follow the chain, as the proposal declares.
- **Minor, the runbook row for a port-less allow lacked the SSH caution:** applied.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: the one thing learned is that a new `firewallRead` field must also be carried by `withUFWIptables`. It is written as a comment on that function, where the next field gets added.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017's outcomes already cover `manual` and `unverified`, and this walk applies the v6 doctrine of #720 to v4.
