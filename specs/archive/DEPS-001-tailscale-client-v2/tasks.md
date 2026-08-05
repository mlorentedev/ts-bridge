---
id: "deps-001-tailscale-client-v2"
type: spec
status: proposed
created: "2026-07-10"
tags: [spec, deps, tailscale, discover, migration, sa1019]
issue: 245
---

# DEPS-001: Tasks

## This PR — bump + migrate (issue #245)

- [ ] **Bump:** `go get tailscale.com@v1.100.0 && go mod tidy` (run detached).
- [ ] **Verify:** read the v2 `Client` / `Devices` API and the v2 `Device`
      struct in the downloaded module source — record the exact field
      names/types before touching code.
- [ ] **Migrate `tailscale.go`:**
  - [ ] Import `tailscale.com/client/tailscale/v2` (aliased `ts`).
  - [ ] Re-express `TailscaleClient` against the v2 method shape.
  - [ ] Construct the v2 client with the tailnet + API key.
  - [ ] Call the v2 devices-list method.
  - [ ] Update `convertTailscaleDevices` field mapping to the v2 `Device`.
- [ ] **Migrate `tailscale_test.go`:** update the mock and the `Device` literals
      to the v2 type; keep the same asserted behaviour.
- [ ] **Green:** `go build ./...`, `go vet ./...`, `go test ./...`.

## Verification (this PR)

- [ ] `grep -R "client/tailscale\"" internal/` returns nothing (no v1 import).
- [ ] `go mod tidy` produces no further diff.
- [ ] CI `lint` job passes (SA1019 cleared) — the gate that blocks #199.
- [ ] `discover` unit tests still assert the same populated device fields.

## Decisions

- **Single PR bumps *and* migrates:** the v2 client is absent at v1.80.0, so the
  code change cannot precede the bump. Supersedes the tailscale half of #199.
- **Cobra bump excluded:** unrelated to SA1019; Dependabot handles it separately
  once tailscale is at v1.100.0.
- **Public API frozen:** `ListDevices` / `convertTailscaleDevices` signatures do
  not change; the `discover.Device` mapping absorbs any v2 rename/retype.
