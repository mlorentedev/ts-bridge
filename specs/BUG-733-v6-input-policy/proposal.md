---
id: "BUG-733-v6-input-policy"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-12"
issue: "mlorentedev/ts-bridge#733"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-733-v6-input-policy

## Why

#691 reports an iptables host (no ufw) whose v4 `INPUT` policy is `ACCEPT` as `manual`, because the `tailscale0` scope restricts nothing there. The v6 policy is not read. A host with `-P INPUT DROP` on v4 and `-P INPUT ACCEPT` on v6 exposes the service port on every IPv6 address, while `host check` reads `ok` with `firewall_ok: true`. The v6 read from #526 only scans for an unscoped allow for the port, and on a default-ACCEPT v6 chain no allow is needed. The default `ip6tables` policy is `ACCEPT`, and tailscaled leaves it so (the captured chain in the tests is `-P INPUT ACCEPT`), so this is the common case, not an edge.

## What

- `readIP6` also reports whether the v6 listing's policy line is `-P INPUT ACCEPT`. It uses the same first-line rule as #691's `iptPolicyAccept`, so a multi-line comment cannot forge it.
- On the iptables backend, a v6 `ACCEPT` policy adds a note to the step. An `ok` or `changed` step becomes `manual`; the fix names `ip6tables -P INPUT DROP`, preceded by the loopback, replies and SSH rules to add first, or "confirm another firewall filters this host, or that IPv6 is disabled". An `unverified` or `failed` step keeps its outcome and fix, and only the message gains the note, as #691 does.
- `firewall_ok` is false while the v6 policy is `ACCEPT` on an iptables host.
- Setup never changes the v6 policy.
- Unchanged:
  - A failed v6 read only adds a note, and a host without `ip6tables` reads as before (#526).
  - On a ufw host the v6 policy is not used: ufw sets its own v6 default when enabled, and an inactive ufw is already reported (#655), as #691 does for v4.

## Out of scope

- A v6 `ACCEPT` policy on a host whose IPv6 is disabled reads as `manual`. The fix names that case for the operator to confirm. Reading `disable_ipv6` is a separate change.
- nftables and firewalld detection (#739).

## Risks / open questions

- Most iptables hosts without a v6 firewall turn `manual`. Intended: their port is open on IPv6.
- The e2e guest's v6 policy is `ACCEPT`, so the existing e2e cases must close it the way they close v4 (loopback, replies, port 22 first).

## Acceptance criteria

- [x] [AC1] v4 `DROP` with v6 `ACCEPT` on an iptables host: `check` and `setup` report `manual`, the message names the ip6tables policy, the fix names `ip6tables -P INPUT DROP` with the keep-open rules, `firewall_ok` is false, and setup runs no `-P`.
- [x] [AC2] v6 `DROP`, a failed v6 read, no `ip6tables`, a ufw backend, or an `unverified` v4 step: the outcome is as before. A failed or unverified step gains only the note.
- [x] [AC3] v4 and v6 both `ACCEPT`: one `manual` step whose fix names both policies.
- [x] [AC4] e2e on the Linux guest: with v4 closed and v6 `ACCEPT`, check reads `manual` and `firewall_ok` false; after the guest closes v6 (keep-open rules first), check is `ok` again.

## References

- #733, #691, #526; ADR-017 Amendment B.
