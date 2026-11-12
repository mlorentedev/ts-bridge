---
id: "BUG-526-tailscaled-firewall-modes"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-11-09"
issue: "mlorentedev/ts-bridge#526"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-526-tailscaled-firewall-modes

## Why

#492 taught the Linux firewall read to follow INPUT's jump to tailscaled's `ts-input`, but only
for IPv4 iptables with `--netfilter-mode=on`. Three configurations stayed outside it: nodivert,
nftables mode and IPv6. In the first two, a read that misses tailscaled's tailnet ACCEPT can make
`host setup` append a rule that opens the port on every interface. In the third, `check` cannot
see a v6 rule that opens the port on every interface, so it reports a host converged while it is
exposed over IPv6.

## What

- **nodivert** (already on master, now pinned with a capture): `ts-input` exists but INPUT does
  not jump to it, so the read is `unverified` and names the mode. Setup writes nothing.
- **nftables mode** (already on master, now pinned with a capture): on a host whose `iptables` is
  the nf_tables backend, `iptables -S` prints tailscaled's chain, so the read is the same as in
  iptables mode. With the legacy backend, `nft list chain ip filter ts-input` finds the chain and
  the read is `unverified` naming nftables mode.
- **IPv6** (new): where `ip6tables` is on the path, `check` and `setup` also read
  `ip6tables -S INPUT` and look for unscoped allows for the port: no interface, source or
  destination. Such a rule is reported as `manual` and `firewall_ok` is false. The message names
  `ip6tables` and the rule, and the fix is its `ip6tables -D` line. Setup never deletes it, since
  setup never wrote a v6 rule. If the v6 read fails, a note is added to the step message and the
  outcome does not change.
- The fixtures for all three cases come verbatim from a real host:
  `scripts/e2e/capture-tsfw.sh` enrols tailscaled 1.104.4 against the local Headscale and records
  every mode (log `tsfw-linux-20261110T054239Z.log`).

## Out of scope

- IPv6 reachability. The descriptor carries `tailscale ip -4`, so whether v6 admits the tailnet
  decides nothing. The v6 read is an exposure scan, not a second reachability decision. A host
  that converges on IPv4 stays converged whatever v6 holds, unless v6 holds an unscoped allow
  for the port.
- Writing or deleting v6 rules.
- ufw's v6 rules. ufw keeps them in its own `ufw6-*` chains, and `ufw show added` already lists
  both families, so the ufw read covers them.
- nftables mode on a host whose `iptables` is the legacy backend, beyond the existing
  `unverified` report. The capture did not cover that combination.

## Risks / open questions

- **Hosts without IPv6** (`ipv6.disable=1`): `ip6tables -S` exits non-zero. That is a note, not a
  failure, so such a host still converges.
- **Port-less v6 accepts** (`-A INPUT -p tcp -j ACCEPT`) are the operator's posture, as in IPv4
  (#631), and are not reported. A v6 policy ACCEPT is not reported either, matching the IPv4 read.
- **Decision fork, taken by the owner on 2026-11-10:** read ip6tables in this PR rather than
  record IPv6 as a non-goal. The read is scoped to exposure so that it cannot change a converged
  v4 host's outcome for any other reason.

## Acceptance criteria

- [ ] AC1: With `--netfilter-mode=nodivert`, check reports `unverified` naming the mode, and
  setup writes nothing. Pinned by the captured fixtures.
- [ ] AC2: In nftables mode on an nf_tables `iptables`, the read admits the port through
  tailscaled's ACCEPT, as in iptables mode. Pinned by the captured fixtures.
- [ ] AC3: An unscoped v6 allow for the port in `ip6tables -S INPUT` makes check and setup report
  `manual`, naming the rule and its `ip6tables -D` fix, with `firewall_ok` false. Setup deletes
  nothing.
- [ ] AC4: A v6 read that fails, or `ip6tables` missing from the path, leaves the IPv4 outcome
  unchanged. A failed read adds a note to the message.
- [ ] AC5: The Linux e2e adds an unscoped v6 allow and sees check report it, and after the
  operator deletes it, sees check return `ok`.

## References

- Issue #526 (split out of #492); ADR-017 Amendment B (the scope rule) and #631 (unscoped allows)
- Lesson 055 and its 2026-11-10 correction (`docs/lessons/lesson-055-2026-11-04.md`)
- Capture driver `scripts/e2e/capture-tsfw.sh` and guest script `scripts/e2e/tsfw_linux.sh`
