---
id: "adr-017"
type: adr
status: accepted
date: "2026-10-20"
issue: "mlorentedev/ts-bridge#450"
tags: [architecture, host, cross-platform, idempotence, contract]
owner: manu
created: "2026-10-20"
---

# ADR-017: Declarative, Daemonless Host Setup with Parity on Linux, macOS and Windows

## Status

Accepted by the operator on 2026-10-20. Implementation is tracked by #450 and its
sub-tickets. Amendment A below (HOST-008, #522) was accepted by the operator on 2026-11-01 in #525.

## Context

The client (`connect`, `init`, `browser`, `auth`) took 35 commits between June and
October 2026; the host commands took 7, none adding behaviour since CFG-002. The result is
inventoried in [`docs/design/host-client-parity.md`](../design/host-client-parity.md):
nine client capabilities the host never received (G1-G9), defects on Linux (#454) and
Windows (#455), a descriptor that always claims Tailscale SaaS (#453), and a macOS stub.

The operator requires host and client to work on Linux, macOS and Windows. Two facts make
the current model unable to get there by patching:

1. **A host can expose more than one remote-access service.** macOS offers Remote Login
   (SSH) and Screen Sharing (VNC); Linux may offer xrdp and SSH. `CheckResult` has one
   `RDPPort` and one `RDPEnabled`.
2. **`SetupStep.Success` is a bool.** It cannot say "already configured", so the
   idempotence rule (a second run changes nothing) cannot be verified on any OS. Windows
   recreates its firewall rule on every run; Linux appends a duplicate iptables rule.

### Constraints

| ID | Constraint | Origin |
| --- | --- | --- |
| C1 | Daemonless host: `host setup` configures the machine and exits | ADR-011 C4 |
| C2 | `host check --json` evolves additively; existing keys stay | ADR-013, `smoke.bats` |
| C3 | A second `host setup` reports no change | IaC doctrine |
| C4 | No check reports a state it did not verify | Verification doctrine |
| C5 | The descriptor carries the host's real control plane | ADR-005, #453 |
| C6 | No new dependencies | AGENTS.md |
| C7 | Every OS branch is testable on the Linux runner and confirmed natively | Lesson 040 |
| C8 | Nothing the OS reserves for user consent is automated; it is reported with a fix | Apple docs, RustDesk |
| C9 | Elevation changes the user: per-user stores resolve to root/Administrator | `internal/config/statedir.go` |
| C10 | macOS ceiling: Screen Sharing cannot be enabled by CLI since 12.1 (MDM only); Remote Login needs root and Full Disk Access | Apple Remote Desktop guide, `man systemsetup` |

### Reference audit

| Reference | Relevant evidence | Pattern taken |
| --- | --- | --- |
| Ansible modules (`ufw`, `firewalld`, `systemd_service`, `osx_defaults`) | Read state, compare, act on difference; `ok/changed/failed/skipped` with `diff` | State-first steps with tri-state outcome |
| RustDesk platform code | Queries macOS TCC state; missing consent becomes a "Configure" action deep-linking to System Settings | A distinct manual outcome carrying a fix |
| Tailscale CLI | Build-tagged OS files; `goos` injected in tests; `status --json` grows by adding fields and keeping deprecated ones | OS seams testable anywhere; additive JSON |
| Vendor docs (Apple, Microsoft, systemd, ufw, iptables) | Facts behind C8-C10, #454 and #455 | Bounds of what a CLI may automate |

The full matrix and divergence log live in the maintainer's knowledge store
(`prestudy/host-parity-reference-audit-matrix.md`).

## Options Considered

| Option | Door | C2 | C3 | C4 | C8 | C10 | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A: patch each OS, keep RDP-shaped fields | Type 2 | yes | no | no | no | no | Cannot model a multi-service host |
| B: generic step/outcome and service model | Type 1 (additive contract) | yes | yes | yes | yes | yes | Desired state stays implicit in code |
| **C: B plus a declared desired state that `setup` converges and `check` diffs** | Type 1 | yes | yes | yes | yes | yes | Chosen |
| D: delegate to Ansible or `kardianos/service` | Type 1 | n/a | yes | n/a | n/a | n/a | New dependency, breaks the single binary |
| E: macOS as client only | Type 2 | yes | n/a | n/a | n/a | no | Contradicts the operator requirement |

## Decision

Choose **C**.

### 1. Desired state is declared

A host configuration file, selected with an explicit `--config <path>`, declares what the
host should look like:

```yaml
host:
  services:
    - kind: rdp        # rdp | ssh | vnc
      port: 3389
  firewall:
    rule: Tailscale-RDP-Ingress
  power:
    prevent_sleep: true
  # control_url: https://headscale.example   # optional override of the detected control plane (C5)
```

Without a file, the declared state is the per-OS default: Windows `rdp`, Linux `rdp`
(xrdp), macOS `ssh`. `TS_HOST_*` variables and a `.env` in the working directory keep
working as a compatibility layer with the existing precedence
(`flags > env > file > defaults`). `host init` writes this file instead of `.env`.

The host configuration is **machine-scoped and explicit**. Host commands do not read the
per-user profile or credential store: under elevation those resolve to the elevated
user's directories (C9), and the host needs no credential.

The file is validated against the target OS before any step runs. A declared entry the OS
has no step for (a `kind` outside its row in §5, or `power` where §5 lists no power step)
is a configuration error naming the entry and the OS; it is never skipped, so `check`
cannot report a host converged while part of its declared state was ignored.

### 2. `setup` converges, `check` diffs, using the same steps

Each OS contributes an ordered list of steps. Every step reads the current state first,
compares it with the declared state, and acts only on a difference. A step reports one
outcome:

| Outcome | `setup` | `check` |
| --- | --- | --- |
| `ok` | already in the declared state; nothing done | in the declared state |
| `changed` | the system was changed to match | would change (drift) |
| `failed` | could not converge; error carries why and fix | could not read or verify |
| `manual` | the OS reserves this for the user (C8); fix names the action | same |
| `unverified` | n/a | state needs elevation to read; fix says how (C4) |

`check` runs the read half of every step and never mutates. `setup` requires elevation;
`check` does not and degrades to `unverified` instead of guessing. A second `setup` on a
converged host reports only `ok` (C3). `check` exits zero only when every step is `ok`.
Exit status, as `terraform plan -detailed-exitcode`: 0 every step `ok`, 1 the check could
not run, 2 it ran and at least one step is not `ok`. Before HOST-004 PR B2, `check` exited
0 whatever it found; `host check && ...` changes meaning from that release.

`setup` reports its own failures, not the host's convergence (amended 2026-10-22, #476):
0 every step `ok`, `changed` or `manual`; 2 it ran and at least one step is `failed`; 1 it
could not run (not elevated, bad config). `manual` exits 0 because setup has printed the
action the OS leaves to the user (C8); a non-zero status there would make `host setup &&`
never chain on macOS once it has steps (#459), since Screen Sharing is always `manual`.
Until then macOS `setup` prints "not applicable" and exits 0. Only `failed` exits 2: setup
has no `unverified` outcome (table above), and a step that ever reported one would exit 0,
as `manual` does. Whether the host converged
is `check`'s answer, so "configure, then verify" is `sudo ts-bridge host setup && ts-bridge
host check`. Before this amendment `setup` exited 0 even when a step failed.

### 3. Services, not RDP fields

`host check --json` gains a `services` array
(`kind`, `port`, `enabled`, `active`, `firewall_ok`) and `steps[].outcome` / `steps[].fix`.
The existing keys stay and are derived (C2): `rdp_port` / `rdp_enabled` from the `rdp`
service (zero/false when absent), `firewall_ok` true only when at least one service is
declared and every declared service has `firewall_ok == true` (an `unverified` firewall
read counts as false). `steps[].success` stays as `outcome in {ok, changed}`.

### 4. One descriptor per service, carrying the real control plane

The host emits one `tsb://` descriptor per declared service, adding a `svc=<kind>` query
parameter. Clients already ignore unknown parameters (`TestParse_ToleratesUnknownQueryParams`
in `internal/profile`), so older clients keep importing the
target and port, and the profile store does not keep `svc`: to an older client every
descriptor is a plain TCP target. That is all the bridge needs (it forwards bytes), so
`ssh` and `vnc` descriptors work through any client; only a client that reads `svc` can
launch the matching viewer. The control plane is detected from the local Tailscale and overridable
by `control_url`. An absent key means "detect"; an empty `control_url` is a configuration
error, so unset and empty cannot be confused (#282). An undetectable control plane is reported, never silently encoded as
SaaS (C5, #453). The legacy single `descriptor` key carries the first declared service.

### 5. OS boundaries

| OS | Services | Automated | Verified and guided only |
| --- | --- | --- | --- |
| Windows | `rdp` (+ `ssh` via OpenSSH Server when declared) | RDP enable honouring Group Policy precedence, firewall rule (state-first), unattended Tailscale via `tailscale set`, power | none beyond policy-managed settings |
| Linux | `rdp` (xrdp), `ssh` | `systemctl enable --now`, firewall INPUT rule (ufw `allow` / iptables `-C` then `-A`) | none |
| macOS | `ssh` (Remote Login), `vnc` (Screen Sharing) | Remote Login when elevated and Full Disk Access is granted | Screen Sharing; Remote Login without Full Disk Access; application-firewall "block all" |

On macOS, `check` reads Remote Login without root from the launchd override for
`com.openssh.sshd` and, when no override exists, from the `Disabled` key of its launch
daemon plist (shipped `true`), so a missing override reads as off.

### 6. Testability

OS steps run behind injected seams (command runner, registry reader, file reader) and the
step list is built from an OS argument, not a `runtime.GOOS` read (lesson 040). Every OS's
steps are table-tested on the Linux runner; `test-windows` and `test-macos` confirm them.

## Rationale

The ok/changed/failed model is the one intersection every audited reference shares, and
it is the only way the idempotence rule becomes checkable. A declared state is what turns
`check` from a list of probes into a drift report that answers "does this host match what
was asked", and it is what a future control plane (#451) or GUI can read and write without
a second schema. Keeping it machine-scoped and explicit avoids the elevation-user trap
instead of hiding it behind `SUDO_USER` resolution, which has no clean Windows equivalent.

## Consequences

### Positive

- One model for three OSes; each OS adds steps, not fields.
- Idempotence and drift become observable outcomes, testable by a second run.
- macOS host support within what Apple allows, with no step pretending otherwise.
- Headscale hosts emit correct descriptors.

### Negative

- `host check --json` grows; derived legacy keys must stay consistent with `services`.
- A new host configuration schema to version and document.
- The step refactor touches every OS implementation before any OS gains features.

### Neutral

- Installing the bridge as an OS service is out of scope; it belongs to the session
  supervisor (#451), with the broken Linux runbook tracked in #307.
- Port-level macOS filtering (pf) and MDM profiles are out of scope.

## Rejected Alternatives and Reopen Triggers

| Alternative | Rejected because | Reopen trigger |
| --- | --- | --- |
| A: RDP-shaped patch | Fails C3, C4, C10 | Never as the model; defect fixes still land |
| D: Ansible / `kardianos/service` | New dependency, single binary lost | Fleet-wide host management from one controller |
| E: macOS client only | Operator requires parity | Operator drops macOS host |
| `--profile` on host commands | Per-user store under elevation (C9) | A machine-scoped profile store is introduced |
| `SUDO_USER`-aware store resolution | Unix-only, implicit | Same as above |
| A default machine-wide config path | Explicit `--config` chosen for now | Operators repeatedly pass the same path |

## Implementation plan

Each item is its own spec and atomic PR, in this order:

1. Step/outcome model, seams and additive JSON, no behaviour change (#457).
2. Host configuration file and `host init` writing it (desired state) (#458).
3. Linux convergence (#454), Windows convergence (#455), macOS host (#459). These can run in
   parallel after 1 and 2.
4. Per-service descriptors with control-plane detection (#453). Can start after 1.

## Amendment A (Accepted, 2026-11-01): `host serve`, supervised by the OS

Status: proposed 2026-10-30 for HOST-008 (#522); accepted by the operator on 2026-11-01 in #525.

Context: a host on a network that sinkholes or TLS-inspects a self-hosted control plane
cannot enrol with native Tailscale, which has no SSH bootstrap (lesson-014). Clients
already reach that control plane with `connect --bootstrap-ssh` (CLI-363), so the host
side needs the same path, and it needs a process that stays up.

Decision:

- `ts-bridge host serve --config <path>` joins the tailnet as a **persistent** node and
  publishes exactly the services the host file declares (§1, §3). It reads only the host
  file and the paths it names (C9 holds: no per-user profile or credential store).
- Serve runs **in the foreground**, like `connect`. ts-bridge stays daemonless: it
  installs no service and supervises nothing. The OS starts and restarts it (Windows
  Scheduled Task at startup as SYSTEM; a systemd unit on Linux), registered by the
  consumer's provisioning, with the recipes documented here.
- The Neutral consequence "installing the bridge as an OS service is out of scope"
  still holds for ts-bridge itself; #451 remains the place for a local control plane.

Rejected for this amendment: an OS service inside ts-bridge (Windows SCM or systemd
integration in the binary), which is #451's question and would add a supervisor before
that architecture is decided.

## References

- #450 (epic), #457, #458, #459, #453, #454, #455, #307, #408, #451.
- [`docs/design/host-client-parity.md`](../design/host-client-parity.md).
- [ADR-005](adr-005-headscale-compat.md), [ADR-011](adr-011-shareable-connection-profile.md),
  [ADR-012](adr-012-config-profiles-model.md), [ADR-013](adr-013-cli-tests-in-go.md).
  ADR-016 (self-contained profile connections) lands with #442.
- `man systemsetup`; Apple Remote Desktop user guide (Screen Sharing on macOS 12.1+);
  Microsoft Learn: RDP listening port, `fDenyTSConnections`; `systemctl(1)`; `ufw(8)`;
  `iptables(8)`.
