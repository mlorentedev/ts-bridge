## Problem

`Merge()` in `internal/config/merge.go` did not resolve `cfg.AutoInstance` from the full precedence chain (flags > env > yaml > default):

- `applyEnv()` ignored `TS_AUTO_INSTANCE` / `TS_MANUAL_MODE` env vars
- `applyFlags()` did not propagate `flags.ManualMode` to `cfg.AutoInstance`
- `cfg.AutoInstance` remained `true` (default) even in manual mode, creating an inconsistent state

This meant `TS_AUTO_INSTANCE=false` in `.env` had no effect — only `--manual-mode` flag worked.

## Fix

Added `resolveAutoInstance()` that runs after `applyFlags()` and before `applyAutoInstance()`, properly resolving `AutoInstance` from the full precedence chain:

1. `--manual-mode` flag → forces `AutoInstance = false`
2. `TS_MANUAL_MODE` env var → overrides YAML/default
3. `TS_AUTO_INSTANCE` env var → overrides YAML/default
4. YAML `auto_instance` field → overrides default
5. Default → `true` (auto-mode on)

## Tests

7 new unit tests:
- `TestMergeManualModeFlagDisablesAutoInstance`
- `TestMergeEnvAutoInstanceFalse`
- `TestMergeEnvManualModeTrue`
- `TestMergeFlagManualModeOverridesEnvAutoInstance`
- `TestMergeYAMLAutoInstanceExplicit`
- `TestMergeAutoInstanceEnvOverridesYAML`
- `TestMergeManualModeNoAutoDerivedValues`

All existing tests pass. Linter clean.

## Closes

Closes #150, Closes #149
