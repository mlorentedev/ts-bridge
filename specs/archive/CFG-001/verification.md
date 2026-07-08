---
tags: [spec, verification]
created: "2026-06-26"
---

# Verification - CFG-001

## Evidence

Every acceptance criterion from `proposal.md` mapped to concrete proof. All new tests landed in the squash-merge commit `6046f22` (PR #232).

- [x] Criterion 1 — `init --profile <name> --target <host:port>` writes the profile via `ProfileStorePath()`; re-run with `--force` updates, without `--force` fails with a clear error → `TestRunInitProfile/writes_profile_without_control_URL`, `TestRunInitProfile/existing_profile_without_--force_returns_error` (error contains `"already exists"`), `TestRunInitProfile/existing_profile_with_--force_overwrites` (commit `6046f22`, `cmd/cli/init_test.go`). Storage layer covered by `TestStore_Set/writes_profile_without_control_URL` and `.../writes_Headscale_profile_with_control_URL` (`internal/profile/store_test.go`).
- [x] Criterion 2 — `connect --profile <name>` loads the profile and an explicit `--target` overrides it (`flags > profile`) → **shipped in CFG-002 (PR #228)**: `TestApplyProfile_ResolvesTargetFromStore` + `applyProfile` only fills `cfg.Target`/`cfg.ControlURL` when empty, so an explicit flag always wins. CFG-001 is the write-side complement; no change to the connect path.
- [x] Criterion 3 — without `--profile`, existing `.env` behavior is identical (regression) → non-profile init path unchanged: `TestWriteEnvConfig_CreatesFullConfig`, `TestWriteEnvConfig_DetectsExistingFile`, `TestWriteYAMLConfig_CreatesYamlAndEnv` (`cmd/cli/init_test.go`); `validateProfileModeFlags` gates profile-only flags via `cmd.Flags().Changed()` so non-profile invocations are untouched. Full suite green, no regressions.
- [x] Criterion 4 — no secrets in generated profile data; write rejected if target carries key material → `TestStore_Set/rejects_secret_in_target:_tskey-auth-abc123:3389` and `.../rejects_secret_in_target:_hskey-preauth-xyz:3389`; `Store.Set` scans `secretPrefixes` (`tskey-`/`hskey-`) and errors before writing (`internal/profile/store.go:72`).
- [x] Criterion 5 — ADR-012 written; ADR-002 marked deprecated → `docs/adr/adr-012-config-profiles-model.md` (`status: accepted`, +123 lines) and `docs/adr/adr-002-single-binary-no-config.md` frontmatter set to `status: deprecated` (commit `6046f22`).

## Test status

- Test suite: `go test ./... -count=1` → all packages pass, 0 failures (CI runs with `-race` on Linux)
- New tests: `TestStore_Set` (6 subtests: write, Headscale, idempotent, overwrite, empty-name guard, 2× secrets guard) + `TestRunInitProfile` (4 subtests: write, control URL, overwrite protection, force overwrite)
- No regressions in existing test suite: yes

## Decisions made during implementation

- `Store.Set(name, target, controlURL)` rejects `tskey-`/`hskey-` prefixes in `target` at write time (not just at read/print) — the store is the last line of defense against a secret leaking into a shareable profile file.
- `validateProfileModeFlags` uses `cmd.Flags().Changed()` (not zero-value checks) so it only rejects flags the user *explicitly* set, avoiding false positives on defaults.
- `runInitProfile(storePath string, f initFlags)` takes `storePath` as a parameter so tests inject a `t.TempDir()` path — same test-isolation pattern as CFG-002's `defaultProfileStorePath` var.
- Table-driven `t.Run` subtests throughout (project convention, aligns with CodeRabbit review guidance for this repo).

## Promotion candidates

- [x] Lesson for the repo's `docs/lessons.md`? No — clean feature addition, no post-mortem-worthy surprise.
- [x] ADR-worthy decision for the repo's `docs/adr/`? Yes — already executed: ADR-012 (config profiles model) written, ADR-002 deprecated, both merged in PR #232.
- [x] New pattern candidate for `00_meta/patterns/`? No — single-project concern.

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/CFG-001/` -> `specs/archive/CFG-001/`
- [x] Bitácora board ticket #185 closed with PR link (ADR-018) — closed on PR #232 merge via `Closes #185`
- [x] Promotions above executed (ADR-012 + ADR-002 deprecation shipped in PR #232)
