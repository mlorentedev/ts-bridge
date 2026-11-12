---
id: "BUG-735-ufw-incoming-allow"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-11-12"
issue: "mlorentedev/ts-bridge#735"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-735-ufw-incoming-allow

## Why

#691 and #733 report an INPUT policy of ACCEPT, v4 and v6, but only on iptables hosts: `withPolicy`, `withV6Policy` and `linuxFirewallOK` all gate on `backend == backendIPT`. An active ufw with `ufw default allow incoming` (or `DEFAULT_INPUT_POLICY="ACCEPT"`) leaves both kernel policies at ACCEPT, so the `tailscale0` scope restricts nothing and `host check` still says `ok` with `firewall_ok` true (#735). A ufw host whose v6 policy is ACCEPT while v4 is DROP (ufw with `IPV6=no` after a boot) has the same blindness on IPv6 (PR-Agent on #746, recorded on #735).

## What

- On an active ufw, the firewall read keeps the kernel's v4 INPUT policy (`iptables -S INPUT`, which `readUFW` already runs) and the v6 one (`ip6tables -S INPUT`, already read since #733), instead of discarding them.
- The policy promotion of #691/#733 applies to an active ufw as well as to raw iptables: an `ok` or `changed` step becomes `manual`; a `failed` or `unverified` step gains only the note; `firewall_ok` is false. Setup never changes a default or a policy.
- The fix on a ufw host names ufw's own commands, not raw `-P`, which ufw overwrites on its next reload: v4 `ufw allow <your SSH port>/tcp` first, then `ufw default deny incoming`; v6 `IPV6=yes` in `/etc/default/ufw` and `ufw reload`, keeping `ufw default deny incoming`.
- An inactive ufw is unchanged: it already reports `manual` naming `ufw enable`, and its ACCEPT policies are that same fact.

Deviation from the issue's proposed fix: the issue suggests parsing `Default: allow (incoming)` from `ufw status verbose`. Probed on the Linux e2e guest (ufw 0.36.2), ufw writes the kernel policies from its incoming default: `deny` gives `-P INPUT DROP` on both families, `allow` gives `-P INPUT ACCEPT` on both, and inactive gives ACCEPT. Reading the kernel policy needs no new process, reports what the kernel enforces rather than what ufw says it configured, and covers the `IPV6=no` case, where ufw leaves the v6 policy untouched.

## Out of scope

- firewalld and native nftables detection (#739).
- Following a v6 jump into ufw's or tailscaled's chains (#720).
- Changing any ufw default or kernel policy from `setup`.

## Risks / open questions

- A ufw host without the `iptables` binary: `readUFW` skips the iptables read, so the v4 policy is unknown and nothing changes. ufw depends on iptables, so this is a broken install, not a mode.
- A failing `iptables -S INPUT` on an active ufw (xtables lock contention, a broken backend) while ufw's own rules decide the state: the policy is unknown, so the step is `unverified` with the read's error, and `firewall_ok` is false. It is never reported safer than a raw iptables host on the same failure (spec review round 1, F3).
- With `IPV6=no`, the v6 policy is whatever was there before ufw ran: DROP if an earlier ufw run set it, ACCEPT after a boot. The read reports the kernel as it is, so the outcome can differ across a reboot. That is the truth of the host, not a flake.

## Acceptance criteria

- [x] [AC1] An active ufw whose v4 INPUT policy is ACCEPT: `check` and `setup` report `manual`, the message names the policy, the fix names `ufw default deny incoming` after allowing SSH, never `iptables -P`, and `firewall_ok` is false.
- [x] [AC2] An active ufw with v4 DROP and v6 ACCEPT: `manual`, the fix names `IPV6=yes` and `ufw reload`, `firewall_ok` is false.
- [x] [AC3] An active ufw with both DROP, an inactive ufw, and an unreadable ufw keep their outcome as before; a `failed` or `unverified` step gains only the note; `Success` is false on every `manual` rewrite. An active ufw whose `iptables -S INPUT` fails while its own allow decides is `unverified`, naming the error, with `firewall_ok` false.
- [x] [AC5] On a ufw host the message names ufw's incoming default (and `IPV6=no` for v6), not "unless another firewall filters this host".
- [x] [AC4] e2e on the Linux guest: under `ufw default allow incoming`, `check` reads `manual` naming `ufw default deny incoming` and not `firewall_ok`; after `ufw default deny incoming`, `check` is `ok` again.

## References

- Issue: #735 (and its comment on the `IPV6=no` case)
- #691, #733 (the iptables-host policy reads this extends)
- ADR-017 Amendment B (`docs/adr/adr-017-host-desired-state.md`)
