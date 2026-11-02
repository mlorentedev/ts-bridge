---
id: "BUG-480-iptables-duplicate-rules"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-11-01"
issue: "mlorentedev/ts-bridge#480"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, linux, firewall]
template_version: "1.0"
---

# BUG-480-iptables-duplicate-rules

## Why

Before #477, Linux `host setup` ran `iptables -A INPUT -p tcp --dport <port> -j ACCEPT` on every run, so a host set up N times carries N identical copies of that rule. Since #477 setup sees the first copy and reports `ok`, but nothing reports the others: `host check` cannot tell one rule from five. The drift stays silent, and an operator who later deletes "the" rule leaves the port still open through a copy (#480).

## What

On the iptables backend, `host check` counts the lines of `iptables -S INPUT` whose fields equal the printed form of the rule setup appends: `-A INPUT -p tcp -m tcp --dport <port> -j ACCEPT` (iptables prints `-m tcp`, which setup's command line omits). When the firewall reads as present and that count is greater than one:

- the firewall step reports **`manual`**. Its message names the count and the rule, and its fix gives the command that removes one copy (`iptables -D INPUT -p tcp --dport <port> -j ACCEPT`) and how many times to run it, so one copy remains;
- `firewall_ok` in the services JSON stays **true**, because the port is still allowed.

Only that exact form counts. A scoped allow (`-i tailscale0 ...`) next to the unscoped one is the operator's hardening (#479), not drift. Rules inside `ts-input` never match, because they are `-A ts-input` lines.

When a deny decides the port first, the blocked step (BUG-481) is reported as today, and duplicates add nothing to it.

`host setup` is unchanged: it still reports `ok` on such a host and deletes nothing. On that host check and setup therefore differ (`manual` against `ok`). Setup reports what it converged; check reports drift it cannot fix.

## Out of scope

- `host setup` deleting the extra copies. Deleting firewall rules the operator may rely on is a separate decision, as #480 states, and setup's contract is that it never deletes or reorders firewall rules.
- `host setup` reporting the copies. Its outcome stays the one #477 defined.
- ufw. `ufw allow` skips a rule that already exists, so ufw never piles up copies.
- Copies of any other rule shape, and duplicates inside other chains.

## Risks / open questions

- **Outcome differs from the ticket.** #480 proposes `changed`. On the check path, `changed` means "setup would converge this" (`firewallReadStep` reports `changed` for a missing rule, and setup then appends it). Setup will not delete copies, so `changed` would promise a convergence that never comes. `manual` with a fix is the outcome ADR-017 gives to drift the operator must resolve, the same shape as the blocked step.
- **Exit status.** `host check` exits 2 unless every step is `ok`. Hosts that were set up more than once before #477 move from exit 0 to exit 2. Under the ticket's `changed` they would too. That is the purpose of the change, and it is recorded here so that no consumer discovers it by surprise.
- **Printed form.** `-p tcp -m tcp --dport 3389 -j ACCEPT` is what iptables 1.8.10 (nf_tables) prints for the rule setup appends. The BUG-492 end-to-end run in `ubuntu:24.04` recorded that exact line after `host setup`.

## Acceptance criteria

- [ ] AC1: with two copies of the setup rule in `iptables -S INPUT`, `host check` reports the firewall step as `manual`. The message names the count, the fix names `iptables -D INPUT -p tcp --dport <port> -j ACCEPT` and how many times to run it, and `firewall_ok` stays true.
- [ ] AC2: one copy, or one copy beside a scoped allow for the same port, reports `ok` as today.
- [ ] AC3: copies behind tailscaled's unqualified `ts-input` jump are still counted, and the chain's own rules never count.
- [ ] AC4: when a deny decides the port before the copies, the step is the blocked `manual` step of BUG-481, naming the deny, with no count.
- [ ] AC5: `host setup` on a host with copies reports `ok` and runs no `iptables -D`.
- [ ] AC6: every fixture uses the verbatim `iptables -S` printed form.

## References

- Issue #480; #477 (setup stops appending), #479 (scoped allows), #481 (deny order), #492 (ts-input).
- `specs/archive/BUG-035-iptables-scoped-allow/`, `specs/archive/BUG-481-firewall-deny-order/`, `specs/archive/BUG-492-ts-input-chain/`.
- ADR-017 outcome model.
