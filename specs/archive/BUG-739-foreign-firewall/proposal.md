---
id: "BUG-739-foreign-firewall"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-11-12"
issue: "mlorentedev/ts-bridge#739"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-739-foreign-firewall

> **Naming**: file lives at `<repo>/specs/BUG-739-foreign-firewall/proposal.md`. `BUG-739-foreign-firewall` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

The Linux firewall read knows two backends, ufw and raw iptables, and reads any host without ufw as raw iptables. On a host filtered by firewalld (Fedora, RHEL) or by a native nftables ruleset, `iptables -S INPUT` shows `-P INPUT ACCEPT` over an empty chain while the real filtering happens in another table. Since #691 and #733 the step then reports `manual`, "every port is open", with `firewall_ok` false, on a host that may be closed; and setup appends an iptables rule that admits nothing past the other firewall. Probed on an Ubuntu 24.04 guest: with firewalld running, `nft list chains` shows `table inet firewalld` with a `hook input` chain, and `iptables -S INPUT` still prints `-P INPUT ACCEPT`.

## What

Before choosing ufw or iptables, the read looks for a firewall it does not read:

1. **firewalld**: `firewall-cmd --state` prints `running` (exit 0). Not running prints `not running` (exit 252), and an absent binary skips the probe.
2. **A native nftables ruleset**: `nft list chains` shows a base chain with `hook input` in a table that is not one of iptables-nft's own. iptables-nft's tables are family `ip` or `ip6`, named `filter`, `nat`, `mangle`, `raw` or `security`. tailscaled's nftables mode and ufw on iptables-nft live in those tables, so they never trigger. A table with no input hook (NAT only, output only) does not filter inbound traffic and does not trigger. firewalld's nftables backend also appears here as `inet firewalld`; the firewalld probe runs first, so it names firewalld.

When one is found, the step names it and reads nothing else, since iptables is not authoritative there:

- **`check`**: `unverified`, naming the firewall, and `firewall_ok` false.
- **`setup`**: `manual`, naming the firewall. It writes no rule: an iptables rule would admit nothing past the other firewall, and ts-bridge does not write firewalld or nftables rules (issue step 3). ADR-017 gives setup no `unverified` outcome, and `manual` is its "operator must act, fix names the action" outcome.
- **The fix names the tool's own commands**:
  - firewalld: a zone of its own for `tailscale0`, then the port in that zone, then `firewall-cmd --reload`. Probed on the guest.
  - nftables: `nft insert rule <family> <table> <chain> iifname "tailscale0" tcp dport <port> accept`, then persist it in the distribution's ruleset file.
- **Neither policy read is applied**, v4 or v6, and the ip6tables exposure scan is skipped. The v6 read is skipped entirely: it would otherwise claim "ip6tables admits TCP N on every interface" where firewalld decides (the #739 comment from #733).

## Out of scope

- Writing firewalld or nftables rules (issue step 3; a separate decision).
- Reading firewalld's zones or a native nft ruleset to decide whether the port is admitted. `check` stays `unverified` on such a host even after the operator opens the port.
- A firewalld e2e case: it needs a package install on the guest. The firewalld path is unit-tested and probed by hand with the built binary on a guest; the native nft case is in e2e.

## Risks / open questions

- **A failed `nft list chains`** (nft on PATH, the listing fails) is not "nothing found". The read is unknown: the step reports that the ruleset could not be read, through the existing unknown path, and setup writes nothing. Existing fixtures that put `nft` on PATH must answer the new listing. A fixture that does not answer it shows up as a failing test, never as a silent pass.
- **A failed `firewall-cmd --state`** with an exit other than 252 is unknown in the same way. Exit 252 with "not running" is not running.
- **Debian's stock `/etc/nftables.conf`** (`table inet filter`, input hook, policy accept, no rules), when the nftables service is enabled, turns the step `unverified`/`manual` even though it filters nothing. This is conservative and named in the message, so the operator can tell. The same holds for another tool's table with an input hook, such as LXD's or Incus's `inet lxd`/`inet incus`.
- **`policyRead`, `linuxFirewallOK` and the message builders** must not read a foreign-firewall read as raw iptables. The backend stays empty, so `policyRead` is false. `linuxFirewallOK` is false because the state is not present, and a test pins it.
- **The new messages** must not contain the raw-iptables runbook keys ("the iptables INPUT policy is ACCEPT", "the ip6tables INPUT policy is ACCEPT"), like the ufw ones (#748).

## Acceptance criteria

- [x] AC1: firewalld running: `check` is `unverified` naming firewalld, `setup` is `manual` naming firewalld and writes nothing, `firewall_ok` is false, and neither the v4 nor the v6 policy note appears.
- [x] AC2: a native nft table with an input hook (`inet filter`): the same, naming the table. An nft ruleset holding only iptables-nft or tailscaled tables, or a table with no input hook, changes nothing.
- [x] AC3: a failed firewalld or nft probe is an unknown read: `check` is `unverified`, `setup` writes nothing.
- [x] AC4: current iptables and ufw behaviour is unchanged: the existing host tests pass, with fixtures extended only to answer the new probes.
- [x] AC5: e2e: on the Linux guest, an `inet` table with an input hook (policy accept) gives `check` `unverified` naming nftables and setup `manual` with no rule written. Deleting the table gives `ok` again.

## References

- Issue #739, and its comment from #733 (gate the v6 read too)
- ADR-017 (`docs/adr/adr-017-host-desired-state.md`): outcome table, Amendment B firewall lines
- #691, #733, #735: the policy reads this gates
