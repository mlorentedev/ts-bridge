---
id: "BUG-492-ts-input-chain"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-10-30"
issue: "mlorentedev/ts-bridge#492"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, linux, firewall]
template_version: "1.0"
---

# BUG-492-ts-input-chain

## Why

With `--netfilter-mode=on`, the default, tailscaled inserts `-A INPUT -j ts-input` at the top of INPUT and appends `-A ts-input -i tailscale0 -j ACCEPT`. Tailnet traffic to the RDP port is therefore already accepted. The Linux iptables read looks only at `iptables -S INPUT` and treats the jump as opaque. On that default host, `host check` reports the firewall as absent, and `host setup` appends `-A INPUT -p tcp --dport <port> -j ACCEPT`. That rule adds no tailnet reachability and opens the port on every interface (#492). It is the most common case of the widening that #479 and #483 address.

## What

When INPUT holds an **unqualified** jump to `ts-input` (exactly `-A INPUT -j ts-input`, the rule tailscaled inserts), the iptables read also runs `iptables -S ts-input`. The chain's rules are then evaluated **at the jump's position**: the walk reads INPUT's rules before the jump, then the chain's rules, then INPUT's rules after the jump. The existing decision rules (BUG-035, BUG-483, BUG-481) apply unchanged to every line, so:

- `-A ts-input -i tailscale0 -j ACCEPT` covers the port (an interface-scoped ACCEPT), and the firewall reads as **present**. Setup appends nothing.
- tailscaled's other `ts-input` rules decide nothing. The loopback ACCEPT is ignored as loopback, the CGNAT `RETURN`/`DROP` rules are scoped by `! -i`, and the magicsock ACCEPT is UDP.
- An unscoped deny in INPUT **before** the jump still decides first (BUG-481). A deny after the jump is shadowed by the chain's ACCEPT.

A jump with a qualifier (`-A INPUT -i eth0 -j ts-input`) is followed by nothing, as today. Its rules apply only to part of the traffic, so evaluating them inline would overstate what they admit.

If INPUT jumps to `ts-input` but `iptables -S ts-input` fails, the read is **unknown**, as when `iptables -S INPUT` fails: setup reports `failed` and appends nothing, check reports `unverified`. The reason names the command.

## Out of scope

- Jumps to any chain other than `ts-input` (docker, fail2ban, operator chains). They still decide nothing.
- IPv6 (`ip6tables`), `--netfilter-mode=nodivert` (chain present, no jump) and tailscaled's nftables mode: #526.
- An unscoped `-j RETURN` inside `ts-input`. tailscaled never writes one, and it clears the chain on restart (`AddChains` runs `ClearChain`), so operator rules there do not persist.
- ufw. tailscaled's rules are not in `ufw show added`, and ufw's own user rules decide as today.

## Risks / open questions

- The rule shape is taken from tailscale.com v1.104.2 (`AddChains`, `AddHooks`, `addBase4`, `AddExternalCGNATRules`, `AddLoopbackRule`, `AddMagicsockPortRule`). Those calls were replayed in that order with iptables 1.8.10 (nf_tables) in `ubuntu:24.04`, and the printed form is recorded on #492. It was not captured from a running tailscaled: installing tailscaled in the container was not available in this session. The replay pins the printed form, not tailscaled's runtime behavior.
- `iptables -S ts-input` prints `-N ts-input` before its `-A` lines. The walk already skips anything that is not an `-A` line.
- Any reader of `iptables -S INPUT` now makes a second read on every host where tailscaled manages netfilter. Both reads need root, and the read already refuses to run unprivileged.

## Acceptance criteria

- [ ] AC1: With INPUT jumping to a `ts-input` that holds `-i tailscale0 -j ACCEPT` and no allow in INPUT, the read is present. Setup reports `ok` and appends nothing, and check reports `ok` with `firewall_ok` true.
- [ ] AC2: The chain's rules decide at the jump's position. An unscoped deny in INPUT before the jump reads as blocked. A deny after the jump is shadowed by the chain's ACCEPT. tailscaled's loopback, CGNAT and magicsock rules alone decide nothing.
- [ ] AC3: `ts-input` is read only for an unqualified `-A INPUT -j ts-input`. Without a jump, or with a qualified jump, no second read runs and the answer is unchanged.
- [ ] AC4: When the jump exists and `iptables -S ts-input` fails, setup reports `failed` without appending, and check reports `unverified`. Both name the command.
- [ ] AC5: Every test of the chain uses the replayed `iptables -S` printed form recorded on #492.

## References

- Issue #492, decisions in its 2026-10-30 comment; follow-up #526.
- `specs/archive/BUG-035-iptables-scoped-allow/`, `specs/archive/BUG-483-iptables-portless-allow/`, `specs/archive/BUG-481-firewall-deny-order/`.
- `tailscale.com@v1.104.2/util/linuxfw/iptables_runner.go`.
- ADR-017 outcome model.
