---
id: "UX-003"
type: spec
status: proposed
created: "2026-06-18"
tags: [spec, ux, discovery, api, tailscale, headscale]
issue: 168
---

# UX-003: Auto-discovery — list tailnet hosts from Tailscale API, select one interactively

## Why

The user has to know and manually write the hostname/IP of the target in `.env`:

```env
TS_TARGET=acemagic-lab-1:3389
```

This is friction for open-source: new users do not know what hosts are available on their tailnet.

## What

New `ts-bridge discover` subcommand that queries the Tailscale/Headscale API for tailnet devices, presents an interactive selection UI, and auto-updates `.env` with the selected `TS_TARGET`.

### Architecture

```
┌─────────────────────────────────────────────────┐
│  cmd/cli/discover.go        (Cobra CLI glue)    │
│  - discoverCmd, runDiscover                        │
│  - flags: --json, --filter, --auto, --port       │
├─────────────────────────────────────────────────┤
│  internal/discover/                                   │
│  ├── discoverer.go       (Discoverer interface)     │
│  │   ├── Device struct                               │
│  │   ├── Cache (5-min TTL)                         │
│  │   └── SelectHost(devices) → Device              │
│  ├── tailscale.go      (tailscale.com/client/      │
│  │                     tailscale.Client.Devices)    │
│  └── headscale.go      (custom HTTP client)        │
├─────────────────────────────────────────────────┤
│  internal/discover/env.go       (.env merge)       │
│  - UpdateEnv(path, target, port)                    │
└─────────────────────────────────────────────────┘
```

### API Strategy

- **Tailscale SaaS:** Use `tailscale.com/client/tailscale` — already a transitive dependency. `Client.NewClient(tailnet, APIKey)` + `Client.Devices(ctx, fields)`. Tailnet derived from auth key or `TS_TAILNET` env var.
- **Headscale:** Simple `http.Get` to `TS_CONTROL_URL/api/v1/device` with `Authorization: Bearer <TS_HEADSCALE_API_KEY>`. Parse custom JSON response shape.
- **Auto-detection:** Try Tailscale API first; if it fails with "tailnet not found" or similar, try Headscale. Or detect via `TS_CONTROL_URL` — if it points to a non-tailscale.com domain, use Headscale client.

### Interactive UI

Terminal-based list (no external TUI dependency). Two selection modes:
1. **Numeric:** `[1] hostname  100.64.0.1  authorized  last seen 2m ago`
2. **Name filter:** Type to narrow the list, Enter to select

### `.env` Integration

Non-destructive merge (same pattern as `buildEnvContent` in `init.go`): read existing vars, update/append `TS_TARGET`, write back. `--auto` skips confirmation.

## Out of scope

- Auto-configure RDP port detection (assume 3389, configurable with `--port`)
- Multi-tailnet support (one tailnet per auth key)
- Real-time host status updates (poll once on discover)
- SSH key management
- Integration into `ts-bridge init` wizard (deferred — UX-003 is standalone first)
- Fuzzy search library dependency (use simple substring match)

## Acceptance criteria

- [ ] `ts-bridge discover` lists tailnet hosts (Tailscale SaaS)
- [ ] `ts-bridge discover` lists tailnet hosts (Headscale)
- [ ] Interactive interface with numeric or name selection
- [ ] `--json` output for scripting
- [ ] `--filter` to filter by hostname/IP
- [ ] Auto-update `.env` with selected `TS_TARGET`
- [ ] `--auto` flag for non-interactive mode
- [ ] Graceful handling of API errors (invalid auth, rate limit, etc.)
- [ ] Tests for API response parsing
- [ ] Linter clean

## References

- Issue #168
- `tailscale.com/client/tailscale` API docs
