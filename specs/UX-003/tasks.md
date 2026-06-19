---
id: "UX-003"
type: spec
status: proposed
created: "2026-06-18"
tags: [spec, ux, discovery, api, tailscale, headscale]
issue: 168
---

# UX-003: Tasks (TDD order)

## Phase 1: API layer — data models + Tailscale API client

- [x] **Test:** `internal/discover` package — define `Device` struct + JSON unmarshaling tests for Tailscale API response shape
- [x] **Test:** `internal/discover` — `HeadscaleDevice` struct + JSON unmarshaling tests for Headscale API response shape
- [x] **Implement:** `internal/discover/tailscale.go` — `ListDevices(ctx, authKey, tailnet) ([]Device, error)`
- [x] **Test:** `tailscale.go` — parse real Tailscale API v2 response (fixture-based)
- [x] **Implement:** `internal/discover/headscale.go` — `ListDevices(ctx, apiKey, controlURL) ([]Device, error)`
- [x] **Test:** `headscale.go` — parse real Headscale API v1 response (fixture-based)

## Phase 2: Discoverer abstraction + cache

- [x] **Test:** `internal/discover/discoverer.go` — `Discoverer` interface + `Cache` struct with TTL tests
- [x] **Implement:** `Discoverer` interface (`List(ctx) ([]Device, error)`)
- [x] **Implement:** `Cache` wrapper (5-min TTL, skip if fresh)
- [x] **Test:** `Cache` — stale cache forces API call, fresh cache skips it

## Phase 3: CLI subcommand — `discover`

- [x] **Test:** `cmd/cli/discover.go` — `discoverCmd` Cobra command structure (help text, flags)
- [x] **Implement:** `discoverCmd` with flags: `--json`, `--filter`, `--auto`, `--auth-key`, `--port`
- [x] **Test:** `discover.go` — `--json` flag produces valid JSON output
- [x] **Test:** `discover.go` — `--filter` flag filters results correctly
- [x] **Implement:** `runDiscover` — orchestrates discoverer call → output formatting

## Phase 4: Interactive selection UI

- [x] **Test:** `cmd/cli/discover.go` — terminal list rendering (text-only, no TUI lib)
- [x] **Implement:** `selectHost(devices) (Device, error)` — interactive numeric selection
- [x] **Test:** `discover.go` — numeric input validation, out-of-range handling
- [x] **Test:** `discover.go` — name/hostname partial match selection
- [x] **Implement:** Substring match (type hostname to filter)
- [x] **Test:** `discover.go` — empty results when filter matches nothing

> **Decision:** Simple numeric + substring match (no external TUI dependency). Complexity reduction via function extraction kept gocyclo under 15.

## Phase 5: `.env` integration

- [x] **Test:** `internal/discover/env.go` — `UpdateEnv(path, target string, port int) error` — creates .env if missing
- [x] **Test:** `env.go` — merges TS_TARGET into existing .env without destroying other vars
- [x] **Test:** `env.go` — `--auto` flag skips confirmation prompt
- [x] **Implement:** `UpdateEnv` — non-destructive merge (read existing, update/append TS_TARGET, write back)
- [x] **Implement:** `runDiscover` integration — after selection, call `UpdateEnv` with confirmation (unless `--auto`)

## Phase 6: Error handling + polish

- [x] **Test:** `discover.go` — API error handling: invalid auth key → clear error message
- [x] **Test:** `discover.go` — network timeout → diagnostic message with remediation hint
- [x] **Test:** `discover.go` — no devices found → informative message
- [x] **Implement:** Error diagnosis helpers (`diagnoseTailscaleAPIError`, `diagnoseHeadscaleAPIError`)
- [x] **Implement:** Banner output for discover command ("TAILNET HOSTS" header)

## Phase 7: Integration + verification

- [x] **Test:** `go test ./...` — all packages green
- [x] **Verify:** `golangci-lint run` — clean
- [x] **Verify:** `gosec ./...` — clean (0 issues, 1 #nosec for G304 justified)
- [ ] **Note:** Production diff ~872 LOC (14 new files, 1795 total with tests). Needs split into 2-3 PRs per ~300 LOC cap.
