---
tags: [spec, verification]
created: "2026-10-22"
---

# Verification - BUG-481-firewall-deny-order

## Evidence

- [x] AC1 -> `TestFirewallDecideOrder` (`deny before allow`, `reject before allow`, `deny and no allow`; each on ufw and iptables); `TestIptablesDecideOnly` (`blanket drop before the allow`, `tcp reset before the allow`); `TestSetupForLinux/ufw, a deny decides the port first` and `/iptables, a drop and no allow: nothing appended after it` (`manual`, no `run`); `TestCheckForLinux/iptables, a drop decides the port first` (`manual`, `firewall_ok` false).
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

## Promotion candidates

- no: the lesson (docs/lessons/lesson-047-2026-10-22.md) is ts-bridge firewall detail; the cross-project point (read rules in evaluation order) is already the general shape of lesson 046's allow-list guidance.
