---
id: "adr-008"
type: adr
status: accepted
date: "2026-06-10"
tags: [architecture, decision, cli, configuration]
owner: manu
---

# ADR-008: CLI Architecture — Cobra Subcommands + Optional YAML Config

## Context

ts-bridge currently relies on shell scripts (run.sh, run.ps1, bootstrap.sh, bootstrap.ps1) for client operations and environment-variable-only configuration (ADR-002). This creates:

1. **Cross-platform fragmentation** — dual shell scripts (bash + PowerShell) that must be kept in sync
2. **No unified CLI** — users must know which script to run, with different flags per platform
3. **No multi-target support** — env vars are single-target by nature (CFG-001 backlog item)
4. **No interactive setup** — bootstrap requires manual .env editing

The project has matured to v1.7.2 with a stable core. The shell-script launcher pattern was appropriate for early development but is now the primary friction point for users.

## Options Considered

### CLI Framework

1. **stdlib flag + manual dispatch** — Zero dependencies, but verbose for 5+ subcommands, poor --help generation.
2. **spf13/cobra** — Industry standard (Docker, Kubernetes, Hugo, GitHub CLI). Subcommands, autocomplete, man pages, help generation. ~5MB in go.sum.
3. **alecthomas/kong** — Struct-tag-based, less boilerplate than cobra. Smaller ecosystem, fewer examples.
4. **charmbracelet/bubbletea** — TUI framework. Overkill for a CLI tool; the init wizard can use simple stdin prompts.

### Config Format

1. **Env vars only (status quo)** — ADR-002. Works but no multi-target, no persistent config.
2. **YAML** — Standard in Go/DevOps. gopkg.in/yaml.v3 is mature. Supports comments, multi-target.
3. **TOML** — Simpler than YAML but less familiar to the target audience.
4. **JSON** — Zero dependencies but no comments, verbose.
5. **Env vars + YAML (chosen)** — Backward compat + optional structured config.

## Decision

**Adopt Cobra as the CLI framework and add optional YAML config file support while maintaining env-var backward compatibility.**

### Subcommand Tree

```
ts-bridge
├── connect          # Run the bridge (replaces run.sh/ps1)
├── init             # Interactive setup wizard (replaces bootstrap.sh/ps1)
├── host setup       # Configure RDP host (Windows, admin)
├── host check       # Verify host readiness
├── status           # Show bridge status / health
├── version          # Show version info
└── help             # Built-in
```

### Config Precedence (high to low)

1. CLI flags
2. Env vars (TS_*)
3. YAML config file (--config)
4. Built-in defaults

### Security Constraint

The auth key (TS_AUTHKEY) MUST remain exclusively in the env var. It MUST NOT be readable from YAML config files. YAML config is for non-sensitive settings only.

## Consequences

### Positive

- **Single binary, cross-platform** — no more dual shell scripts
- **Professional CLI** — --help, autocomplete, consistent UX
- **Backward compatible** — all existing TS_* env vars still work
- **Multi-target ready** — YAML config sections enable future multi-instance support
- **Reduced maintenance** — one code path instead of bash + PowerShell

### Negative

- **New dependencies** — cobra + yaml.v3 added to go.mod
- **ADR-002 partially deprecated** — env vars remain primary for secrets, YAML for structure
- **Migration effort** — scripts must be ported, tested, then removed (~7 PRs)

### Neutral

- The core bridge logic (proxy, health, telemetry) is unchanged
- The config.Config struct is extended but not replaced

## References

- ADR-002 (env-var-only config, being superseded for structure)
- specs/CLI-001 through CLI-006
- https://github.com/spf13/cobra
- https://pkg.go.dev/gopkg.in/yaml.v3
- Issue #38