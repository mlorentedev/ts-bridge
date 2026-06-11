---
id: "CLI-003"
type: spec
status: archived
created: "2026-06-10"
tags: [spec, cli, init, wizard]
issue: 50
---

# CLI-003: Implement ts-bridge init interactive setup wizard

## Why

Replace bootstrap.sh/bootstrap.ps1 with a native Go interactive wizard. Users run ts-bridge init and get prompted for auth key, target, instance name, and config format.

## What

Implement ts-bridge init with interactive and non-interactive modes.

### Interactive mode (no flags)

When run without --auth-key and --target:
1. Prompt for auth key (masked input, no echo)
2. Prompt for target host:port (with validation)
3. Prompt for instance name (optional)
4. Prompt for config format: YAML or .env
5. If YAML: write non-sensitive settings to YAML, write auth key to .env (referenced from YAML or loaded at runtime)
6. If .env: write all settings including auth key to .env
7. Print next steps

### Non-interactive mode

When --auth-key and --target are provided, writes config silently (useful for automation/CI):
- Default format is .env (backward compat)
- Use --format yaml to write YAML
- Auth key NEVER goes into YAML even in non-interactive mode

### Security

- Auth key input is masked in interactive mode (using term.ReadPassword or equivalent)
- In YAML mode: auth key goes to .env file, NOT to the YAML file
- Show clear warning if .env or config file has world-readable permissions after writing

### CLI Flags

```
--auth-key KEY        Auth key (non-interactive mode) — WARNING: visible in process list
--target HOST:PORT    Target address (non-interactive mode)
--instance NAME       Instance name (optional)
--port-range RANGE    Port range for auto mode
--format FORMAT       Output format: yaml or env (default: env)
--config PATH         Output config file path (default: ./ts-bridge.yaml for yaml, ./.env for env)
```

## Out of scope

- Removing old scripts (OPS-002)

## Dependencies

- Depends on CLI-001 (Cobra scaffold)
- Depends on CLI-002 (YAML config loader, for writing valid YAML)

## Acceptance criteria

- [ ] Interactive mode prompts for auth key (masked), target, instance, format
- [ ] Auth key input is masked (no echo to terminal or history)
- [ ] Non-interactive mode: ts-bridge init --auth-key X --target Y writes config silently
- [ ] YAML output: auth key goes to .env, non-sensitive settings to YAML
- [ ] .env output matches current .env.example format
- [ ] --config PATH writes to custom location
- [ ] Warning if config file has world-readable permissions
- [ ] go test ./... green
- [ ] golangci-lint run clean
- [ ] PR < 250 lines diff (excluding tests)

## References

- ADR-008
- Issue #50
- scripts/client/bootstrap.sh, bootstrap.ps1
- TECH-005 (auth key security)