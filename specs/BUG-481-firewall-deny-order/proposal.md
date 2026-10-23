---
id: "BUG-481-firewall-deny-order"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-10-22"
issue: "mlorentedev/ts-bridge#481"
tags: [spec, proposal, host, linux, firewall]
template_version: "1.0"
---

# BUG-481-firewall-deny-order

## Why

Both Linux firewall reads answer only "does an allow cover the port", and they ignore rule order. A deny or DROP that comes first shadows the allow, so `host check` reports the firewall `ok` while RDP is blocked. When no allow exists at all, `host setup` appends one after the deny, where it never matches, and reports `changed` (#481). The fix must not misreport the legitimate hardened ruleset, "deny the LAN, allow Tailscale", where a scoped deny precedes a scoped allow.

## What

Each backend reads its rules **in evaluation order** and stops at the first rule that decides the port:

- a covering allow (the existing BUG-035/BUG-483 rules) reads as **present**;
- an **unscoped** deny on the port reads as **blocked**.

A deny is unscoped when it would cover the port if it were an allow, and it carries no interface, source or destination qualifier, negated or not. A scoped deny is the operator's judgment and does not decide.

- **iptables** (`-S INPUT`): the target is `DROP` or `REJECT`, and `--reject-with <type>` is ignored. Unscoped means no `-i`, `-s`, `-d` or `!` scope. `-A INPUT -j DROP` and `-A INPUT -p tcp -j REJECT --reject-with tcp-reset` are unscoped and cover every port. A deny narrowed by an unmodeled match (`-m conntrack --ctstate INVALID -j DROP`) does not decide, exactly as the same shape would not count as an allow.
- **ufw** (`ufw show added`, which lists user rules in `user.rules` order): the action is `deny` or `reject`, and `out` and `route` rules are excluded as they are for allows. Unscoped means no `from` and no `on`. ufw folds `from any` away, so `ufw deny 3389`, `ufw deny 3389/tcp` and `ufw deny proto tcp to any port 3389` are unscoped.

Blocked is reported as **`manual`** in both `setup` and `check`. Setup never deletes or reorders firewall rules, so it runs no command. The message names the rule, and the fix says to remove or reorder it. `Services[].firewall_ok` is false. With #476, `setup` exits 0 and `check` exits 2.

## Out of scope

- tailscaled's `ts-input` chain (#492), and rules inside other jumped-to chains.
- Policies: `-P INPUT DROP` does not decide, since a later allow still admits.
- A scoped deny that happens to match Tailscale traffic (`-i tailscale0 --dport 3389 -j DROP`). This is a deliberate block, not a mistake, and judging it is the operator's call.
- ip6tables, nftables-only and firewalld hosts.

## Risks / open questions

- Measured in `ubuntu:24.04` (ufw 0.36.2, iptables 1.8.10 nf_tables):
  - `ufw show added` prints rules in `user.rules` order, `ufw insert 1` rules included, which is the order ufw evaluates them.
  - ufw replaces a rule with an identical match in place ("Rules updated"), so `allow 3389/tcp` and `deny 3389/tcp` never coexist. A `deny 3389` (any protocol) and an `allow 3389/tcp` do.
  - `ufw deny from any to any port 3389` is stored as `ufw deny 3389`, and `ufw deny proto tcp to any port 3389` as `deny 3389/tcp`.
  - iptables prints `-j REJECT --reject-with icmp-port-unreachable` for a bare `-j REJECT`, and `--reject-with tcp-reset` as typed.
- The cost of a false `blocked` is a `manual` note on a working host. The cost of a missed block is today's silent `ok`. Only unscoped denies decide, so the hardened ruleset never reads as blocked.

## Acceptance criteria

- [ ] AC1: On both backends, an unscoped deny on the port before any covering allow reads as blocked. Setup and check report `manual`, naming the rule, and setup runs no firewall command. This holds both when an allow follows the deny and when no allow exists.
- [ ] AC2: On both backends, the hardened ruleset reads as present (`ok`): a scoped deny on the port (source, or interface on iptables), then a scoped allow.
- [ ] AC3: An allow before an unscoped deny reads as present. A deny on another port or on UDP, and an iptables deny narrowed by an unmodeled match, does not decide.
- [ ] AC4: One table test feeds the same rulesets to both backends, written in each backend's verbatim syntax, and expects the same answer from each.

## References

- Issue #481; `specs/archive/BUG-035-iptables-scoped-allow/`, `specs/archive/BUG-483-iptables-portless-allow/`.
- #476 (exit status), #492 (ts-input).
- ADR-017 outcome model (`manual`).
