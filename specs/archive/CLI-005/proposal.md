---
id: "CLI-005"
type: spec
status: archived
created: "2026-06-10"
tags: [spec, cli, config, yaml]
issue: 52
merged: bb6553e
pr: 68
---

# CLI-005: Add YAML config file support

## Why

Env vars are great for secrets and simple config, but multi-target support and persistent config require a structured file format. YAML is the standard in the Go/DevOps ecosystem.

## What

Add optional YAML config file support via --config flag. Config precedence: CLI flags > env vars > YAML > defaults. Auth key is NEVER read from YAML (security, see TECH-005).

## Out of scope

- Multi-target instances (future enhancement)
- TOML/JSON formats (YAML only)

## Acceptance criteria

- [ ] --config ts-bridge.yaml loads and applies settings
- [ ] CLI flags override YAML values
- [ ] Env vars override YAML values
- [ ] Missing YAML file is not an error (config is optional)
- [ ] Unknown YAML fields produce a warning
- [ ] Invalid YAML values produce a clear error
- [ ] Auth key in YAML is explicitly rejected with message to use TS_AUTHKEY
- [ ] go test ./... green
- [ ] PR < 250 lines diff (excluding tests)

## References

- ADR-008
- Issue #52
- TECH-005 (auth key security)