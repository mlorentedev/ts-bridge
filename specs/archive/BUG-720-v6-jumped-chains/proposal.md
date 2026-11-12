---
id: "BUG-720-v6-jumped-chains"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-11-12"
issue: "mlorentedev/ts-bridge#720"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# BUG-720-v6-jumped-chains

## Why

The IPv6 exposure scan added for #526 (#714) reads only `ip6tables -S INPUT` and scores `-A INPUT` lines. An unscoped allow for the service port inside a chain INPUT jumps to (tailscaled's `ts-input`, or an operator's own chain) opens the port off the tailnet over IPv6, and `host check` still reports `firewall_ok`. The IPv4 read already follows tailscaled's jump into `ts-input` (#492); the v6 scan does not. Found by PR-Agent on #714 (THEORETICAL); no default host is affected.

## What

- `readIP6` follows every `-A INPUT` rule whose `-j`/`-g` target is a chain, reads it with `ip6tables -S <chain>`, and scores its `-A <chain>` rules with the same exposure test as INPUT (`ip6Exposed`). It recurses into the chains those jump to, once per chain (a visited set ends `A -> B -> A`).
- A jump is followed only when every packet for the port can take it: the jump rule's match applies to TCP on the port (`iptMatchesPort`), it is not held to part of the traffic by a plain `-i`, `-s` or `-d` (`ip6Narrowed`), and it is not loopback. An unscoped allow behind a narrowed jump admits only that scope, so it is not exposure on every interface.
- A found rule is reported as INPUT's are: check and setup are `manual`, and the fix names `ip6tables -D <chain> ...`, which `withIP6` already builds from the printed `-A <chain>` line.
- A target that is not a chain (`LOG`, `MARK`, any extension) answers `No chain/target/match by that name` and is skipped. The verdicts ACCEPT, DROP, REJECT, RETURN, QUEUE and NFQUEUE are never read. Any other failure to read a chain becomes a v6 note naming the chain; what INPUT and the other chains showed is kept.

The v6 read stays an over-reporting exposure scan, not the v4 decision walk: it ignores rule order (an allow behind a deny is still listed), and it does not stop at a qualified jump, which the v4 walk treats as an unknown read (#532). That distinction is chosen here, not missed: the v6 read decides nothing setup writes.

## Out of scope

- ufw's v6 chains (`ufw6-*`) are not followed. ufw writes them, and `ufw show added` already reports an allow there (one ufw rule covers both families) with a `ufw delete` fix that holds; an `ip6tables -D` would be undone on ufw's next reload.
- A jump qualified by anything but a scope or a TCP port match (`-m conntrack`, `-m set`) is not followed, as `iptMatchesPort` reads such a qualifier as narrowing the rule.
- Rule order inside a followed chain (`RETURN` before the allow, a deny before it). Over-reporting a dead rule is the scan's existing trade-off.
- The v4 read, which already follows `ts-input` (#492).

## Risks / open questions

- Fixture provenance: tailscaled's v6 `ts-input` is copied from `scripts/e2e/capture-tsfw.sh` run against the local Headscale on 2026-11-12 (verification.md records the log), and cross-checked with `addBase6` in tailscale.com `util/linuxfw/iptables_runner.go`.
- Each followed jump costs one more `ip6tables -S` call. A host has a handful of chains; the visited set bounds a loop.

## Acceptance criteria

- [x] AC1: A `-A ts-input -p tcp -m tcp --dport 3389 -j ACCEPT` rule with no interface or source scope, behind `-A INPUT -j ts-input`, reads `manual` in check and setup, and the fix names `ip6tables -D ts-input -p tcp -m tcp --dport 3389 -j ACCEPT`.
- [x] AC2: tailscaled's captured v6 `ts-input` (`-A ts-input -i tailscale0 -j ACCEPT`) still reads `ok`, and the read asks `ip6tables -S ts-input` for it.
- [x] AC3: An unscoped allow inside a chain that INPUT reaches only through a narrowed jump (`-i eth0`, or a port match that excludes the port) reads `ok`; one in a chain two jumps deep reads `manual`; a jump loop ends.
- [x] AC4: A `-j LOG` target is skipped without a note; a chain whose read fails gives a v6 note naming it, and an unscoped allow in INPUT is still reported.
- [x] AC5: `make e2e-linux` passes with a case that puts an unscoped 3389 allow in a chain INPUT jumps to: check is `manual` naming `ip6tables -D <chain>`, setup leaves it, and check is `ok` once it is removed.

## References

- Issue #720; #526, #714 (the v6 scan), #492 (the v4 jump into ts-input), #532.
