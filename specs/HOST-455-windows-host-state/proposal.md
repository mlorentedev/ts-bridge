---
id: "HOST-455-windows-host-state"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-04"
issue: "mlorentedev/ts-bridge#455"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# HOST-455-windows-host-state

## Why

`ts-bridge host setup` and `host check` on Windows act without reading state first (#455, found in
the HOST-003 audit, `docs/design/host-client-parity.md`). Setup runs `tailscale up --unattended`.
`tailscale up` refuses on a node that already carries prefs it was not given again, such as a
Headscale `--login-server`, and that is exactly the host ADR-005 targets. Check reports RDP enabled
whenever the port reads, and never reads `fDenyTSConnections`. The firewall step deletes and
recreates its rule on every run, so a converged host can never report `ok`. That breaks the ADR-017
contract that a re-run is a no-op.

## What

Each Windows step reads before it writes, and reports `ok` when nothing needed changing:

- **W1, unattended mode.** Setup reads the node's prefs from `tailscale debug prefs`. The field is
  `ForceDaemon`, which is what `--unattended` sets.
  - `true` → `ok`, nothing run.
  - Otherwise it runs `tailscale set --unattended`, which changes only that pref → `changed`.
  - It never runs `tailscale up`. Prefs output that cannot be read falls through to the `set`, which
    is idempotent.
- **W2, effective RDP state.** One read returns two values:
  - the Group Policy value: `HKLM\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services\fDenyTSConnections`;
  - the local value: `HKLM\SYSTEM\CurrentControlSet\Control\Terminal Server\fDenyTSConnections`.

  The effective value is the policy value when present, else the local value, else `1` (Windows'
  default: denied).
  - Setup:
    - effective `0` → `ok`, no write;
    - policy `1` → `manual`, no write, and the fix names the policy key (a local write would be
      reverted on the next policy refresh);
    - otherwise it writes the local value `0` → `changed`.
  - Check: `services[].enabled` is `effective == 0`. A local denial is `changed`, because setup
    would fix it. A policy denial is `manual`.
- **W3, firewall rule.** Setup reads the named rule's enabled state and local port.
  - Exactly one rule, enabled, on the RDP port → `ok`, nothing run.
  - Absent, disabled, on another port, or duplicated → today's recreate → `changed`.
  - Check uses the same read, so a disabled rule or a rule on the wrong port no longer counts as
    `firewall_ok`.

## Out of scope

- A live Windows run. Fixtures pin the PowerShell output shapes; nobody has observed them on a
  host. Live acceptance is Manu's (see `verification.md`).
- Reporting unattended mode from `host check`. Check never reported it, and adding a row is a
  contract change for #450's follow-ups.
- The other Windows steps (services, network profile, power settings) still act on every run.
- `runHostSetup` length (#588).

## Risks / open questions

- `tailscale debug prefs` is a debug subcommand, so its output is not a stable interface.
  Mitigation: any output that does not parse to `ForceDaemon: true` runs the `set`. Drift therefore
  costs one redundant idempotent command and never skips the step.
- The built-in `Enable-NetFirewallRule -DisplayGroup "Remote Desktop"` call stays in the recreate
  branch only, so a converged host no longer runs it. The named rule already opens the declared
  port, which is what the step promises. This is decided and recorded in `verification.md`.
- A registry read that fails maps to `failed` in setup and in check (today's `actRead`), not to
  `unverified`. `unverified` means "reading needs elevation", and any user can read these HKLM
  values.

## Acceptance criteria

- [ ] AC1: Setup never runs `tailscale up`.
  - With a Headscale-joined prefs fixture (`ControlURL` set, `ForceDaemon: false`), it runs
    `tailscale set --unattended` and reports `changed`.
  - With `ForceDaemon: true`, it runs nothing for that step and reports `ok`.
- [ ] AC2: `services[].enabled` and the setup outcome follow the effective `fDenyTSConnections`
  (policy first, then local, default denied). A policy denial is `manual`, its fix names the policy
  key, and setup writes nothing.
- [ ] AC3: The firewall step reads the rule and acts only when the rule is absent, disabled, on
  another port or duplicated.
  - A converged rule is `ok`, with no `Remove`/`New` call.
  - Check reports `firewall_ok` false for a disabled or wrong-port rule.
- [ ] AC4: All of it is table-tested through the `Runner` seam on the Linux runner, and runs in
  `test-windows`.

## References

- Issue: mlorentedev/ts-bridge#455; audit: `docs/design/host-client-parity.md` (W1-W3)
- ADR-017 (step outcomes `ok/changed/failed/manual/unverified`), ADR-005 (Headscale targets)
