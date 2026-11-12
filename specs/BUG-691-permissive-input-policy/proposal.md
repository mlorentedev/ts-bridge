---
id: "BUG-691-permissive-input-policy"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-11-11"
issue: "mlorentedev/ts-bridge#691"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-691-permissive-input-policy

> **Naming**: file lives at `<repo>/specs/BUG-691-permissive-input-policy/proposal.md`. `BUG-691-permissive-input-policy` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

On an iptables host with `-P INPUT ACCEPT` and no ufw, every port is open on every interface whatever rule setup writes, so the `-i tailscale0` scope from #631 restricts nothing. `check` still reports the firewall converged and `firewall_ok` true: the tool tells the operator the port is tailnet-only when it is not (#691, ADR-017 Amendment B records the assumption). The owner decided on 2026-11-12 that this is reported as `manual`.

## What

`host check` and `host setup` read the INPUT chain policy from the `iptables -S INPUT` output they already fetch (no new command). When the backend is iptables (ufw not installed) and the policy is ACCEPT, a firewall step that would be `ok` or `changed` becomes `manual`; its message says the policy admits every port so the scope restricts nothing, its fix names enabling a firewall or setting a DROP policy after allowing SSH, and `firewall_ok` is false. Setup still adds the scoped rule (it is needed once the policy is DROP) and never changes the policy.

## Out of scope

- The IPv6 policy (`ip6tables -P INPUT ACCEPT`): same gap, separate ticket.
- Hosts with ufw installed: an inactive ufw is already `manual` (#608); an active one sets a DROP-equivalent policy.
- Changing the policy from setup: a DROP without an SSH allow can lock the operator out.

## Risks / open questions

- Every default-ACCEPT iptables host stops being `ok`. That is the intended, honest outcome; the runbook and parity doc say so.
- The e2e Linux iptables section and the v6 cases from #714 assume ACCEPT; the guest moves to a DROP policy (loopback, established and SSH allowed first) before them, and one case asserts `manual` under ACCEPT.
- A step that is already `unverified`, `failed` or blocked keeps its outcome; only the message gains the policy note.

## Acceptance criteria

- [ ] [AC1] iptables backend, policy ACCEPT, scoped rule present: `check` and `setup` report `manual`, the message names the ACCEPT policy, the fix names a DROP policy after allowing SSH or enabling a firewall, and `firewall_ok` is false.
- [ ] [AC2] iptables backend, policy ACCEPT, rule absent: `setup` adds the scoped rule and reports `manual`, not `changed`; no command changes the policy.
- [ ] [AC3] Policy DROP, or the ufw backend, or a step already `unverified`/`failed`: outcomes are unchanged from before this change.
- [ ] [AC4] e2e on the Linux VM: a default-ACCEPT iptables guest reports `manual` with `firewall_ok` false; under DROP the existing iptables cases pass as before.

## References

- Issue #691 and the owner's decision comment; ADR-017 Amendment B; #631 (scope), #608 (inactive ufw), #526 (v6 exposure, `manual` precedent)
