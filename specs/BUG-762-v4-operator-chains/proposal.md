---
id: "BUG-762-v4-operator-chains"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-13"
issue: "mlorentedev/ts-bridge#762"
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-762-v4-operator-chains

## Why

The IPv4 exposure read scores `-A INPUT` lines only (`iptUnscoped`). The only jump it follows is to tailscaled's `ts-input` (#492). If INPUT jumps to a chain of the operator's own, the read misses an unscoped allow inside that chain, even though the allow opens the port on every interface:

```
-P INPUT DROP
-A INPUT -j fw-open
-A fw-open -p tcp -m tcp --dport 3389 -j ACCEPT
```

On this host `host check` does not report the exposure. The same is true of a port-less `-A fw-open -j ACCEPT`, which opens every port. The IPv6 scan follows these jumps since #720 (#752), and it reports port-less allows since #753 (#742). IPv4 does neither.

## What

The v6 walk (`ip6Walk`) becomes `iptWalk`, which takes the tool, the chains to skip, and the exposure predicate for each chain. The v6 read calls it as before.

The v4 read (`iptablesRead`) also calls it, walking all of `iptables -S INPUT`:

- **Jumps followed.** It follows the same jumps as v6 (`iptJump`): unnarrowed jumps whose match takes every new TCP connection on the port. The walk is transitive and reads each chain once, as on v6. A chain reached through a chain admits the same traffic as one reached from INPUT, so one level would miss the same shape one jump deeper.
- **Chains skipped.**
  - `ts-input`, by name. It is inlined for the decision already. Under the `iptUnscopedRead` doctrine, an unscoped accept inside it would count as the scoped rule.
  - Every `ufw-` chain, when the read is through ufw. `ufw show added` already reports ufw's own allows, with a fix ufw keeps.
- **Exposure in a chain.** A rule counts when it passes the predicate v4 INPUT uses: `iptExposes`, not loopback, and no scope by `iptScopedAllow` (#743 will change that predicate for both). INPUT itself stays scored by `iptUnscoped`, with setup's legacy copy filtered out. Only the lines from other chains are added to `foreign`. `foreign` then flows through `legacyHeld`, `legacyCheckStep` and `foreignFix` unchanged: `manual`, `firewall_ok` false, a fix that names the `-A <chain> ...` rule, and the SSH caution for a port-less one. setup deletes none of them.
- **An unreadable chain.** INPUT jumps to it, but `iptables -S <chain>` fails for any reason other than "no chain by that name". That chain reads as unknown, not as clear, as #762 asks. A `firewall_ok` true step becomes `unverified`, and its message names the failed read. A missing chain is still skipped silently (`chainMissing`), as on v6.

The decision (`iptablesDecide`) is unchanged. The `TestCheckForLinux` row "iptables, a tailnet accept in an operator chain is no rule for the port: changed (#762)" pins today's behaviour. On a host whose only tailnet accept is `-A fw-open -i tailscale0 ... --dport 3389 -j ACCEPT`, check reports "No firewall rule found", and setup appends its own scoped rule to INPUT. The result is a harmless duplicate, never an open port.

## Out of scope

- **Following operator chains in `iptablesDecide`.** It would remove the duplicate described above. It is a separate change, and a riskier one, because setup would then trust a chain it does not own.
- **The v6 read.** Its walk only notes a chain it cannot read and keeps `firewall_ok` true. That is filed as #763 so the two reads converge. This spec does not change v6 behaviour.
- **Rule order.** As on v6 and in INPUT, an allow behind an unscoped deny is still listed.

## Risks / open questions

- A host whose INPUT jumps to many chains (Docker does not, but kube-proxy's `KUBE-FIREWALL` does) now runs one `iptables -S` per chain. The walk reads each chain once.
- An operator chain holding a deliberate unscoped allow now turns `manual`. That is the intent: the scope is void there.
- The walk scores a chain whichever way it is reached, including from a jump with a match the parser does not model. `iptJump` fails toward "not followed": an unmodelled qualifier narrows the jump.

## Acceptance criteria

- [ ] [AC1] An unscoped port allow, and a port-less `-j ACCEPT`, in a chain INPUT jumps to are reported by `check` and `setup` as `manual`, naming the `-A <chain>` rule, with `firewall_ok` false. setup deletes neither.
- [ ] [AC2] Not reported:
  - a scoped (`-i tailscale0`) allow in the chain;
  - an allow behind a narrowed jump (`-i eth0 -j fw-open`);
  - `ts-input`;
  - ufw's chains under ufw.
- [ ] [AC2] A chain reached through another chain is reported. A jump loop ends.
- [ ] [AC3] A chain INPUT jumps to that cannot be read makes a `firewall_ok` true step `unverified`. A missing chain does not.
- [ ] [AC4] e2e on the Linux VM, under a DROP policy: an unscoped allow in a jumped chain is reported `manual` with `firewall_ok` false, and reads `ok` once removed.

## References

- #762, #720 (#752), #753 (#742), #492, #743.
