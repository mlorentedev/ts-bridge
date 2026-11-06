---
id: "SEC-631-scoped-firewall-rules"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-06"
issue: "mlorentedev/ts-bridge#631"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, security, host]
template_version: "1.0"
owner: manu
---

# SEC-631-scoped-firewall-rules

## Why

`host setup` opens the service port on every interface whenever its firewall read reports the
rule absent (#530 named the root cause and said it needs an ADR-017 decision):

- **iptables.** `iptablesAppend` writes `-A INPUT -p tcp --dport <port> -j ACCEPT`, with no `-i`.
- **ufw.** The rule is `ufw allow <port>/tcp`, with no interface.
- **Windows.** `New-NetFirewallRule ... -Profile Any` has no `RemoteAddress`, and the same command
  runs `Enable-NetFirewallRule -DisplayGroup "Remote Desktop"`, which turns on Microsoft's own
  RDP rules, also for any remote address.

A host that joins the tailnet to be reached over it is reachable from its LAN, and from the
internet where it has a public address, on the RDP port. The mesh is the only path the operator
meant to open.

## What

The owner decided the scope on #631 (2026-11-06); this spec implements it and records it as
ADR-017 Amendment B.

- **Linux, scope by interface.** iptables: `-A INPUT -i tailscale0 -p tcp --dport <port> -j ACCEPT`.
  ufw: `ufw allow in on tailscale0 to any port <port> proto tcp`.
- **Windows, scope by tailnet ranges** (no stable interface name):
  `-RemoteAddress 100.64.0.0/10,fd7a:115c:a1e0::/48`.
- **`check` reports a legacy unscoped rule.** An allow for the port with no scope (no interface,
  source or destination address) is reported, `firewall_ok` is `false`, and `setup` replaces it
  with the scoped rule.

### Replacement is two tiers

ADR-017 §3 and the code say "setup does not delete firewall rules". "Setup replaces the legacy
rule" narrows that, and the narrowing is as small as the decision allows:

1. **Setup deletes only the exact rule earlier releases wrote**: `ufw allow <port>/tcp` as
   `ufw show added` prints it, and `-A INPUT -p tcp -m tcp --dport <port> -j ACCEPT` field for field
   as `iptables -S` prints it. The scoped rule goes in first; the delete follows, so the port is
   never closed in between. A failed add deletes nothing; a failed delete is `failed`, with the
   delete command as the fix.
2. **Any other unscoped allow that names the port** (`ufw allow 22,3389/tcp`, an operator's
   `-m multiport`, a rule with a comment) is reported as legacy unscoped and stays `manual`:
   setup cannot know it is its own. It does not count as the scoped rule, so setup still adds one.

Windows needs no tier: the rule is recreated by name, so recreating it scoped is already the
replacement. The order matches the other OSes: the scoped rule is created first (under the same
display name, which Windows allows to repeat) and the rules that were there are removed after, so
RDP is never left without an allow rule. A failed create is terminating and removes nothing.
The old rules are identified before the create, by the objects `Get-NetFirewallRule` returned
(their `Name`, the unique key), and removed through the pipeline; removing by display name would
delete the new rule too, since it repeats the name. `check` therefore expects exactly one rule of
that name after a recreate. Removing first would lock out an operator running `host setup` over the RDP session it is
configuring.

### Windows: the built-in "Remote Desktop" group

Decision: **setup stops enabling `-DisplayGroup "Remote Desktop"`**, does not disable it, and
`check` and `setup` report an enabled group as `manual`. Why:

- Those rules allow any remote address, so enabling them voids the scope the named rule just set.
  Setup's own rule opens the port by itself, so RDP keeps working without them.
- **Upgrade impact.** Earlier releases ran `Enable-NetFirewallRule -DisplayGroup "Remote Desktop"`
  on every recreate, so on an upgraded host the group is usually enabled *by setup*, not by the
  operator. Stopping the enabling does not turn it off: the group's any-address rules keep inbound
  RDP open to the LAN and void the new scope until someone disables it. The fix is therefore not
  complete on those hosts without the report below, and the report is part of #631, not a later
  ticket (#685 is folded in).
- **Setup still does not disable the group.** It cannot tell whether setup or the operator
  enabled it, and the group is Microsoft's own: the operator may use it for another profile.
  Instead `check` reports it (`manual`, `firewall_ok` false) with the exact command as the fix:
  `Disable-NetFirewallRule -DisplayGroup "Remote Desktop"`. Disabling it automatically, with a
  flag or when the group's rules are provably setup's, is a larger decision and stays an option
  for the owner (see Risks).

## Out of scope

- IPv6 on the iptables backend: setup never wrote `ip6tables` rules (#526 tracks it).
- A tailscaled started with `--tun=<name>`: the interface name is fixed at `tailscale0`. A custom
  name makes the scoped rule match nothing. That fails closed only under a restrictive policy (INPUT
  default DROP, or active ufw): the port is then unreachable until the operator allows it by hand.
  On a host with `iptables -P INPUT ACCEPT` and no other firewall the scope restricts nothing, the
  port stays open as it was before this change, and `check` still reports the rule it expects as
  satisfied; `check` reads the rule text, not the policy or the interface. Reporting a permissive
  policy is #691, an owner decision (it would turn every default-ACCEPT host `manual`).
- macOS: `socketfilterfw` is per application and has no port rules (§5).
- `ssh` on Linux and Windows, which is refused today (#521; `internal/host/file.go` lists it as
  planned). Windows setup therefore never enables OpenSSH Server and its built-in
  `OpenSSH-Server-In-TCP` rule (any address, all profiles); when #521 lands it must scope that rule
  or report it `manual` the same way as the RDP group.

## Risks / open questions

- **Printed form drift.** The legacy detector compares the rule field for field against what
  `iptables -S` prints. If `iptablesAppend` and its printed form disagree, every `setup` re-adds
  the rule (the #480 failure). Mitigation: the e2e case runs setup twice on a real iptables host
  and requires the second run to report `ok`.
- **ufw with an interface that does not exist.** A fresh guest has no `tailscale0`. ufw is
  expected to accept the rule anyway; the e2e guest has no Tailscale, so the first run proves it.
  If ufw refuses, that is a blocker for the owner, not something to work around.
- **Windows address normalisation.** The read must treat `RemoteAddress` as a set: the order and
  the textual form Windows returns for the two ranges are not verified here. The Windows guest is
  shared with another session, so PR 4 says its e2e is pending.
- **Windows CGNAT residual.** The Windows scope is a range, not an interface, so a local network
  addressed inside `100.64.0.0/10` (managed or carrier-style networks) is admitted as if it were
  the tailnet. Linux does not have this residual, which is why Linux scopes by interface. Accepted:
  Windows has no stable interface name to match (the alias is localised and changes with the
  adapter), and the range is what Tailscale itself assigns from. Operators on such a LAN need an
  additional block rule or a different scope.
- **Windows upgrade, group left enabled.** See the group decision: the report is the mitigation,
  and the group stays on until the operator runs the printed command. An automatic disable is an
  open option for the owner.
- **Windows ordering.** The create-before-remove order and the terminating-error behaviour are
  unit-tested as command text, not run on a host (the Windows e2e is pending).
- **ufw inactive.** Setup still adds the scoped allow "for when ufw is enabled" and replaces ufw's
  own legacy allow, but deletes no iptables copies, which are the only live rules (PR 3).
- **Decision forks taken conservatively** (the owner decision does not cover them): the two tiers
  above; the Windows group not disabled but reported; `firewall_ok` is `false` while any unscoped allow remains;
  an unscoped allow with no port in it (`-A INPUT -p tcp -j ACCEPT`) is the operator's posture, not
  "a rule for the port", and is neither reported nor replaced.

## Acceptance criteria

- [ ] AC1: ADR-017 Amendment B records the scope (hybrid), the two-tier replacement and the
  Windows group decision.
- [ ] AC2: On an iptables host, setup appends the scoped rule, and a second setup reports `ok`.
- [ ] AC3: On a ufw host, setup writes `ufw allow in on tailscale0 to any port <port> proto tcp`,
  and a second setup reports `ok`.
- [ ] AC4: A legacy unscoped rule is reported by `check` (`firewall_ok` false, message names the
  rule); `setup` replaces the exact legacy forms and reports the others `manual`.
- [ ] AC5: On Windows, the rule carries the tailnet ranges, `check` reports a rule with
  `RemoteAddress Any` as differing, setup no longer enables the built-in group, and `check` reports an enabled group as `manual`
  (`firewall_ok` false).
- [ ] AC6: The e2e harness covers both OSes (Linux on a real VM; Windows case added, run pending).

## References

- Issue #631; #530, #576, #480, #526 (firewall read history); ADR-017 §5 and Amendment B.
- `internal/host/linux_steps.go`, `internal/host/windows_steps.go`, `scripts/e2e/linux.sh`,
  `scripts/e2e/windows.ps1`.
