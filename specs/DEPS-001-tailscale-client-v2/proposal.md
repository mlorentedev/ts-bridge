---
id: "deps-001-tailscale-client-v2"
type: spec
status: proposed
created: "2026-07-10"
tags: [spec, deps, tailscale, discover, migration, sa1019]
issue: 245
---

# DEPS-001: Migrate `internal/discover` to Tailscale client v2 — Proposal

## Problem

Dependabot #199 bumps `tailscale.com` **v1.80.0 → v1.100.0**. The build, tests,
vet, and every other CI job pass at v1.100.0 — but the `lint` job fails with
`staticcheck SA1019` (use of deprecated symbol): `tailscale.com/client/tailscale`
and `ts.NewClient` are deprecated at v1.100.0 in favour of
`tailscale.com/client/tailscale/v2`. This is the enabled-in-CI SA1019 check, so
#199 cannot merge as-is.

The v2 client package **does not exist at v1.80.0** — it only ships from a later
release. So the migration cannot land on `master` on its own: it must be a
single PR that *both* bumps `tailscale.com` to v1.100.0 *and* rewrites the code
to the v2 client. This effectively **supersedes the tailscale half of #199**.

## Goal

Move `internal/discover`'s Tailscale API access off the deprecated v1 client and
onto `tailscale.com/client/tailscale/v2`, clearing SA1019, while keeping the
package's public behaviour byte-for-byte identical.

## Scope

### In scope

- `go.mod` / `go.sum` — bump `tailscale.com` to v1.100.0 **and** add the
  `tailscale.com/client/tailscale/v2` module (see the constraint below);
  `go mod tidy` clean.
- `internal/discover/tailscale.go` — import, `TailscaleClient` interface, client
  construction, the `Devices` call, and `convertTailscaleDevices` field mapping.
- `internal/discover/tailscale_test.go` — the mock that implements
  `TailscaleClient`, and the conversion tests, updated to the v2 `Device` type.

### Comes along transitively (not separable)

- `github.com/spf13/cobra` 1.8.1 → 1.10.2 and `github.com/spf13/pflag`
  1.0.5 → 1.0.10. `go mod graph` shows `tailscale.com@v1.100.0` **requires**
  cobra 1.10.2 / pflag 1.0.10, so MVS bumps them the moment tailscale moves to
  v1.100.0. This is exactly the cobra bump #199 also carried — so this PR fully
  supersedes #199 rather than just its tailscale half.

### Out of scope

- `ListDevices` / `convertTailscaleDevices` **public signatures** — unchanged.
  The migration is internal to client construction, the method call, and the
  `Device` field mapping.
- Any other package. Only `internal/discover` imports the tailscale client.

## Constraint & approach

**`tailscale.com/client/tailscale/v2` is a separate Go module** (major-version-2
import path), not a subpackage of `tailscale.com`. It is absent from the
`tailscale.com` module tree at every version, so the migration must `go get`
it as its own dependency (pinned to v2.10.1). The v1 client it replaces is
marked "only intended for internal and transitional use", so `//nolint`-ing the
SA1019 is not an acceptable alternative — the v2 module is the sanctioned path.

The v2 API is shaped differently (methods are grouped — e.g.
`client.Devices().List(ctx)` — and struct fields may be renamed, such as
`DeviceID` → `ID`, or retyped, such as `LastSeen` string → `time.Time`). The
exact v2 `Device` field names and types **must be verified against the
downloaded v2 module source**, not assumed. The mapping into our own
`discover.Device` (defined in `device.go`, unchanged) absorbs any rename/retype
so downstream code and JSON output are unaffected.

TDD: the existing `tailscale_test.go` conversion tests are the contract. They are
adapted to the v2 `Device` type and must keep asserting the same populated
fields (Hostname, OS, Authorized, IsExternal, …).

## Acceptance criteria

- [ ] `tailscale.com` at v1.100.0 in `go.mod`; `go mod tidy` leaves no diff.
- [ ] `internal/discover` imports `tailscale.com/client/tailscale/v2`; zero
      references to the deprecated v1 client remain.
- [ ] `go build ./... && go vet ./... && go test ./...` all green locally.
- [ ] CI `lint` job green — no SA1019 (the check that blocks #199).
- [ ] `discover` behaviour unchanged: the same device fields are populated from
      the API response.

## Non-functional / risks

- **Module download** — `tailscale.com@v1.100.0` is a heavy download; run
  detached so a timeout-killed `go` cannot corrupt the module cache on Windows.
  Never run two `go` processes concurrently.
- **Zero-dependency ethos** — the v2 client module is already pulled in
  *indirectly* by `tailscale.com@v1.100.0`; the migration only promotes it to a
  direct requirement (`go get tailscale.com/client/tailscale/v2@v2.10.1`) and
  drops the deprecated v1 client import. No genuinely-new module enters the tree.
- **Lint verified in CI only** — `golangci-lint` (and thus the SA1019 check) is
  not reliably runnable on the maintainer's Windows box; the SA1019 clearance is
  confirmed by the CI `lint` job, not locally.
