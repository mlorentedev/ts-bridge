---
id: "UX-003"
type: spec
status: proposed
created: "2026-06-18"
tags: [spec, ux, discovery, api, tailscale, headscale]
issue: 168
---

# UX-003: Verification

## Evidence

> Fill after implementation. Include:
> - Commit hashes of merged PR(s)
> - Test output (`go test -race -v ./...`)
> - Linter output (`golangci-lint run`)
> - Security scan output (`gosec ./...`)
> - Manual smoke test results (actual `ts-bridge discover` run on real tailnet)

## Manual Smoke Test Checklist

- [ ] `ts-bridge discover` on a tailnet with multiple hosts → shows interactive list
- [ ] `ts-bridge discover --json` → outputs valid JSON array of devices
- [ ] `ts-bridge discover --filter "desktop"` → filters to matching hosts
- [ ] `ts-bridge discover --auto` → auto-selects first matching host, updates .env
- [ ] `ts-bridge discover` with invalid auth key → clear error message
- [ ] `ts-bridge discover` with no devices → informative "no hosts found" message
- [ ] `.env` after `--auto` → TS_TARGET updated, other vars preserved

## PR Link

> Fill after merge: link to the merged PR(s)
