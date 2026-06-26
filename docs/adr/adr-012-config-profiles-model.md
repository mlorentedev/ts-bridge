---
id: "ADR-012-config-profiles-model"
type: adr
status: accepted
owner: manu
date: "2026-06-26"
issue: "ts-bridge#185"
tags: [architecture, decision, configuration, profiles]
created: "2026-06-26"
---

# ADR-012: Named Profile Model for Multi-Tailnet Configuration

## Status

Accepted. Supersedes ADR-002 (env-var-only config).

## Date

2026-06-26

## Context

ADR-002 chose environment variables as the sole configuration method and explicitly
named CFG-001 as the future trigger to revisit that decision: *"Multi-target support
will eventually require a config file. When that happens, this ADR should be updated
to 'deprecated' and replaced."* That trigger is now active.

The concrete driver: operating two mutually-invisible tailnets (`work` → Tailscale
SaaS, `kubelab` → self-hosted Headscale) requires swapping `.env` files manually,
with the risk of using the wrong auth key on the wrong tailnet. A single flat `.env`
has no concept of a named profile.

CFG-002 (PR #228) built the first layer of this model: `internal/profile` (Store +
Descriptor), `ts-bridge import <tsb://> --profile <name>`, and `ts-bridge connect
--profile <name>`. CFG-001 completes it by adding manual profile creation via
`ts-bridge init --profile <name>` — without requiring a shareable descriptor URL.

### Hard constraints inherited from ADR-002

- **Secrets out of config:** `tskey-*` / `hskey-*` must never appear in the YAML
  profile store. They are provided via `--auth-key-file` or `.env` only.
- **Backward compatible:** no `--profile` flag → behavior identical to today's `.env`
  path. The precedence chain is `flags > env > profile > defaults`.
- **Locked-down machines:** the profile store path must be resolvable without admin
  rights or non-standard tooling.

## Decision

**Named profiles stored in a YAML file at `ProfileStorePath()`**, which is already
live in the codebase (`internal/config/statedir.go`). This resolves to:

- Windows: `%LOCALAPPDATA%\ts-bridge\profiles.yaml`
- Linux: `$XDG_STATE_HOME/ts-bridge/profiles.yaml` (falls back to `~/.local/state/ts-bridge/profiles.yaml` when `$XDG_STATE_HOME` is unset)
- macOS: `~/Library/Application Support/ts-bridge/profiles.yaml`

The file is a sibling of the `state/` subdirectory (not inside it) — `ProfileStorePath()` is derived from `filepath.Dir(StateDirForPlatform()) + "profiles.yaml"`, so profiles survive a state reset that wipes `state/`.

### Why not `os.UserConfigDir()` + `~/.config/ts-bridge/config.yaml`

`os.UserConfigDir()` (XDG config base) was evaluated and rejected in favor of the
existing `ProfileStorePath()` path for two reasons:

1. **Already live:** CFG-002 wrote `profiles.yaml` via `ProfileStorePath()`. Migrating
   to a different base directory would require a migration path for existing stores
   with no user benefit.
2. **Semantic fit:** profiles contain connection state (target, control URL) that
   logically belongs in the state directory hierarchy, not the config directory. The
   XDG distinction between `$XDG_CONFIG_HOME` (user preferences, version-controlled)
   and `$XDG_STATE_HOME` (persistent runtime state, not version-controlled) supports
   this placement.

### Profile schema (additive, not breaking)

```yaml
descriptor_version: 1
profiles:
  work:
    target: "host:3389"
  kubelab:
    target: "host:3389"
    control_url: "https://vpn.kubelab.live"
```

Profiles hold only non-secret parameters. The schema is additive — unknown keys are
ignored on load, so future fields can be added without a migration.

### Relationship with `tsb://` descriptors (CFG-002)

`tsb://` is a one-shot shareable URL for host→client propagation. `profiles.yaml` is
persistent local storage with a name. They are distinct representations with a
defined converter: `ts-bridge import <tsb://...> --profile <name>` (already live).
No new converter is needed.

## Consequences

### Positive

- Operators can switch tailnets with `--profile work` / `--profile kubelab` — no
  manual `.env` swapping.
- `ts-bridge init --profile <name>` enables manual profile creation without a
  shareable descriptor, covering the case where the host does not emit one.
- Secrets remain out of config — the guard is enforced at the `Store.Set` layer.
- Backward compatible: existing `.env` workflows are unchanged.

### Negative

- The profile store lives on disk; locked-down machines where `%LOCALAPPDATA%` is
  restricted may not be able to write it. Mitigation: error is surfaced clearly;
  `.env` path remains available as fallback.

### Neutral

- `ts-bridge profile list/rm` (profile management UI) is deferred to a follow-up PR.
- Multi-target simultaneous connections (issue #186) depend on this model but are
  not included here.

## References

- Supersedes: `docs/adr/adr-002-single-binary-no-config.md`
- Issue: ts-bridge#185
- CFG-002 implementation: PR #228 (`internal/profile/`, `ts-bridge import`, `connect --profile`)
- Related: `docs/adr/adr-011-shareable-connection-profile.md` (tsb:// design)
