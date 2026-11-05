---
id: "ts-bridge-e2e-vm"
type: runbook
status: active
created: "2026-11-05"
tags: [testing, e2e, host, linux, windows, release]
---

# Local E2E on Throwaway VMs

`make e2e` runs `ts-bridge host` and the client commands against real operating
systems: a fresh Ubuntu 24.04 guest (Lima) and a fresh Windows 11 Pro Arm64 guest
(QEMU), both through `dotf vm`. It exists because faked runners prove the argv we
chose, not that the OS accepts it (lesson 054).

It does not run in CI: GitHub runners cannot nest a VM. Run it:

- before merging a Release PR that carries a breaking or host-facing change;
- after any change to `internal/host` steps or to `cmd/cli/host.go`.

## Prerequisites

- `dotf` >= 0.79 (`dotf vm linux|windows up/exec/cp/down`), `gh` logged in, Go.
- Windows only: a clean base image, built once with `dotf vm windows build`
  (`~/.local/state/dotfiles/vm/windows/clean-base.qcow2`).
- An Arm64 host (Apple Silicon). `run.sh` builds both candidates with
  `GOARCH=arm64` and downloads the previous release's arm64 assets, so on an
  x86_64 host the guest is amd64 and every case fails with `exec format error`.
- RAM: one guest at a time. `make e2e` runs Linux, then Windows, never both.

## Run

```sh
make e2e-linux      # ~6 min
make e2e-windows    # ~15 min
make e2e            # both, one after the other
make e2e-clean      # delete any leftover tsb-e2e-* session; safe to repeat
```

Each run builds the working tree as the candidate (`tsb-new`), downloads the
previous release as `tsb-old` (checksums verified) for the upgrade cases, deletes
any leftover session, boots a fresh one, runs the scenario elevated, and deletes
the session on exit, failed runs included. `E2E_PREV=v1.20.0` pins the previous
release; `E2E_KEEP=1` keeps the guest for debugging (`make e2e-clean` after).

## Read the result

One line per case, and a summary:

| Line | Meaning |
|------|---------|
| `PASS name` | holds |
| `FAIL name` | regression: the run fails |
| `XFAIL #N name` | known open bug #N, still failing as expected |
| `XPASS #N name` | #N's case now holds: drop the issue from the case, the run fails until then |

The full log is in `scripts/e2e/out/<os>-<timestamp>.log` (git-ignored).

## What it covers

Scenarios: `scripts/e2e/linux.sh` and `scripts/e2e/windows.ps1`.

- `host init`: flags, file mode, `--force`, refusals.
- `host setup`: a first run on a fresh guest, then a second run that must
  change nothing (ADR-017).
- Drift: each state change is detected by `check` and repaired by `setup`. Linux covers
  the ufw rule and the xrdp service. Windows covers the firewall rule protocol, enabled
  state and deletion, RDP denied, and power.
- Refusals: ssh (#521), an unknown key, an unsupported version.
- The upgrade from the previous release's `.env`.
- The `host serve` preflight refusals.
- `import` and `status` on the client side.

Not covered: the tailnet data path. A real connection through a mesh needs a
control plane the guests can reach, which is tracked in #607.

## Troubleshooting

- `this dotf has no 'vm linux up'`: upgrade dotf.
- A run killed mid-way can leave a session: `make e2e-clean`.
- The Windows guest has no stdin: every `dotf vm windows exec` in `run.sh` reads
  `< /dev/null`.
