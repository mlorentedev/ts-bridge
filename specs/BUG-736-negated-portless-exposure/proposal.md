---
id: "BUG-736-negated-portless-exposure"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-12"
issue: "mlorentedev/ts-bridge#736"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-736-negated-portless-exposure

## Why

The Linux firewall reads report an unscoped allow for the RDP port as `manual`, so the operator learns the `tailscale0` scope is void (#631 for v4, #526 for v6). Both reads decide "allows the port" through `iptPortCovers`, which only matches a rule that names the port in a non-negated destination list. Two common shapes open the port on every interface and are missed:

- a negated port, `-A INPUT -p tcp -m tcp ! --dport 22 -j ACCEPT` ("everything but SSH");
- a portless allow, `-A INPUT -p tcp -j ACCEPT` or `-A INPUT -j ACCEPT`.

On v4 the miss is worse than silence: `iptablesDecide` already counts a portless ACCEPT as admitting the port, so the step reads `ok` with `firewall_ok: true` while every port is open, the same false report #691 removed for a permissive policy. Found by the BUG-526 round-2 review (negated port) and CodeRabbit on #738 (portless).

## What

One predicate, `iptExposes(f, port)`, used by both exposure reads (`iptUnscoped` for v4 INPUT, `ip6Exposed` for v6 INPUT) in place of `iptPortCovers`:

- a rule with a port flag exposes the port when it names it (today's `iptPortCovers`), or when its only port match is a negated destination list that does not contain the port, with TCP as the protocol;
- a rule with no port flag exposes the port when `iptPortlessCovers` already says it matches every TCP port (it rejects conntrack, state, limit and other narrowings it does not model).

The existing scope tests stay: loopback, and an interface, source or destination qualifier (v4 `iptHasScope`, v6 `ip6Narrowed`), still exclude a rule. Such rules are reported as `manual` (foreign: never deleted by setup), and `firewall_ok` is false. `iptablesDecide` is unchanged: whether the tailnet reaches the port is a separate question from whether anything else does.

This reverses the documented choice in `iptUnscoped` that "a rule with no port in it is the operator's posture": an unscoped TCP-wide ACCEPT voids the scope exactly as a permissive policy does, and #691 decided that is reported.

## Out of scope

- Rules in chains INPUT jumps to (#720).
- Rule order (an allow behind an unscoped deny is still listed, as today).
- A negated port in `iptablesDecide`.

## Risks / open questions

- Hosts with a deliberate unscoped `-A INPUT -j ACCEPT` turn `manual`. Intended: the scope is void there.
- Over-reporting: a portless rule narrowed in a way the parser does not model is not reported (`iptPortlessCovers` fails closed toward "narrowed"); a negated-port rule with an unmodelled narrowing is reported, as port-bearing rules already are.

## Acceptance criteria

- [ ] [AC1] v4: an unscoped `! --dport 22` ACCEPT, a `-p tcp -j ACCEPT` and a bare `-j ACCEPT` in INPUT are each reported `manual` by `check` and `setup`, naming the rule, with `firewall_ok` false; setup deletes none of them.
- [ ] [AC2] v6: the same three shapes in `ip6tables -S INPUT` are reported by the exposure scan (`manual`, `firewall_ok` false).
- [ ] [AC3] Not reported: `! --dport 3389`, a `-i lo` or `-i eth0`/`-s` scoped portless allow, a conntrack-only allow, a UDP portless allow, and `--sport`-only rules; existing step outcomes are unchanged.
- [ ] [AC4] e2e on the Linux VM: under a DROP policy, an unscoped `! --dport 22` v4 allow and a `-p tcp -j ACCEPT` v6 allow are each reported `manual` and `firewall_ok` false, then `ok` once removed.

## References

- #736 and its 2026-11-12 comment; #631, #526, #691; BUG-526 round-2 review F1; CodeRabbit on #738.
