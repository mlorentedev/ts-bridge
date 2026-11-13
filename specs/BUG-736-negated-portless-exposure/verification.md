---
tags: [spec, verification, templates]
created: "2026-11-12"
id: "BUG-736-negated-portless-exposure"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-736-negated-portless-exposure

## Evidence

- [x] AC1 (v4 negated, `-p tcp -j ACCEPT`, `-j ACCEPT` → `manual`, rule named, `firewall_ok` false, SSH caution in the fix, nothing deleted) -> commits `18b6154` (red: 9 failures on the old code), `8ece357`; tests `TestExposureShapes`, `TestReadIptablesUnscoped/a_port-less_accept_is_foreign`, `.../an_accept_for_every_port_but_SSH_is_foreign`, `TestSetupForLinux/iptables,_an_accept_for_every_port:...(#736)` (appends the scoped rule, deletes nothing), `TestCheckForLinux/iptables,_an_accept_for_every_port_but_SSH:_manual_(#736)` (`FirewallOK` false).
- [x] AC2 (v6 same shapes) -> `TestExposureShapes` (both reads per row), `TestIP6Exposure/a_port-less_v6_accept`, `.../a_v6_accept_for_every_port_but_SSH` (fix names the `ip6tables -D` and the SSH caution).
- [x] AC3 (not reported: `! --dport 3389`, `! --dports 3000:4000`, mixed negated dport + positive sport, `--sport` only, DROP, UDP, `-i lo`, `-i eth0`, `-i tailscale0`, `-s` prefix, conntrack-only, a rule outside INPUT) -> the `false` rows of `TestExposureShapes`; every pre-existing step row unchanged.
- [x] AC4 (e2e) -> `make e2e-linux` at `8b93885` (round-4 head before this record), 108 pass, 0 fail, on the Lima Ubuntu guest, run 2026-11-12 11:23Z. The 15 #736 cases: v4 `! --dport 22` beside the scoped rule (manual, not `firewall_ok`, left in place, ok once removed); the same rule as the only allow (check manual; setup adds the scoped rule, reports manual, leaves it); v4 `! -i lo -p tcp -j ACCEPT` (manual, not `firewall_ok`, ok once removed); v6 `-p tcp -j ACCEPT` (manual with the SSH caution, not `firewall_ok`, left in place, ok once removed). An earlier run at `f8ac37b` gave 102/0 before rounds 2 and 3 added cases.

## Test status

- `go test -race ./...`: all packages ok.
- `golangci-lint run`: 0 issues on darwin and with `GOOS=linux`.
- `gosec`: not installed locally; the CI `security` job runs it.
- No regressions: the two rows that pinned the old "operator's posture" reading were changed on purpose (they are the bug).

## Decisions made during implementation

- `iptablesDecide` is unchanged: it already counts a port-less ACCEPT as admitting the port; only the exposure reads were blind.
- v4 scope: `iptHasScope` treats any `!` as a scope, which hid `! --dport 22`. The exposure read uses `iptScopedAllow`, where a `!` before a port flag negates the port, not the scope. The deny path keeps `iptHasScope`.
- Source ports: clients connect from ephemeral ports, so beside a negated destination port a source-port match does not narrow (`! --dport 22 --sport 1024:65535` is reported, review F2). With no destination match, only an all-negated source match (`! --sport 22`) is reported; `--sport 22` alone is not.
- A portless unscoped allow no longer counts as the scoped rule, so `setup` appends `-i tailscale0` beside it (review F4; proposal and ADR-017 say so).
- Removing a rule with no destination port can close SSH, so both fixes (v4 `foreignFix`, v6 `withIP6`) name the loopback, replies and SSH rules to add first; the list is shared with #691's policy fix as `keepOpenFirst`.
- Residual (#743, "fix(host): report a port-less allow under a negated scope as exposing the port", widened by comment on 2026-11-12): a negated non-loopback scope is not reported on v4. Two mechanisms are involved. For a port-less rule (`! -i tailscale0 -j ACCEPT`), `iptPortlessCovers` reads the negation as an unmodelled narrowing, so neither read reports it. For a port-bearing rule (`--dport 3389 ! -s 100.64.0.0/10`), `iptScopedAllow` counts the negated scope as the operator's, so v4 does not report it, while v6 does. With the scoped rule beside it, a v4 host then reads `ok` and `firewall_ok: true`. A negated loopback scope (`! -i lo`) is reported on both reads (round-2 F1).

## Spec review round 1 (nan/deepseek-v4-flash, FAIL at d167344)

- F1 (Major, REAL, ufw lines given the iptables SSH caution): applied. `iptAdmitsOthers` takes only `-A` lines; `TestWideAllowFix` pins the whole fix for `ufw allow 3389`, `ufw allow 22,3389/tcp`, a port-naming iptables rule, a port-less one, and both v6 shapes. Red at `11c2906` before the fix.
- F2 (Major, REAL, `! --dport 22 --sport 1024:65535` missed): applied, see Decisions. The `TestExposureShapes` row now expects it reported; `--dport 22 ! --sport 22` added as not reported.
- F3 (Major, REAL, empty `verification.md`, unticked tasks): the review read `d167344`, one commit before this file was filled at `cdf6ed0`; `tasks.md` boxes now ticked.
- F4 (Minor, REAL, the `scoped` flip undocumented): applied in `proposal.md` and ADR-017.
- F5 (Minor, REAL, substring fix assertions): applied by `TestWideAllowFix`, which compares whole strings.
- F6 (Minor, SPECULATIVE, `! --sport` over-report): declined, accepted in the proposal; an extra `manual` on a very unusual rule.
- Question (AC4 evidence): recorded above, `make e2e-linux` at `f8ac37b`, 102/0.

## Spec review round 2 (nan/deepseek-v4-flash, FAIL at 96247de)

- F1 (Major, REAL, `! -i lo -p tcp -j ACCEPT` hidden on v4, so a converged host read `ok`/`firewall_ok: true`): applied. `iptScopedAllow` no longer counts a negated loopback scope, through `iptOpenNegation`. New rows: `TestExposureShapes` for `! -i lo`, `! -i lo+` and `! -s 127.0.0.0/8` on both reads, and `TestCheckForLinux/iptables,_an_accept_off_loopback_beside_the_scoped_rule:_manual_(#736)`. Red at `9869eee`. An e2e case was added for it too.
- F2 (Minor, REAL, "as root" on the wrong clause in the v6 fix): applied. The fix now reads "run `ip6tables -D …` as root to delete the v6 rule (setup never writes one), after you keep loopback, …"; `TestWideAllowFix` and `TestIP6Exposure` are updated.
- F3 (Minor, REAL, v4 e2e survival check was a substring match): applied, now `grep -qx` against the printed rule.
- F4 (Minor, REAL, unticked boxes, `features.json` not replayed): applied.
- Question (#743's scope): #743 covers non-loopback negated scopes on both reads, so F1 was fixed here, not deferred. The residual above is restated.

## Spec review round 3 (nan/deepseek-v4-flash, FAIL at b6a6c7b)

- F1 (Blocker, REAL, `! --dport 22` as the only allow read "No firewall rule found"): applied. `legacyHeld` now also holds when the state is absent and a foreign rule exists, and `firewallSetupFrom` checks it before the state switch. New rows: `TestCheckForLinux/...is_the_only_allow:_manual_(#736_review_round_3)` and `TestSetupForLinux/...is_the_only_allow:_scoped_rule_added,_manual_(...)`, red at `584fcb1`. A new e2e case deletes the tailscale0 rule first, so the negated allow is the only one.
- F2 (Major, REAL, v4/v6 asymmetry on a negated non-loopback scope): deferred to #743. It predates #736, and whether v4 should change is the decision that ticket poses. #743 was widened to name both mechanisms; `proposal.md` Risks and the residual above now state the v4 `ok`/`firewall_ok: true` consequence.
- F3 (Minor, REAL, no SSH caution for `--dports 22,3389`): ticketed as #745 (pre-existing since #631) and named in Risks.
- F4 (Minor, THEORETICAL, `-d 0.0.0.0/0`): declared in Risks; no code change.
- Question (AC4): re-run at round-4 head, see AC4.
- Question (#743): its title is recorded in the residual line above.

## Spec review round 4 (nan/deepseek-v4-flash, FAIL at 6093ab9; kept as `review-round4.md`)

- Major REAL, a positive source-port match reads as narrowing (`--sport 1:65535` or `1024:65535` beside the scoped rule reads `ok`): ticketed as #757 by the owner's decision, not fixed here. The fix changes AC3's contract clause, which would need a fifth round; the owner chose to land this change and track the gap. Until #757 lands, `host check` can report `ok` on a host that admits the port through a positive source-port range.
- Major REAL, v4 reads a negated scope as the operator's scope: deferred to #743 (open), as the review itself disposes. Pre-existing, not widened here.
- Minor REAL, AC3 lists "`--sport`-only rules" as unreported although `! --sport 22` is reported: folded into #757, which rewrites the same clause.
- Minor REAL, round 3 artifacts untracked: already committed (`review-round3.md`, `review-request-round3.json` are in the branch).
- Minor REAL, `--dports 22,3389` fix text without the SSH caution: #745 (open).
- Minor THEORETICAL, `-d 0.0.0.0/0` read as a scope on v4: declared in Risks; no change.

Merging master brought #720's jumped-chain v6 walk, which pinned a port-less accept in a chain as the operator's posture. This change reads it with the same predicate as INPUT, so that row now expects the rule reported (`TestIP6Jumps/a_port-less_accept_in_a_chain_is_reported`), which is option A of #753, decided by the owner.

`make e2e-linux` on the merge head a8d2121 (#720's jumped-chain block and this change's blocks together): 126 pass, 0 fail.

PR review (CodeRabbit, after the merge with #720): two findings applied, red first. A v6 jump for every port but another (`-p tcp ! --dport 22 -j wide`) is now followed (`TestIP6Jumps/a_jump_for_every_port_but_SSH`; `! --dport 3389` is not); the merge had joined #720's jump walk, which only followed jumps naming the port, to this change's negated-port reading. The v6 delete fix now keeps ICMPv6 open first, as #733's policy fix does. The third finding (negated `-i` on v6) is #743. The e2e was not re-run for these: they change which v6 chains are walked and the fix text, which the unit rows pin, and the e2e's `SSH open first` assertion still holds.

This spec did not archive on the round 4 review (FAIL); it archives after #757 changes AC3 and a review of the new contract passes.

## #757: the round 4 Major, fixed after #742 merged (PR #759)

The owner chose to report every source-port-only allow, `--sport 22` included. Contract changed: the "What" bullet for a rule with no destination port and AC3 (which listed `--sport`-only rules as not reported, contradicting the `! --sport 22` row); `features.json` f1's behavior follows.

- Code: `iptExposes` sends a rule with no destination port match to `iptPortlessCovers`, and `iptPortlessToken` models `--sport`/`--sports` (negated or not) and `-m multiport` as narrowing nothing. `iptExcludesOthers` keeps only its destination branch. `iptMatchesPort`, and so `iptablesDecide`, is unchanged.
- TDD: bc3de59 fails 7 subtests on the old code: `TestExposureShapes` rows `--sport 22`, `--sport 1:65535`, `--sport 1024:65535`, `-m multiport --sports 1024:65535` (each `true` on v4 and v6), `TestIP6Jumps/a_jump_for_every_source_port`, and two `TestCheckForLinux` step rows (`--sport 1024:65535` beside the scoped rule, and as the only allow: `manual`). Rows that stay `false` and passed on arrival: `--sport 22` with `-m state --state ESTABLISHED` or `-m conntrack --ctstate RELATED,ESTABLISHED`, UDP `--sport 53`, and `-i lo` with a source port. ddd7e21 makes all pass.
- `go test -race ./...`: ok. `golangci-lint run`: 0 issues on darwin and `GOOS=linux`.
- `make e2e-linux` at ddd7e21: 129 pass, 0 fail. New: v4 `--sport 1024:65535` beside the scoped rule reads `manual`, not `firewall_ok`, and `ok` once removed.
- Known over-report, unchanged here: an allow held to replies that names a destination port (`--dport 3389 --ctstate RELATED,ESTABLISHED`) is still reported; #758.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? no: lesson-055 already records the gap and is updated to say it is closed.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-017's text on port-less allows is amended in place (reported `manual`, never deleted).
- [x] New pattern candidate for `00_meta/patterns/`? no: specific to this firewall read.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-736-negated-portless-exposure/` -> `specs/archive/BUG-736-negated-portless-exposure/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
