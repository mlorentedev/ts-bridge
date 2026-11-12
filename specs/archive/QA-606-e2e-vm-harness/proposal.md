---
id: "QA-606-e2e-vm-harness"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-11-04"
issue: "mlorentedev/ts-bridge#606"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# QA-606-e2e-vm-harness

> **Naming**: file lives at `<repo>/specs/QA-606-e2e-vm-harness/proposal.md`. `QA-606-e2e-vm-harness` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #606: QA: local e2e harness for host setup and check on Ubuntu and Windows VMs -->

`host setup` and `host check` are tested only against faked OS state. On 2026-11-05, one manual run on two VMs found problems no unit test could see:

- a Windows power step that had never worked (#603);
- an idempotence break (#604);
- reporting bugs (#605, #608, #609);
- #557 in practice.

A breaking release (2.0.0) needs a repeatable way to see the binary converge a real machine before it is cut.

## What

A local, opt-in harness under `scripts/e2e/`, run with `make e2e` (both OSes), `make e2e-linux`, `make e2e-windows` and `make e2e-clean`. One run:

1. Builds the binary from the working tree for the guest (`linux/arm64`, `windows/arm64`) and downloads the previous release, with checksums verified, for the upgrade case.
2. Deletes any leftover `tsb-e2e-<os>` session, brings a fresh one up with `dotf vm <os> up`, and always tears it down (`trap`, also on failure and Ctrl-C). `E2E_KEEP=1` keeps it for debugging.
3. Copies in the binaries and the guest scenario and runs it elevated. The scenario prints one line per case: `PASS`, `FAIL`, `XFAIL #N` (a known open bug, still failing) or `XPASS #N` (a known bug that now passes, so the marker should go).
4. Exits non-zero on any `FAIL` or `XPASS` and keeps the guest's log under `scripts/e2e/out/<os>-<timestamp>.log` (git-ignored).

The scenarios cover everything the host side implements that needs no tailnet:

- **`host init`:** a missing `--config`, the YAML it writes, overwriting with and without `--force`, a directory or unreadable path, and the Windows-only flags.
- **`setup` and `check`:** two passes, where the second must report no `changed`, and the `--json` contract.
- **Drift repair:**
  - Linux: firewall rule removed, xrdp stopped;
  - Windows: rule disabled, deleted or switched to UDP, RDP denied, power timeouts reset.
- **Refusals:** an `ssh` service (#521), an unknown key (warned), a bad `version`, and `host serve`'s preflight (no `host.mesh`, a loose `state_dir`, an `auth_key_file` under a writable ancestor).
- **Upgrade:** the previous release's `.env` still applies to `setup` and `check` from that directory.
- **Client side, offline:** `version`, `init --profile`, `import` of a `tsb://` descriptor, `status --json` with nothing running.

## Out of scope

- The data path over a tailnet: RDP and SSH through `connect` and `--socks5`, and `host serve` serving. That is #607 (local Headscale).
- CI. GitHub runners cannot nest these VMs, and `.github/**` is owner-only. The release runbook asks for a run before a breaking or significant release.
- x64 Windows. The guest is Windows 11 Pro **Arm64**, and the results say so.
- macOS hosts (#459).

## Risks / open questions

- Resolved: needs `dotf` >= 0.79 (session verbs `up`/`exec`/`cp`/`down`) and a built Windows clean-base (`dotf vm windows build`). The harness checks both and stops with the fix.
- Resolved: `dotf vm windows exec --script` refuses when stdin is not closed, so the harness passes `< /dev/null`. `cp` needs the guest directory to exist, so the guest scenario creates it first.
- Resolved: the guests have outbound network (apt installs xrdp). No credential enters a guest, and Tailscale is not installed, so the Tailscale steps are expected to fail and are asserted as such.
- Accepted: the Linux run takes about 5 min, Windows about 10 min, and both together need about 16 GB of free RAM. `make e2e` runs them one after the other to keep the peak down.

## Acceptance criteria

- [x] AC1: `make e2e-linux` brings a fresh Ubuntu guest up, runs the Linux scenario, prints one result line per case, and exits 0 only when no case is `FAIL` or `XPASS`.
- [x] AC2: `make e2e-windows` does the same on a Windows 11 Arm64 guest, running the real PowerShell reads and writes (#601).
- [x] AC3: every run deletes a leftover session first and leaves no session behind, whether it passes, fails or is interrupted. `make e2e-clean` removes leftovers without running anything. A second run gives the same result.
- [x] AC4: on both OSes, the second `host setup` reports no `changed` step, and every drift case is detected by `check` and repaired by `setup`.
- [x] AC5: a known open bug is marked `XFAIL #N` and does not fail the run. When it is fixed, the run reports `XPASS #N` and fails until the marker is removed.
- [x] AC6: `docs/runbooks/e2e-vm.md` gives the preconditions, the trigger (before cutting a breaking or significant release), how to read the results, and the cleanup. The release runbook links it.

## References

- Bitácora board: the GitHub issue / Project item tracking this spec (see the `issue:` frontmatter field)
- Related ADR: `docs/adr/adr-017-host-desired-state.md` (idempotence contract)
- Related: #601, #603, #604, #605, #607, #608, #609, #557; lesson 054
