---
tags: [spec, verification, templates]
created: "2026-06-24"
---

# Verification - STATE-001

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or
observed behavior).

- [x] Default is absolute per-user (not `./ts-state`) -> test `TestStateDirResolution`
- [x] `StateDirForPlatform` per-OS + XDG (incl. `~/.local/state` fallback) + always absolute ->
      `TestStateDirFor`, `TestStateDirForAlwaysAbsolute`, `TestStateDirForPlatformLive`
- [x] Auto-instance ephemeral state not CWD-relative -> `TestStateDirResolution` (ephemeral case)
- [x] Explicit overrides preserved -> `TestStateDirResolution` (relative) + `TestStateDirAbsoluteOverridePreserved` (env/yaml)
- [x] Ephemeral hostname can't traverse out of temp root -> `TestEphemeralStateDirNoTraversal`
- [x] Warn-if-relative -> test `TestWarnRelativeStateDir`
- [x] Manual-mode regression -> test `TestMergeManualModeNoAutoDerivedValues` (unchanged, still green)
- [x] Docs updated -> `docs/troubleshooting/error-state-permissions.md`, `site/.../{configuration,getting-started,cli-reference}.md`

## Test status

- Local (Windows, no cgo): `go build ./... && go vet ./... && go test ./...` -> all packages `ok`.
- Run **in CI only** (not reproducible on this box): `lint` (golangci-lint v1.62.2 — `gosec`,
  `goconst`, `unparam`, etc.), `test` / `test-windows` with `-race` (needs cgo). First CI run failed
  `lint` on `goconst "windows"` + `unparam stateDirFor`; both fixed in this branch (consts +
  `//nolint:unparam` test-seam). Re-run pending green.
- Manual smoke test: **pending** — needs a real auth key to confirm no `./ts-state` is created in the
  CWD and state lands under the per-user path.
- No regressions in existing suite: yes (only the `config_test.go` legacy `./ts-state` assertion was
  updated, by design).

## Decisions made during implementation

- Emit the relative-state-dir warning inside the `config` package (next to `warnEnvVar`/
  `warnPermission`) rather than in `cmd/cli`, so both the `connect` (`Merge`) and legacy
  (`LoadConfig`) paths are covered uniformly; falls back to stderr when the logger is not yet set.
- Extracted `appDirName` / `stateDirLeaf` consts to avoid a new `goconst` lint failure (the path
  components repeated ≥3× across the package). `gofmt` is **not** enforced by CI (`.golangci.yml`
  enables no formatter), so pre-existing alignment quirks in touched files were left untouched to
  keep the diff focused.
- `StateDirForPlatform` guards with an absolute `os.TempDir()` fallback when the platform base is
  empty — a robustness improvement over `LogDirForPlatform`, which can still return a relative path.
- Post-review (CodeRabbit) hardening: `EphemeralStateDir` now sanitizes the hostname to a single safe
  segment (`SanitizeHostnameLabel`) so a hostname with separators / `..` cannot escape the temp root.
- Kept the `warnRelativeStateDir` stderr fallback (matching the sibling `warnEnvVar` / `warnPermission`
  helpers) rather than switching only this one to a `slog.TextHandler` — consistency over a local
  style nit; converting all of them is out of scope for this PR.

## Promotion candidates

- [ ] Lesson for `docs/lessons.md`? yes — "default paths must be absolute; a CWD-relative default
      leaks node identity into git working trees" (also flags the latent `LogDirForPlatform` empty-base case).
- [ ] ADR-worthy decision? no — follows existing `LogDirForPlatform` convention.
- [ ] New pattern candidate? no — repo-local.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/STATE-001-fixed-per-user-state-dir/` -> `specs/archive/STATE-001-fixed-per-user-state-dir/`
- [ ] Bitácora board ticket (#207) moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
