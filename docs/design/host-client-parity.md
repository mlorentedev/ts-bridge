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
| G1 | Named profiles and YAML `--config`, merged `flags > env > YAML > defaults` (`config.Merge`); a `--profile` takes the YAML file's place (ADR-012) | Closed: `host init`, `setup`, `check` and `serve` take `--config`, the ADR-017 host file, resolved `flags > env > file > per-OS defaults` | #458 (#519, #554, this PR) |
| G2 | `--profile` on `connect`, `init`, `browser` | No `--profile` | Proposed for rejection in ADR-017 (#460): per-user stores resolve to root/Administrator under elevation; replaced by `--config` (#458). Stays open on #450 until #460 merges |
| G3 | `init` writes profiles or YAML; `.env` is the compatibility path being retired (#399) | Closed: `host init --config` writes the ADR-017 YAML host file, never `.env` | #458 |
| G4 | Control-plane awareness: `--control-url`, `config.ValidateControlPlane` (#406) | Closed: the descriptor's `cp=` is the declared `host.control_url`, else the control plane read from `tailscale debug prefs`; when neither is known the descriptor is omitted, never `cp=saas` by default | #453 |
| G5 | `init` refuses to overwrite without confirmation or `--force` (BUG-019) | Closed: `host init` asks before replacing, takes `--force`, and refuses a directory or an unreadable file even with `--force` | #458 |
| G6 | Unreadable `.env` handling is a known defect (#408) | Same defect: an unreadable `.env` is treated as empty and overwritten, dropping every other variable | **#408** (second site) |
| G7 | Logging to console plus a rotating JSON file (`internal/logging`, `LogDirForPlatform`) | stderr only; a command that mutates firewall and registry keeps no persistent record | #457 |
| G8 | Global `-v/--verbose`, consistent `--log-format`, `--json` | Each host subcommand redeclares `--verbose` (no `-v`) and `--log-format`; `host init` has neither `--json` nor `--log-format` | #457 |
| G9 | Every OS branch covered by Go tests and native CI on Windows and macOS (#447) | Per-OS logic reads `runtime.GOOS` / build tags; darwin branch was a stub never exercised | #457 |
| G10 | `connect --bootstrap-ssh` reaches a sinkholed or TLS-inspected control plane (CLI-363) | Native Tailscale only, so a host on such a network cannot enrol | **#522**: `host serve` (persistent tagged node, declared services only, Windows and Linux) |

## Per-OS host state (2026-10-20)

| Capability | Windows | Linux | macOS |
|---|---|---|---|
| Remote-access service | RDP: port read from registry; enabled state not read (W2) | xrdp: `systemctl is-enabled` / `is-active` (the unit, not PATH; `static` counts as enabled, `not-found` is not installed, `disabled` gets `enable --now`, `masked` is `manual` in check and `failed` in setup with the unmask fix, unrecognized answers such as `indirect` are `unverified` in check and `failed` in setup), enabled with `enable --now` (#454, #478). `ssh` (#521) is read the same way, on whichever unit exists (`ssh` first, then `sshd`; an enabled and active `<unit>.socket` stands in for an idle socket-activated service, and on a host whose `<unit>.socket` is enabled (or static), or where neither unit is enabled, the service is not running and the socket is not masked, setup runs `enable --now <unit>.socket` and leaves the service alone (#767), while a host whose service is enabled or running with the socket disabled, or whose socket is masked, keeps `enable --now <unit>`; a masked socket is the operator's opt-out and is never unmasked; a host whose service and socket are both `enabled` (a `static` socket is not) is `manual` in setup and check, whatever their run state (setup starts the socket when neither unit is running, and says so), with the boot-time `systemctl disable <unit>.service` as the fix and nothing disabled automatically, #777), `manual` when the OpenSSH server is missing (nothing is installed) and, for a declared port other than 22, compared with `sshd -T` (`manual` on a mismatch; `sshd_config` is never written); its firewall step reuses the rows below under the name `Firewall configuration (ssh)`, except that an unscoped allow for the ssh port is the operator's, so setup reports it `manual` and never deletes it. `check` prints one line per declared service | **none** (stub) |
| Firewall | Named rule via PowerShell (W3), scoped to `-RemoteAddress 100.64.0.0/10,fd7a:115c:a1e0::/48`; a rule that allows any address is reported as legacy and recreated scoped; the built-in "Remote Desktop" group is no longer enabled, and an enabled group is reported as `manual` (never disabled) because its any-address rules void the scope; the recreate creates the scoped rule before removing the old one (#631, #685) | `ufw allow` / `iptables -A`, after reading whether an existing allow covers the port, scoped and port-less ones included, loopback excluded, in rule order: an unscoped deny before any allow is `manual`, tailscaled's `ts-input` chain is read at its jump's position, and the written rule is scoped to `tailscale0` (`-i` for iptables, `allow in on tailscale0` for ufw); `check` reports any unscoped allow for the port, including one that names no port, excludes only others such as `! --dport 22` (#736) or names only a source port such as `--sport 1024:65535` (#757), or is held only by a negated scope (`! -i tailscale0`, `! -s 100.64.0.0/10`, `! -d ...`) or an any-address literal (`-s 0.0.0.0/0`), which hold nothing to the tailnet, as on IPv6 (#743), but not one held to replies by a conntrack or state match (`--ctstate RELATED,ESTABLISHED`, `! --ctstate NEW`), which admits no new connection and is not the scoped rule either (#758), while a port-less allow held by a state match that admits NEW (`-m conntrack --ctstate NEW`, `! --ctstate INVALID`, `-m state ! --state RELATED,ESTABLISHED`) is reported like any other, on IPv4 and IPv6 alike (#778) (`changed` for setup's own exact pre-#631 rules, which `setup` replaces after the scoped rule is in place; `manual` for any other, which it leaves to the operator) and `firewall_ok` is false until it is gone (#631, ADR-017 Amendment B); beside an unreadable firewall both name them, `setup` as `failed` and `check` as `unverified` (#454, #479, #483, #481, #492, #480, #576, #526); a `ts-input` that INPUT does not jump to (`nodivert`) is `unverified`, not absent, and in tailscaled's nftables mode the read is the same as in iptables mode where `iptables` is the nf_tables backend, and `unverified` naming the mode where it is the legacy backend; `ip6tables -S INPUT`, and every chain it reaches through a jump that applies to all the port's traffic (`ufw6-*` excepted), is read as an exposure scan (#720): an unscoped v6 allow for the port turns an `ok` or `changed` v4 step into `manual` in both `check` and `setup`, naming its `ip6tables -D` fix, with `firewall_ok` false (beside an `unverified` or `failed` v4 step the outcome stays, and only the message gains the v6 rule), and is never deleted, while a failed read of the v6 INPUT only adds a note, a v6 chain INPUT jumps to that cannot be read turns an `ok` step `unverified` as on IPv4 (#763), and a host without `ip6tables` reads as before (#526); on IPv4 the chains INPUT jumps to are walked the same way (`ts-input`, and `ufw-*` on a ufw host, excepted) and scored as INPUT is: an unscoped allow there is reported `manual` with `firewall_ok` false and never deleted, and a chain INPUT jumps to that cannot be read turns an `ok` step `unverified` (#762), as does an existing chain that a jump (in INPUT or a walked chain, v4 and v6) reaches under a match ts-bridge does not model (`-m mark`, `-m limit`, `-m recent`, `-m addrtype`, `-m set`): the exposure is unknown, `firewall_ok` is false, and the note names the rule, while a jump a modelled match narrows (another port or protocol, an interface or prefix, loopback, replies only) or one to a target that is no chain reads as before (#766); on an iptables host (no ufw) whose INPUT policy is `ACCEPT` the scoped rule restricts nothing, so `check` and `setup` turn an `ok` or `changed` step into `manual` naming `iptables -P INPUT DROP` as the operator's fix, `setup` still writes the scoped rule and never changes the policy, and `firewall_ok` is false (#691); an `ip6tables` INPUT policy of `ACCEPT` is reported the same way, naming `ip6tables -P INPUT DROP` with ICMPv6 kept open first (#733); on an active ufw the same two policies are read, since ufw writes them from its incoming default, and the fix names `ufw default deny incoming` and `IPV6=yes` with the SSH allow re-added while ufw is disabled (#735). Where firewalld runs or a native nftables table hooks input (any `inet` table, or an `ip`/`ip6` table that is not iptables-nft's), iptables is not read at all: `check` is `unverified` and `setup` `manual`, both naming that firewall, with its own commands as the fix and no rule written (#739). | not inspected |
| Tailscale | unattended mode + service start | IP only | IP only |
| Power / sleep | disabled unless `--no-sleep` | not handled | not handled |
| Idempotent re-run | not reported (no "already configured" state) | yes: a converged host reports every step `ok`, except a ufw host that still holds leftover copies of setup's old unscoped iptables rule, which stays `manual` until the operator deletes them, an iptables host with an unscoped allow setup did not write, and an iptables host or active ufw host whose INPUT policy (v4 or v6) is ACCEPT, which stays `manual` until the operator closes it (#454, #576, #631, #691, #733, #735; iptables rules last until reboot) | n/a |
| `host setup` / `host check` exit status | setup: 0 ok/changed/manual, 2 a step failed, 1 could not run (#476); check: 0 all ok, 2 not converged, 1 could not run (#471) | same | same (setup prints "not applicable" and exits 0 until #459) |
| `host check` port | registry | config value, not the listener | 0: no service declared until #459 (was a hard-coded 3389) |

### Linux defects found in the audit (#454, fixed)

All four are fixed by #454 (`specs/archive/BUG-034-linux-host-idempotent/` once archived). Kept here as the record of what the audit found.

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
  `tailscale set --unattended` changes only that pref. Fixed: setup reads `ForceDaemon` from
  `tailscale debug prefs` and runs `tailscale set --unattended` only when it is off.
  On a node that has never logged in Tailscale keeps no profile, so the pref is lost on the
  next service restart; setup still sets it and reports `manual` naming `tailscale up` (#740).
- **W2:** `host check` sets `RDPEnabled = rdpPort > 0` and never reads
  `fDenyTSConnections` or its Group Policy copy, so it reports RDP enabled when it is off.
  Fixed: setup and check read both values, the policy value first and an absent value as
  denied; a policy denial is `manual` with the key named, and setup writes nothing.
- **W3:** the firewall step deletes and recreates its rule, so it always changes the
  system and can never report "already configured".
  Fixed: setup and check read the rule first, and setup recreates it only when it is
  absent, duplicated, disabled, not an inbound allow, or on another port.

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
