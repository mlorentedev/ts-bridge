---
tags: [spec, verification]
created: "2026-10-22"
---

# Verification - BUG-481-firewall-deny-order

## Evidence

- [x] AC1 -> `TestFirewallDecideOrder` (`deny before allow`, `reject before allow`, `deny and no allow`; each on ufw and iptables); `TestIptablesDecideOnly` (`blanket drop before the allow`, `tcp reset before the allow`); `TestSetupForLinux/ufw, a deny decides the port first` and `/iptables, a drop and no allow: nothing appended after it` (`manual`, no `run`); `TestCheckForLinux/iptables, a drop decides the port first` (`manual`, `firewall_ok` false). Each of those three rows also asserts that the `manual` message names the deciding rule (`assertNamesBlockedRule`: `ufw deny 3389`, `-A INPUT -p tcp -m tcp --dport 3389 -j DROP`).
- [x] AC2 -> `TestFirewallDecideOrder` (`hardened: scoped deny, then scoped allow`, `interface-scoped deny, then allow`).
- [x] AC3 -> `TestFirewallDecideOrder` (`allow before deny`, `deny on another port`, `udp deny`, `outbound deny`, `deny with a comment naming an interface`, `nothing`); `TestIptablesDecideOnly` (`conntrack drop does not decide`, `negated-scope drop does not decide`, `loopback drop does not decide`, `jump does not decide`).
- [x] AC4 -> `TestFirewallDecideOrder` runs every row through `ufwDecide` and `iptablesDecide` with one expected answer.

## Test status

- Red-green: every new case fails against the base (`fix/issue-483-iptables-portless-allow` @ `5572d4c`) and passes now.
- `go test -race -count=1 ./...` -> all packages ok
- `GOOS=linux|windows|darwin go vet ./...` -> clean; `golangci-lint run` -> 0 issues
- Rule text in the cases is what ufw 0.36.2 (`ufw show added`) and iptables 1.8.10 nf_tables (`-S INPUT`) print in `ubuntu:24.04`.
- End to end in `ubuntu:24.04` with `NET_ADMIN` and a `GOOS=linux` binary:
  - ufw `deny 3389` then `allow 3389/tcp`: `host setup` reports the firewall step as a WARNING naming `ufw deny 3389`, and `ufw show added` is unchanged afterwards;
  - iptables `-A INPUT -p tcp --dport 3389 -j DROP` and no allow: setup appends nothing, and `host check` exits 2;
  - ufw hardened ruleset (`deny from 192.168.0.0/16 ... 3389`, then `allow in on tailscale0 ... 3389`): "Firewall allows".

## Review dispositions

Independent review `nan/deepseek-v4-flash` at `a466a8b`: PASS-WITH-GAPS, three Minors. None touches `proposal.md`, `tasks.md` or `features.json`, so the contract digests stay as reviewed.

- **F3, the message naming the rule (applied).** The setup rows for ufw and iptables and the iptables check row now assert the `manual` message contains the deciding rule. Red-green: dropping `read.blockedBy` from `firewallBlockedStep` fails all three; restored, they pass.
- **F1, a port-bearing deny with an unmodeled match (declined, behaviour pinned).** `-A INPUT -p tcp -m tcp --dport 3389 -m conntrack --ctstate INVALID -j DROP` before an allow decides the port. That is parity, not an oversight: the "What" definition is "a deny decides when it would cover the port if it were an allow", and `iptPortCovers` ignores non-port matches on an ACCEPT, so the same rule as an ACCEPT reads `present`. Modelling the extra match for denies alone would make the two directions disagree on one rule. The cost is a false `manual` (and `check` exit 2) on an uncommon ruleset, the safe direction, already priced in §Risks. AC3's bullet, read alone, over-claims for this shape; it stays as written because the spec is archiving and the code follows the operative definition. Pinned by `TestIptablesDecideOnly/port-bearing conntrack drop decides`, so the answer cannot flip silently. No ticket: a ticket would only reword an archived criterion.
- **F2, `ufw insert` ordering (premise, unverified).** The ufw path assumes `ufw show added` lists rules in evaluation order. Verified in `ubuntu:24.04` (ufw 0.36.2) for appended rules: `deny 3389` then `allow 3389/tcp` print in that order, and an identical match replaces in place. Not verified for a rule placed with `ufw insert N`: the reviewer could not reproduce that listing and no case covers it. If `show added` printed inserted rules out of evaluation order, the walk could stop at the wrong first rule.
- **gosec**: not run locally; the CI `security` job is the evidence. The diff adds no exec, path or input handling.
- **Question, `ufw limit`** (pre-existing, out of scope): a rate-limited allow is skipped, so such a host reads `absent` and setup appends a plain allow after it. Consistent with the proposal's "route, out and limit rules never decide"; not changed here.

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-047-2026-10-22.md
- [x] ADR-worthy decision for the repo's `docs/adr/`? no: ADR-017 already says setup converges without deleting operator state; reporting an unscoped deny as `manual` applies that rule, it does not change it.
- [x] New pattern candidate for `00_meta/patterns/`? no: the cross-project point (read rules in evaluation order) is already the general shape of lesson 046's allow-list guidance; the rest is ts-bridge firewall detail.
