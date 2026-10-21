---
id: "ts-bridge-host-client-parity"
type: design
status: active
tags: [architecture, host, cross-platform, parity, gap-analysis]
created: "2026-10-20"
owner: manu
issue: "mlorentedev/ts-bridge#450"
---

# Host / Client Parity Gap

> Inventory of what the client side (`connect`, `init`, `browser`, `auth`) gained since
> June 2026 that the host side (`host setup`, `host check`, `host init`) never received,
> plus the per-OS state of the host. Input to the HOST-003 ADR (#450). Keep this file
> current as gaps close: each row names the ticket that owns it.

## Why the gap exists

Between 2026-06-20 and 2026-10-20 the client paths took **35 commits** and the host
paths **7**. None of the host commits since CFG-002 (#228, 2026-06-26) added host
behaviour: they were a lint migration, the stderr log fix (#267), command-tree
construction (#290, #293), docs reconciliation (#295) and a shared key-grammar change
(#398). Client work shipped as new ADRs (012, 014, 015, plus 016 decided with #442 and not yet
in `docs/adr/`); none of them listed
`host` among the commands they changed, so nothing prompted a host follow-up.

The host is daemonless by design (ADR-011 C4): `host setup` configures the machine and
exits, and the host runs native Tailscale. That is unchanged. The gaps below are about
the host *command*, not about running ts-bridge on the host.

## Gap inventory: client improvements absent from the host

| # | Client has | Host has | Owner |
|---|---|---|---|
| G1 | Named profiles and YAML `--config`, merged `flags > env > YAML > defaults` (`config.Merge`); a `--profile` takes the YAML file's place (ADR-012) | `.env` + `TS_HOST_*` env only; `--config` on `host setup` is "reserved for future use" | #458 (explicit host config file, proposed in ADR-017, #460) |
| G2 | `--profile` on `connect`, `init`, `browser` | No `--profile` | Proposed for rejection in ADR-017 (#460): per-user stores resolve to root/Administrator under elevation; replaced by `--config` (#458). Stays open on #450 until #460 merges |
| G3 | `init` writes profiles or YAML; `.env` is the compatibility path being retired (#399) | `host init` writes only `.env`, the path #399 deprecates | #458 |
| G4 | Control-plane awareness: `--control-url`, `config.ValidateControlPlane` (#406) | Descriptor always `cp=saas`, even on a Headscale host | **#453** |
| G5 | `init` refuses to overwrite without confirmation or `--force` (BUG-019) | `host init` rewrites `.env` with no confirmation and no `--force` | #458 |
| G6 | Unreadable `.env` handling is a known defect (#408) | Same defect: an unreadable `.env` is treated as empty and overwritten, dropping every other variable | **#408** (second site) |
| G7 | Logging to console plus a rotating JSON file (`internal/logging`, `LogDirForPlatform`) | stderr only; a command that mutates firewall and registry keeps no persistent record | #457 |
| G8 | Global `-v/--verbose`, consistent `--log-format`, `--json` | Each host subcommand redeclares `--verbose` (no `-v`) and `--log-format`; `host init` has neither `--json` nor `--log-format` | #457 |
| G9 | Every OS branch covered by Go tests and native CI on Windows and macOS (#447) | Per-OS logic reads `runtime.GOOS` / build tags; darwin branch was a stub never exercised | #457 |

## Per-OS host state (2026-10-20)

| Capability | Windows | Linux | macOS |
|---|---|---|---|
| Remote-access service | RDP: port read from registry; enabled state not read (W2) | xrdp: detected by binary on `PATH`, not by service state | **none** (stub) |
| Firewall | Named rule via PowerShell (W3) | `ufw` / `iptables` (L1-L3) | not inspected |
| Tailscale | unattended mode + service start | IP only | IP only |
| Power / sleep | disabled unless `--no-sleep` | not handled | not handled |
| Idempotent re-run | not reported (no "already configured" state) | **no** (see L1) | n/a |
| `host check` port | registry | config value, not the listener | hard-coded 3389 |

### Linux defects found in the audit (#454)

- **L1:** `iptables -A INPUT ... -j ACCEPT` runs without a prior `-C` check, so every
  re-run appends a duplicate rule.
- **L2:** `ufw route allow ...` adds a rule for routed (forwarded) traffic, not for a
  local service (`ufw(8)`); opening xrdp needs `ufw allow`. The ufw path itself does
  not duplicate rules.
- **L3:** `checkFirewallRule` matches the port as a substring of `ufw status` /
  `iptables -L` output, so `3389` also matches `33890`, and a `route` rule passes.
- **L4:** xrdp readiness is `exec.LookPath("xrdp")`: proof the binary is installed, not
  that `systemctl is-enabled` / `is-active` hold.

### Windows defects found in the audit (#455)

- **W1:** `tailscale up --unattended` refuses on a node whose existing prefs it was not
  given again (a Headscale `--login-server`), so setup fails on ADR-005 hosts.
  `tailscale set --unattended` changes only that pref.
- **W2:** `host check` sets `RDPEnabled = rdpPort > 0` and never reads
  `fDenyTSConnections` or its Group Policy copy, so it reports RDP enabled when it is off.
- **W3:** the firewall step deletes and recreates its rule, so it always changes the
  system and can never report "already configured".

### macOS facts that bound the design

- Enabling Remote Login (`systemsetup -setremotelogin on`) needs root **and** Full Disk
  Access; reading it without admin fails. Without root, the state is the launchd
  override for `com.openssh.sshd` in `launchctl print-disabled system` when one exists,
  and otherwise the `Disabled` key of `/System/Library/LaunchDaemons/ssh.plist`, which
  ships `true`. Absence from the override list therefore means **off**, never on
  (observed on macOS 27: override `enabled`, plist `Disabled = true`, port 22 open).
- Screen Sharing cannot be enabled from the command line since macOS 12.1 (Apple: only
  MDM); `kickstart` is view-only since 10.14. Setup must report a manual action with a
  System Settings deep link, not attempt it.
- The application firewall (`socketfilterfw`) is per application, with no port rules;
  Apple-signed sshd and Screen Sharing pass when "allow signed" is on. Only "block all"
  needs a step.

## Contract constraints for the redesign

- `host check --json` is a public contract (`tailscale_ip`, `rdp_port`, `rdp_enabled`,
  `firewall_ok`, `tailscale_up`, `platform`, `descriptor`), asserted by
  `scripts/tests/smoke.bats`. Changes must be additive or versioned.
- The `tsb://` descriptor ignores unknown query parameters, so a service kind can be
  added without breaking existing clients.
- Installing ts-bridge as an OS service (the systemd unit lost in #307) is client-side
  bridge lifecycle, not host setup. It belongs with the session supervisor in #451.

## Related

- #450 HOST-003 (parity epic; decision in ADR-017, #460), #457, #458, #459, #453, #454, #455, #408, #307, #399, #451.
- Reference audit (Ansible, RustDesk, Tailscale CLI, vendor docs): maintainer knowledge store, `prestudy/host-parity-reference-audit-matrix.md`.
- [ADR-011](../adr/adr-011-shareable-connection-profile.md) (daemonless host, descriptor),
  [ADR-012](../adr/adr-012-config-profiles-model.md),
  ADR-016 (self-contained profile connections; not yet in `docs/adr/`, lands with #442).
