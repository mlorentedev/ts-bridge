---
id: "BUG-035-iptables-scoped-allow"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-10-22"
issue: "mlorentedev/ts-bridge#479"
tags: [spec, proposal, host, linux, firewall]
template_version: "1.0"
---

# BUG-035-iptables-scoped-allow

## Why

On an iptables host, Linux `host setup` decides whether the port is allowed with the exact form `iptables -C INPUT -p tcp --dport <port> -j ACCEPT`. An allow the operator already scoped (`-i tailscale0`, `-s 100.64.0.0/10`, `multiport`, a range) does not match that form, so setup appends an **unrestricted** ACCEPT and widens RDP past the scope the operator chose. The ufw path already counts the equivalent scoped rule as present, so the two backends answer "is the port allowed" differently (#479). The same issue records the ufw half: an allow scoped by source only (`ufw allow from 100.64.0.0/10`) covers the port but reads as absent.

## What

Both backends answer one question the same way: **does an existing inbound allow cover TCP `<port>`?** The operator's scope (interface, source) is theirs; `check` does not judge whether it admits Tailscale traffic.

- **iptables: read `iptables -S INPUT`, not `-C`.** One read, parsed per rule in chain order. A rule **covers** the port when its target is `-j ACCEPT`, its protocol is `tcp`, and a non-negated `--dport`, `--dports` or `--ports` contains the port (single, comma list, range; `portListHas`). Its other qualifiers (`-i`, `-s`, `! -i`, `-m conntrack`, `-m comment`) are the operator's scope and do not matter.
  - Any covering rule: present (`ok`), setup appends nothing.
  - None: absent, setup appends `iptables -A INPUT -p tcp --dport <port> -j ACCEPT` once, as today.
  - Not covering: rules without a destination port (`-m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT` would otherwise count), `--sport`, a negated port (`! --dport`, `! --dports`), `udp`, another port, any target other than ACCEPT, and jumps (`-j ts-input`, which tailscaled adds to every host it manages).
- **ufw: an allow with no destination port covers every port.** `ufw allow from 100.64.0.0/10`, `ufw allow in on tailscale0` and `... proto tcp` count as present. A source port (`from any port 1234`), `proto udp` or an application profile (`app <name>`) still does not. The short form keeps its meaning: a non-numeric word (an app profile such as `OpenSSH`) does not count.
- `-S` exits non-zero without root on nf_tables, like `-C`; the elevation gate from BUG-034 already runs first. Any non-zero exit is unknown.

## Out of scope

- Duplicate rules left by the pre-#477 code: #480, which builds on this `-S` read.
- Appending a Tailscale-scoped rule (`-i tailscale0`) instead of the unrestricted one. That changes the ADR-017 Linux row and needs its own decision.
- Rules inside user chains are invisible to this read, as they were to `-C`. Following jumps is not attempted.
- Rule order and shadowing. A DROP/REJECT (iptables) or `deny`/`reject` (ufw) before the allow still reads `ok` on both backends, as it does today. Deciding which scoped deny shadows which scoped allow is a check-quality question, tracked separately (#481).
- The chain policy (`-P INPUT DROP`) does not change the answer.
- ip6tables, nftables-only and firewalld hosts (unchanged from BUG-034).
- xrdp probe issues (#478).

## Risks / open questions

- `-S` normalizes what the operator typed. Measured on iptables 1.8.10 (nf_tables), `ubuntu:24.04`:
  - `--dport 3389` prints as `-p tcp -m tcp --dport 3389`; matches may come before `--dport` (`-m conntrack --ctstate NEW -m tcp --dport 3389`), so the parser looks flags up by name, never by position.
  - `multiport` prints `--dports 22,3389` and `--dports 3380:3390,22`; a single range prints `--dport 3380:3390`.
  - Comments keep their quotes and escape inner ones: `--comment "say \"hi\" --dport 3389 -j ACCEPT"`. The parser removes the comment value before splitting fields, so a quoted word cannot pose as a flag.
  - A negation precedes its flag: `! -i lo`, `-m tcp ! --dport 3389`, `-m multiport ! --dports 22,3389`. `--ports 3389` (source or destination) prints as typed. A jump prints `-A INPUT -j mychain`.
  - tailscaled's iptables mode creates `ts-input` and jumps to it from `INPUT` (`tailscale.com/util/linuxfw`), so a jump is normal on the hosts this targets.
- The ufw backend is preferred when ufw is on `PATH`, so a ufw host (whose `INPUT` jumps to `ufw-before-input`) never reaches the `-S` read.
- `-S` prints `iptables-save` rule syntax on both backends; the legacy backend was not measured.
- ufw folds `from X to any` into `from X` ("Skipping adding existing rule"), so both spellings are one line in `ufw show added`.

## Acceptance criteria

- [ ] AC1: On an iptables host, an existing ACCEPT for TCP `<port>` that is interface-, source-, negation-, `multiport`-, range- or comment-qualified reads as present: setup reports `ok` and appends nothing; check reports `ok`.
- [ ] AC2: With no covering rule, setup appends the unrestricted rule once (`changed`) and a second run reports `ok`; a jump in `INPUT` (`-j ts-input`) does not change that.
- [ ] AC3: Rules that do not cover the port never count: another port (`33890`), `udp`, `--sport <port>`, `! --dport <port>`, `! --dports` containing it, a target other than ACCEPT, a comment whose text spells a covering rule, and rules without a destination port (`-m conntrack ... -j ACCEPT`).
- [ ] AC4: On a ufw host, an allow with no destination port (`from 100.64.0.0/10`, `in on tailscale0`, with `proto tcp` or none) reads as present; with `proto udp`, an `app` profile, or only a source port it does not.
- [ ] AC5: The iptables read is `iptables -S INPUT` through `Output`; `iptables -A` is the only mutating command, and a non-zero `-S` exit is `unverified` in check and `failed` in setup.

## References

- Issue #479 and its comment on the ufw source-only allow; follow-up #480.
- `specs/archive/BUG-034-linux-host-idempotent/` (the read this replaces), lesson 042.
- ADR-017 per-OS table, Linux row.
