---
id: "ADR-002-single-binary-no-config"
type: adr
status: deprecated
date: "2024-01-01"
tags: [architecture, decision, portability, configuration]
owner: manu
created: "2026-03-28"
---

# ADR-002: Single Binary with Environment Variables, No Config Files

## Context
ts-bridge targets locked-down corporate machines where users cannot install software or create persistent configuration. The tool must be maximally portable — download, set env vars, run.

## Options Considered
1. **Config file (YAML/TOML)**
    * *Pros:* Structured, supports complex config (multi-target), self-documenting.
    * *Cons:* Requires file creation on disk, another artifact to manage, overkill for 10 parameters.
2. **Environment variables only**
    * *Pros:* No files needed, works with `.env` loader scripts, standard 12-factor pattern, easy systemd/Docker integration.
    * *Cons:* Poor for complex structures (lists, nested objects), no inline comments.
3. **CLI flags + env vars**
    * *Pros:* Maximum flexibility, precedence chain.
    * *Cons:* Complexity for 10+ parameters, inconsistent UX.

## Decision
We chose **environment variables** as the sole configuration method (with `-v` and `-version` as the only CLI flags). This maximizes portability: the binary plus a `.env` file (or inline exports) is all that's needed.

## Consequences
- **Positive:** True single-binary distribution, works on any OS without file system writes (except state dir), trivial systemd/Docker integration via `EnvironmentFile=`.
- **Negative:** Multi-target support will eventually require a config file (tracked as backlog item CFG-001). When that happens, this ADR should be updated to "deprecated" and replaced.

## References
- https://12factor.net/config
- Superseded by: `docs/adr/adr-012-config-profiles-model.md` (issue #185)
