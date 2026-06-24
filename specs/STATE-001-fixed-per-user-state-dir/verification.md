---
tags: [spec, verification, templates]
created: "2026-06-24"
---

# Verification - STATE-001

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or
observed behavior).

- [x] Default is absolute per-user (not `./ts-state`) -> test `TestStateDirDefaultIsAbsolutePerUser`
      (+ `TestStateDirDefaultAutoNoInstanceIsPerUser`)
- [x] `StateDirForPlatform` per-OS + XDG + always absolute -> tests `TestStateDirForPlatform`,
      `TestStateDirForPlatform_EmptyEnvFallsBackToAbsoluteTemp`, `TestStateDirForPlatform_HonorsPlatformBaseEnv`,
      `TestStateDirForPlatform_LinuxXDGOverride`
- [x] Auto-instance ephemeral state not CWD-relative -> test `TestAutoInstanceEphemeralStateNotRelative`
- [x] Explicit overrides preserved -> test `TestStateDirExplicitOverridePreserved`
- [x] Warn-if-relative -> test `TestWarnRelativeStateDir`
- [x] Manual-mode regression -> test `TestMergeManualModeNoAutoDerivedValues` (unchanged, still green)
- [x] Docs updated -> `docs/troubleshooting/error-state-permissions.md`, `site/.../{configuration,getting-started,cli-reference}.md`

## Test status

- Test suite: `go build ./... && go vet ./... && go test ./...` -> all packages `ok` (run locally,
  Windows, no `-race`; CI runs `-race` on Linux).
- Manual smoke test: pending on a real run — confirm no `./ts-state` is created in the CWD and state
  lands under the per-user path.
- No regressions in existing test suite: yes (only `config_test.go` legacy `./ts-state` assertion
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
