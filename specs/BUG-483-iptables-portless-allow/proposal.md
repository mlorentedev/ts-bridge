---
id: "BUG-483-iptables-portless-allow"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-10-22"
issue: "mlorentedev/ts-bridge#483"
tags: [spec, proposal, host, linux, firewall]
template_version: "1.0"
---

# BUG-483-iptables-portless-allow

## Why

On an iptables host, BUG-035 counts an ACCEPT only when it names the destination port. That rule exists so that `-m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT` is not taken for an allow. The same exclusion also drops the blanket allows operators write to trust the tailnet (`-i tailscale0 -j ACCEPT`, `-s 100.64.0.0/10 -j ACCEPT`). `host setup` then appends the unrestricted `--dport <port>` rule next to them and widens RDP past the scope they chose. The ufw backend already reads the equivalent rules (`ufw allow in on tailscale0`, `ufw allow from 100.64.0.0/10`) as present, so the two backends still disagree on the most common hardened shape (#483).

## What

**Loopback never counts.** Any ACCEPT with a non-negated `-i lo`, or a non-negated `-s` or `-d` whose prefix is a loopback address (`127.0.0.0/8`, `::1/128`), reads as absent, with or without a port. Practically every hand-written ruleset contains an `lo` allow. Counting it would report `ok` with RDP still closed on a `-P INPUT DROP` host.

A non-loopback rule in `iptables -S INPUT` that carries **no port flag** (`--dport`, `--dports`, `--ports`, `--sport`, `--sports`) covers every TCP port only when **every** qualifier after the chain is one the parser models as leaving new TCP connections open:

- `-j ACCEPT`;
- `-p tcp`; no `-p` at all also qualifies (`-p all` prints none);
- `-m tcp` and `-m comment --comment <text>`;
- `-i <iface>`, `-s <prefix>` and `-d <prefix>`;
- `--tcp-flags FIN,SYN,RST,ACK SYN`, which is what `--syn` prints;
- a negated loopback scope: `! -i lo`, or `! -s`/`! -d` with a loopback prefix.

Any other token makes the rule read as absent. That covers other matches (`conntrack`, `state`, `limit`, `recent`, ...), `-f`, every other `--tcp-flags` set (`ACK`, `SYN,ACK`, another mask), any other negation (`! -p tcp`, `! -s 100.64.0.0/10`, `! -i tailscale0`) and non-TCP protocols. The parser lists what it accepts instead of what it rejects, so an unmodeled shape falls to absent, the visible failure.

Interface, source and destination qualifiers other than loopback are the operator's scope, exactly as BUG-035 treats them on port-bearing rules. Rules with a port flag otherwise keep the BUG-035 logic.

## Out of scope

- tailscaled's own `ts-input` chain, which holds `-i tailscale0 -j ACCEPT` on every host where tailscaled manages netfilter. `-S INPUT` does not read it. Tracked as #492.
- Rule order and shadowing (#481).
- Duplicate rules (#480), ip6tables, nftables-only and firewalld hosts.
- The ufw backend. `ufw show added` lists only the rules the operator added, and ufw keeps its loopback allow in `before.rules`, so the loopback trap does not arise there.

## Risks / open questions

- Measured on iptables 1.8.10 (nf_tables), `ubuntu:24.04`, with rules added by `iptables -A` and read with `-S INPUT`:
  - `-i tailscale0 -j ACCEPT`, `-s 100.64.0.0/10 -j ACCEPT` and `-p tcp -j ACCEPT` print as typed. `-j ACCEPT` alone and `-p all -j ACCEPT` both print `-A INPUT -j ACCEPT`.
  - Addresses gain a prefix length: `-d 127.0.0.1` prints `-d 127.0.0.1/32`, and ip6tables prints `-s ::1/128`.
  - `--syn` prints `-p tcp -m tcp --tcp-flags FIN,SYN,RST,ACK SYN`, which counts because it admits new connections. `--tcp-flags FIN,SYN,RST,ACK ACK` (established traffic only), `... SYN,ACK` and `--tcp-flags SYN,RST SYN` print as typed after `-p tcp -m tcp`.
  - `-f` prints `-A INPUT -f -j ACCEPT`; negations print before their flag (`! -s 100.64.0.0/10`, `! -i tailscale0`).
  - A single-word comment loses its quotes: `-m comment --comment tailnet`.
  - `-m recent --rcheck` prints extra options (`--mask 255.255.255.255 --rsource`). The module rule excludes it regardless.
- A false `present` leaves RDP closed and silent, since setup reports `ok`. A false `absent` reproduces today's widening. The loopback exclusion and the token allow-list both err toward `absent`.
- `-i eth0 -j ACCEPT` on a tailnet-only host reads as present. That is the same "operator's scope" judgment BUG-035 made for port-bearing rules.

## Acceptance criteria

- [ ] AC1: A port-less ACCEPT that is blanket, interface-scoped, source-scoped, negated-loopback (`! -i lo`, `! -s 127.0.0.0/8`), `-p tcp`, `--syn` (`--tcp-flags FIN,SYN,RST,ACK SYN`) or comment-qualified reads as present. Setup reports `ok` and appends nothing; check reports `ok`.
- [ ] AC2: A port-less ACCEPT reads as absent, and setup appends the rule as before, when it is: loopback (`-i lo`, `-s 127.0.0.0/8`, `-d 127.0.0.1/32`, `-s ::1/128`); non-TCP (`-p udp`, `-p icmp`, `! -p tcp`); narrowed by another match (`conntrack`, `state`, `limit`, `recent`); limited to fragments (`-f`); limited by a `--tcp-flags` set other than `--syn` (`ACK`, `SYN,ACK`, another mask); or negated on a non-loopback scope (`! -s 100.64.0.0/10`, `! -i tailscale0`).
- [ ] AC3: Every BUG-035 `TestIptablesCovers` case keeps its answer. A port-bearing ACCEPT scoped to loopback (`-i lo ... --dport 3389`, `-s 127.0.0.0/8 ... --dport 3389`) now reads as absent; BUG-035 had no case for it.

## References

- Issue #483. `specs/archive/BUG-035-iptables-scoped-allow/` holds the parser this widens and the review finding that filed #483.
- #492 (ts-input), #481 (shadowing), #480 (duplicates).
- ADR-017 per-OS table, Linux row.
