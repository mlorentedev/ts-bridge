---
id: "UX-003"
type: spec
status: archived
created: "2026-06-18"
tags: [spec, ux, discovery, api, tailscale, headscale]
issue: 168
---

# UX-003: Verification

## Evidence

Shipped and released — archived after the feature landed on `master`.

- **Merged PR:** [#171](https://github.com/mlorentedev/ts-bridge/pull/171) — `feat: add 'ts-bridge discover' subcommand for tailnet host auto-discovery`
- **Merge commit:** `e6f41b6`
- **Released in:** v1.14.0 ([#172](https://github.com/mlorentedev/ts-bridge/pull/172))
- **CI:** test / build-matrix / lint / security all green on the merged PR (gate to merge).
- **Surface:** `ts-bridge discover` is present in the released binary, with `internal/discover` (Tailscale + Headscale clients, fixture-based unmarshal tests).

> Detailed local `go test` / `golangci-lint` / `gosec` logs were not captured at
> archive time; the merged-PR CI run is the authoritative evidence. The manual
> smoke checklist below was not executed against a live tailnet and is left
> unchecked for honesty.

## Manual Smoke Test Checklist

- [ ] `ts-bridge discover` on a tailnet with multiple hosts → shows interactive list
- [ ] `ts-bridge discover --json` → outputs valid JSON array of devices
- [ ] `ts-bridge discover --filter "desktop"` → filters to matching hosts
- [ ] `ts-bridge discover --auto` → auto-selects first matching host, updates .env
- [ ] `ts-bridge discover` with invalid auth key → clear error message
- [ ] `ts-bridge discover` with no devices → informative "no hosts found" message
- [ ] `.env` after `--auto` → TS_TARGET updated, other vars preserved

## PR Link

- [#171](https://github.com/mlorentedev/ts-bridge/pull/171) (merged, commit `e6f41b6`)
