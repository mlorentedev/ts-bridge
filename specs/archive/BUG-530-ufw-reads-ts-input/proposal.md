---
id: "BUG-530-ufw-reads-ts-input"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-11-02"
issue: "mlorentedev/ts-bridge#530"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, linux, firewall]
template_version: "1.0"
---

# BUG-530-ufw-reads-ts-input

## Why

On a host that runs ufw, `readLinuxFirewall` reads only `ufw show added`. tailscaled writes `-A INPUT -j ts-input` and `-A ts-input -i tailscale0 -j ACCEPT` straight into iptables, and ufw never lists them, so the read reports the port as absent. `host setup` then runs `ufw allow <port>/tcp`, which admits the port on every interface. The tailnet was already allowed, so the rule adds no reachability and only widens exposure (#530; BUG-492 review finding F6). The same blind spot hides copies of the pre-#477 iptables rule on hosts that installed ufw after an earlier setup: they also open the port on every interface, and `host check` cannot report them (#530, comment from the BUG-480 review).

## What

On a ufw host, the firewall read keeps `ufw show added` as its first source and adds the iptables read that #492 built (INPUT, plus `ts-input` behind tailscaled's unqualified jump):

- ufw reports the port allowed or blocked → that answer stands, as today.
- ufw reports it absent → the iptables read decides. If it reports the port present (tailscaled's `-i tailscale0` ACCEPT, or an earlier ACCEPT in INPUT), the step is `ok` and setup writes nothing. If it reports it absent, setup runs `ufw allow <port>/tcp` exactly as today.
- The iptables read fails, or reaches a qualified jump into `ts-input` that it cannot evaluate → the state is unknown: `host check` reports `unverified` and `host setup` writes nothing (the #532 rule: never open on a guess).
- Copies of setup's own iptables rule are counted on ufw hosts too, and **one or more** is reported as `manual`. On a ufw host setup never writes that rule, so every copy is a leftover from an iptables-era run that opens the port on every interface. The fix text therefore asks for all of them to be deleted, not all but one as the iptables-only BUG-480 step does. (Decided by the owner on 2026-11-03, when the debt was folded in.)

The ufw chains (`-j ufw-before-input`, ...) are jumps, which the walk already treats as deciding nothing. The captured output (verification.md) shows tailscaled's jump at the top of INPUT ahead of them, whether ufw started first or reloaded after tailscaled.

## Out of scope

- **Option 1 from #530: making the write tailnet-scoped** (`ufw allow in on tailscale0 ...`, `-i tailscale0` for iptables). It changes ADR-017's documented setup action, so it waits for an owner decision. When nothing covers the port, a ufw host still receives the unscoped `ufw allow <port>/tcp`, as today. That is a deliberate non-goal of this spec, not an oversight.
- #526's modes: IPv6 (`ip6tables`), `--netfilter-mode=nodivert` and tailscaled in nftables mode.
- Deleting or rewriting any rule. Setup never deletes firewall rules (BUG-480).

## Risks / open questions

- Decided: a single pre-#477 copy on a ufw host is reported (`manual`, delete every copy) instead of reading as `ok`. See `## What`.
- Fixture provenance: the output comes from ufw 0.36.2 and iptables v1.8.10 (nf_tables) in an Alpine 3.20 container, with tailscaled's `ts-input` replayed from BUG-492's recorded chain. The Ubuntu 24.04 image could not install packages on this host (mirror signature failure), but the ufw and iptables versions match the existing Ubuntu fixtures.
- Two reads instead of one on ufw hosts. Both already need root, and the iptables binary is a dependency of ufw.

## Acceptance criteria

- [ ] AC1: On a ufw host whose `ts-input` accepts `tailscale0` and whose `ufw show added` does not list the port, `host check` reports the firewall `ok` and `host setup` runs no `ufw allow`. A table test drives the verbatim ufw and iptables output.
- [ ] AC2: On a ufw host with one or two copies of setup's iptables rule in INPUT, `host check` reports them as `manual`, and the fix deletes every copy. The test uses the verbatim INPUT from the capture. On an iptables-only host, one copy stays `ok` and two stay the BUG-480 step (regression).
- [ ] AC3: On a ufw host whose iptables read fails, or which reaches a qualified jump into `ts-input` with nothing decided, `host check` is `unverified` and `host setup` writes nothing.
- [ ] AC4: On a ufw host where neither source covers the port, behaviour is unchanged: `host setup` runs `ufw allow <port>/tcp`, and a ufw allow or deny still decides first.
- [ ] AC5: Each new path fails under its mutation: skipping the iptables read fails AC1, and treating the read error as absent fails AC3.

## References

- Issue #530; BUG-492 spec review F6 (`specs/archive/BUG-492-ts-input-chain/review.md`)
- ADR-017 (declarative host setup), unchanged by this spec; option 1 would amend it
- Related: #526 (other tailscaled modes), BUG-480 (copies), #532 (qualified jumps)
