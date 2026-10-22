---
id: "BUG-034-linux-host-idempotent"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-10-22"
issue: "mlorentedev/ts-bridge#454"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# BUG-034-linux-host-idempotent

> **Naming**: file lives at `<repo>/specs/BUG-034-linux-host-idempotent/proposal.md`. `BUG-034-linux-host-idempotent` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #454: BUG-034: Linux host setup is not idempotent and opens the wrong firewall chain -->

`ts-bridge host setup` on Linux acts without reading first. Every run appends another `iptables` ACCEPT rule (L1). The ufw path adds a `route` rule, which covers forwarded traffic, not the local xrdp listener (L2). `host check` then accepts any listing that contains the port as a substring, so a `route` rule or port `33890` reads as "allowed" (L3). xrdp counts as ready when its binary is on `PATH`, whether or not the service runs (L4). ADR-017 (row "Linux" of the per-OS table) already decides the fix: state-first steps, ufw `allow`, iptables `-C` before `-A`, and `systemctl enable --now`.

## What

- **Elevation first.** Every Linux firewall read needs root: `ufw show added`, `ufw status` and `iptables -C` all refuse without it (measured below). The `Runner` reports elevation. Without root, the firewall step runs no command and reports `unverified` in check and `failed` in setup, each with a fix that names root.
- **Firewall, one backend.** Use ufw when it is on `PATH`, otherwise iptables. Setup and check share one read, so check is exactly the read half of setup.
  - **ufw:** read `ufw show added`, which lists the configured user rules whether or not ufw is active. A rule counts when it is an `allow` on exactly `<port>/tcp`, `<port>`, or `port <port>` (with `proto tcp` or no proto), interface-scoped (`in on tailscale0`) included. It does not count when it is `out`, `route`, `deny`/`reject`/`limit`, or a different port. If no rule counts, setup runs `ufw allow <port>/tcp`.
  - **iptables:** run `iptables -C INPUT -p tcp --dport <port> -j ACCEPT` as root. Exit 0 means present. Exit 1 means absent, and only then does setup run `-A`. Any other result means the state is unknown.
- **xrdp, service state.** `systemctl is-enabled xrdp` and `systemctl is-active xrdp` decide the step. Neither needs root.
  - Not installed: `failed`.
  - `systemctl` missing (a non-systemd host): `unverified` in check, `failed` in setup.
  - Enabled and active: `ok`.
  - Anything else: setup runs `systemctl enable --now xrdp` and reports `changed`; check reports `changed`.
- **Services and legacy keys.** `services[].enabled` and `services[].active` come from those reads, so `active` stops being `null` on Linux wherever `systemctl` answers; where it cannot be read, `active` stays `null` and `enabled` false, and the xrdp step reports `unverified`. As a result, the legacy `rdp_enabled` changes meaning on Linux. It used to mean "xrdp binary on `PATH`" and now means `is-enabled == enabled`, so a host with xrdp installed but disabled flips from `true` to `false`. ADR-017 §3 derives the legacy keys from the services, so this follows the rule. For the same reason, `rdp_port` reads 0 while xrdp is disabled, as it already did when the xrdp binary was missing. An `unverified` firewall leaves `firewall_ok` false (ADR-017 §3).
- **Idempotence.** A second `host setup` on a converged host reports every step `ok` and runs no mutating command.

Measured on Ubuntu 24.04 (ufw 0.36.2, iptables 1.8.10 nf_tables), 2026-10-22:
- `ufw show added` lists `ufw allow 3389/tcp` while `ufw status` says `Status: inactive`.
- A second `ufw allow 3389/tcp` prints `Skipping adding existing rule`.
- `iptables -C` exits 0 for a present rule and 1 for an absent one. Without root it exits 4 on this backend.
- Without root, `ufw` exits 1 ("You need to be root").

## Out of scope

- firewalld and nftables-only hosts, which have neither ufw nor iptables. They keep failing with the existing fix text.
- An SSH service on Linux, and the explicit `--config` host file (#458).
- The `port 0` text rendering from #475; this PR closes only #475's partial-read half.
- Persistence of the iptables rule across reboot. `iptables -A` lasts until the next boot; after a reboot, setup re-adds the rule and reports `changed`. Saving rules (`netfilter-persistent`, `iptables-save`) is the operator's. ufw rules persist on their own.
- Enabling ufw itself. A configured rule on an inactive ufw counts as present.

## Risks / open questions

- Older iptables backends (legacy) may exit 1 without root, which would read as "absent". The elevation gate runs first, so `-C` never runs unprivileged.
- The `Runner` returns errors, so the iptables exit status comes from any error that exposes `ExitCode()` (`*exec.ExitError` does). An error without one means the state is unknown.

## Acceptance criteria

- [ ] AC1: Each Linux setup step reads its state first and acts only on a difference. A second run on a converged host reports every step `ok` and runs no mutating command (the only `run` left is the read-only `iptables -C`).
- [ ] AC2: The ufw path adds an INPUT `allow <port>/tcp` rule, never `route`. The iptables path runs `-C` before `-A` and never runs `-A` when `-C` exits 0.
- [ ] AC3: The firewall read matches the exact port and an inbound rule. `33890` does not satisfy `3389`, and a `route`/`FWD` rule does not count.
- [ ] AC4: xrdp readiness uses `systemctl is-enabled` and `is-active` through the injected `Runner`. Every branch (not installed, no systemctl, enabled+active, disabled, inactive) is table-tested on any OS.
- [ ] AC5: A read that cannot be done (not root, no backend, an unexpected exit status, no systemctl) yields `unverified` in check and `failed` in setup, never `ok` or `changed`, and runs no mutating command.

## References

- Issue #454; epic #450; ADR-017 (per-OS table, Linux row); `docs/design/host-client-parity.md` L1 to L4
- #473 (unverified on an unreadable read), #475 (partial read, closed here in part)
