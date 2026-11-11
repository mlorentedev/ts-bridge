---
id: "HOST-011-control-liveness"
type: spec
status: draft # draft | implementing | verifying | archived
created: "2026-11-11"
issue: "mlorentedev/ts-bridge#565"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, serve, health]
template_version: "1.0"
owner: manu
---

# HOST-011-control-liveness

## Why

`host serve` reports its control session on `/health/ready` from the node's IPN bus states (#559/#560): any state but `Running` reads not ready with `reason=control_plane_unreachable`. Measured on 2026-11-11 against the local QA-607 mesh, the bus does not see an outage: with Headscale stopped for four minutes after `READY`, the backend state stayed `Running`, no health warning appeared, and `/health/ready` answered 200 throughout. A host that has lost its control plane therefore reads healthy, and `host check --json` keeps reporting it published (HOST-008 AC4, AC6).

## What

`host serve` also polls the node's own status and reads `Self.Online`, the flag the measurement showed tracking the control connection (false within 3 s of the stop, true again 14 s after the restart). While the node is offline for three consecutive polls, `/health/ready` answers 503 with `reason=control_plane_unreachable`; the first online poll clears it. The bus watcher stays as it is: it still reports a logout or a bus that ends.

Design, settled by the measurement:

- **Signal**: `ipnstate.Status.Self.Online`. Health warnings are not a signal: the list stayed empty for the whole outage.
- **Bound**: poll every 10 s; down after 3 consecutive offline samples (about 30 s, so one missed map response does not flap readiness); up on the first online sample. A status read that fails counts as an offline sample, so a broken local API fails closed.
- **Readiness source**: a second source, `control-online`, beside the bus's `control`, with the same reason token. The fold already reports the first down source's reason, so the token on `/health/ready` is the same whichever fires, and the two watchers share no state.
- **Seam**: the poller takes a `func(context.Context) (*ipnstate.Status, error)` and a tick channel, so unit tests drive an online, offline, offline, offline, online sequence with no clock and no tailnet.

## Out of scope

- Reconnecting: tsnet's control client already retries; this only makes health say so.
- The live run with `bootstrap_ssh` against the lab control plane: that is HOST-008's live acceptance, owned by another session. The fold covers the combination in unit tests.
- `host check`'s own reading of `published`: it already follows `/health/ready`.

## Risks / open questions

- `Self.Online` comes from the control server's view relayed in the map response. If a control server stopped sending it, the poller would read offline forever. Mitigation: the e2e outage case runs against Headscale, the control plane this project targets, and checks the healthy baseline first.
- The 30 s bound is a choice, not a measurement. It trades a short flap window for detection time; it is one constant.

## Acceptance criteria

- [ ] AC1: with the control plane unreachable after `READY`, `/health/ready` answers 503 with `reason=control_plane_unreachable` within 40 s, and 200 again within 20 s of recovery. Shown end to end on the local mesh (no `bootstrap_ssh`); with `bootstrap_ssh` declared, the unit tests show the same reason whichever source is down.
- [ ] AC2: one or two offline samples do not change readiness; the third does. A failed status read counts as an offline sample.
- [ ] AC3: the bus watcher keeps its behavior: a non-`Running` state or a bus that ends still reads not ready.

## References

- Issue #565; HOST-008 (#522), PRs #559/#560.
- Measurement log: `verification.md` (2026-11-11).
