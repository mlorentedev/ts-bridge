---
id: "HOST-004-host-step-model"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-10-21"
issue: "mlorentedev/ts-bridge#457"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, cross-platform]
template_version: "1.0"
---

# HOST-004-host-step-model

## Why

<!-- from issue #457: HOST-004: state-first host step model with injected OS seams -->

ADR-017 makes `host setup` converge a declared state and `host check` diff it, with every step reporting `ok | changed | failed | manual | unverified`. Today each OS's steps live behind a build tag and call `os/exec` directly, so only the runner's own OS is ever tested (lesson 040): the Windows steps have never run under test on the Linux job, and darwin is a stub. Every other HOST-003 ticket (#458, #454, #455, #459, #453) needs one engine and one seam to land on, so this spec builds them first and changes no OS behaviour.

## What

Two PRs.

**PR A: engine and mechanical port.** It changes no command any OS runs.
- `internal/host` gains an untagged `Outcome` type (`ok`, `changed`, `failed`, `manual`, `unverified`) and a `Runner` seam (`Run`, `Output`, `LookPath`) with the `os/exec` implementation.
- The step lists become `setupFor(goos, cfg, runner)` and `checkFor(goos, cfg, runner)`. They are untagged, so the Windows, Linux and macOS lists are table-tested on any runner. Only `isElevatedImpl` stays build-tagged.
- `SetupStep` gains `Outcome` and `Fix`. `Success` stays, derived as `outcome in {ok, changed}`.
- Every existing step is ported with the same commands, arguments and order. A recording `Runner` asserts the exact argv sequence per OS against today's calls.

**PR B1: the contract, additive.** `host check` becomes the read half of the setup steps: it reports `steps[]` and `services[]` and still issues only reads. `host setup --json` and `host check --json` add `steps[].outcome` and `steps[].fix`, and `host check --json` adds `services[]` (`kind`, `port`, `enabled`, `active`, `firewall_ok`). `rdp_port`, `rdp_enabled` and `firewall_ok` stay, derived from the services (ADR-017 §3). The exit code does not change, so `smoke.bats` passes unchanged.

**PR B2: the exit code.** `check` exits 0 only when every step is `ok`. This breaks the current contract (`smoke.bats` and `docs/qa-coverage.md` assert exit 0 on a runner where nothing is installed, and `host check && ...` scripts change meaning), so it ships alone as an explicit contract change that updates both. Decided by the owner on 2026-10-21 when AC4 and AC5 were found to contradict each other.

### PR B1 read steps (today's reads, no new ones)

| OS | Steps `check` reports | Service |
|---|---|---|
| Linux | Tailscale IP, xrdp detection (`ok`/`failed`), Firewall configuration (`ok`, or `changed` = setup would add the rule) | `rdp`, `enabled` = xrdp on `PATH` |
| Windows | Tailscale IP, RDP configuration (registry read `ok`/`failed`), Firewall rule (`ok`/`changed`) | `rdp`, `enabled` = registry port read |
| macOS | Tailscale IP | none until #459 |

`active` is `null`: no step reads service state yet (L4 is #454, the Windows equivalent #455). Steps with no read today (Windows services, unattended mode, network profile, power) are not reported by `check` until #455 adds their reads. One value changes: macOS `rdp_port` was a hard-coded 3389 and is now 0, because macOS declares no service.

### Outcome mapping in PR A (today's truth, not the target)

| OS | Step | setup: success | setup: failure (fix) |
|---|---|---|---|
| Linux | xrdp detection (`LookPath xrdp`) | `ok` (a read) | `failed`, fix `sudo apt install xrdp` |
| Linux | Firewall (`ufw route allow ...` or `iptables -A ...`) | `changed` (acts unconditionally; L1/L2 are #454) | `failed`, fix names the command |
| Linux / Windows | Tailscale IP (`tailscale ip -4`) | `ok` (a read) | `failed`, fix "start Tailscale and log in" |
| Windows | Unattended mode (`tailscale up --unattended`) | `changed` | `failed`, fix "check the Tailscale GUI" (W1 is #455) |
| Windows | Tailscale / SSDP / UPnP services (`sc config`, `sc query`, `net start`) | `changed` | `failed` |
| Windows | Network profile, RDP configuration, Firewall rule, Power | `changed` (Firewall deletes and recreates: W3 is #455) | `failed` |
| macOS | (none: stub until #459) | none | none |

A step that only reads is `ok` when it succeeds, and a step that acts is `changed` every run. That is honest about today: a re-run still reports `changed`, and #454/#455 add the reads that turn it into `ok`.

## Out of scope

- Any read-first behaviour change: Linux `iptables -C`, ufw `allow` vs `route allow`, xrdp service state (#454); `tailscale set --unattended`, `fDenyTSConnections`, firewall rule state-first (#455); macOS steps (#459).
- The declared-state file and `--config` (#458), control-plane detection (#453).
- JSON contract changes: PR B1. The exit-code change: PR B2.

## Risks / open questions

- **Log lines change shape on Linux.** Linux logged ad-hoc messages per step. The shared runner logs one line per step, `Info` for `ok`/`changed` and `Warn` otherwise, as Windows already did. No test or contract reads these lines. This is the only observable difference in PR A, and it is deliberate.
- The ~300 LOC cap. Moving the Windows helpers out of the tagged file counts as new lines. If PR A exceeds the cap, Windows moves to its own PR (A2) with the same golden-argv test.

## Acceptance criteria

- [ ] AC1: Outcomes `ok | changed | failed | manual | unverified` exist with a `fix` string, and `SetupStep.Success` is derived from the outcome (PR A).
- [ ] AC2: Command runner is an injected seam. The Windows, Linux and macOS setup and check step lists are built from an OS argument and table-tested on Linux, and the recorded argv sequence per OS equals today's calls (PR A).
- [ ] AC3: `host check` runs only the read half of the same steps and never mutates (PR B1).
- [ ] AC4: `host check --json` / `host setup --json` add `services[]`, `steps[].outcome` and `steps[].fix`. Existing keys stay and are derived, and `smoke.bats` passes unchanged (PR B1).
- [ ] AC5: `check` exits 0 only when every step is `ok`; `smoke.bats` and `docs/qa-coverage.md` updated to the new contract (PR B2).

## References

- Bitácora board: `mlorentedev/ts-bridge#457` (epic #450)
- Related ADR: `docs/adr/adr-017-host-desired-state.md` (§2 outcomes, §3 contract, implementation plan)
- Gap inventory: `docs/design/host-client-parity.md` (G9, L1-L4, W1-W3)
- Lesson: `docs/lessons/lesson-040-2026-10-20.md` (OS as an input)
