---
tags: [spec, verification]
created: "2026-11-13"
id: "BUG-762-v4-operator-chains"
type: spec-verification
status: active
owner: manu
---

# Verification - BUG-762-v4-operator-chains

## Acceptance criteria

- **AC1.** Covered by these `TestCheckForLinux` rows:
  - "an unscoped allow in an operator chain: manual";
  - "a port-less accept in an operator chain: manual, SSH caution";
  - "an allow two jumps deep, through a loop: manual".

  It is also covered by the `TestSetupForLinux` row "an unscoped allow in an operator chain: scoped rule added, nothing deleted, manual". In that row the recorded calls hold no `-D`.
- **AC2.**
  - The rows "a scoped allow in an operator chain: ok" and "an operator chain behind a narrowed jump is not read: ok" cover it. In the second, no `iptables -S fw-open` call is recorded.
  - `ts-input` is not walked: every existing iptables row records exactly one `iptables -S ts-input`, and removing the skip turns 83 tests red.
  - ufw's chains are not walked: the ufw host rows record no `iptables -S ufw-*`, and removing the skip turns 22 tests red.
  - The loop row ends, through `fw-open` → `fw-deep` → `fw-open`.
- **AC3.** Covered by the rows "an operator chain that cannot be read: unverified" (check), "the scoped rule in place and a chain that cannot be read: unverified" (setup) and "a jump to a missing chain: ok". Each of these is red if `chainUnread` is dropped from `linuxFirewallOK` or `withChains` is short-circuited.
- **AC4.** `make e2e-linux` on c4d8077 gave 135 pass, 0 fail. The code is the same as in 9414418; the commits after it change docs and the spec only. The v4 jumped-chain cases all passed:
  - a tailnet accept is ok;
  - an unscoped allow is `manual`, its message names `-A tsbe2e4 -p tcp -m tcp --dport 3389 -j ACCEPT`, and `firewall_ok` is false;
  - setup reports it and leaves it;
  - check is ok again once it is removed.

## Commands

- `go test -race ./...`: ok.
- `golangci-lint run` (darwin, linux and windows): 0 issues.
- `make e2e-linux`: 135 pass, 0 fail. `make e2e-clean` reported no session left.

## Decision side

`iptablesDecide` is unchanged. The row "a tailnet accept in an operator chain is no rule for the port: changed (#762)" pins today's behaviour, as the proposal states.

## Promotion candidates

None.
