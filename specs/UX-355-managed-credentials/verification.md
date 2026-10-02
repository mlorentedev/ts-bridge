---
tags: [spec, verification, templates]
created: "2026-09-29"
---

# Verification - UX-355-managed-credentials

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] Criterion 1 -> credential store tests plus `TestAuthSetStoresMaskedCredentialAndBindsProfile` and overwrite tests.
- [x] Criterion 2 -> `TestStoreCredentialReference`, backward-compatibility/preservation, clear/list, and validation tests.
- [x] Criterion 3 -> connect precedence tests, `TestBrowserManagedProfileCredentialOverridesEnvironmentWithWarning`, and `TestBrowserAuthKeyFileOverridesProfileCredential`.
- [x] Criterion 4 -> auth list/status/remove and shared-reference protection tests.
- [x] Criterion 5 -> store validation/remove tests, `TestStoreFailedAtomicReplacePreservesExistingCredential`, `TestCredentialStoreDirIsSiblingOfProfileStore`, Unix mode test, and Windows `icacls` argument test.

## Test status

- Foundation packages: `go test ./internal/credential ./internal/profile` -> passed.
- Path contract: `go test ./internal/config -run TestCredentialStoreDirIsSiblingOfProfileStore -count=1` -> passed.
- Corrected f1 contract: `go test ./internal/credential ./internal/config -run '^(TestStoreSetGetAndList|TestStoreRefusesOverwriteUnlessExplicit|TestStoreFailedAtomicReplacePreservesExistingCredential|TestStoreValidatesNamesAndKeysWithoutLeakingValues|TestStoreRemove|TestStoreUsesOwnerOnlyPermissionsOnUnix|TestHardenCredentialPathUsesOwnerOnlyWindowsACLs|TestCredentialStoreDirIsSiblingOfProfileStore)$' -count=1` -> passed.
- Controlled mutation proof: temporarily bypassing `ValidateName` made the corrected f1 command fail in `TestStoreValidatesNamesAndKeysWithoutLeakingValues`; production code was restored before the passing run.
- Browser boundary: `go test ./cmd/cli -run '^TestBrowserManagedProfileCredentialOverridesEnvironmentWithWarning$' -count=1` -> passed after first failing because the ignored environment credential had no diagnostic.
- Atomic rollback: `go test ./internal/credential -run '^TestStoreFailedAtomicReplacePreservesExistingCredential$' -count=1` -> passed; bypassing the injected rename seam made it fail with `Set() error = <nil>, want injected rename failure`, then the production call was restored.
- Missing managed credential fallback: `go test ./cmd/cli -run '^TestConnectEnvOverridesMissingProfileCredential$' -count=1` -> passed; removing the `os.Getenv("TS_AUTHKEY")` guard made it fail while loading the absent managed credential, then the production guard was restored.
- Final f4 contract: `go test ./cmd/cli ./internal/config -count=1` -> passed; making `ProfileAuthKey` override a populated environment key made the exact command fail in `TestPrecedenceProfileAuthKeyIsBelowEnvironmentAndFlags/environment_overrides_profile`, then production was restored.
- Shared init validator: `go test ./cmd/cli -run '^TestInitAuthKeyFile/prefixed_malformed_key_returns_validation_error$' -count=1` and `go test ./cmd/cli -run '^TestInitAuthKeyValidationMatchesConfigMerge$' -count=1` -> failed before delegation because `tskey-auth-abc` was accepted by init, then passed after `validateAuthKey` delegated to `credential.ValidateKey`.
- Headscale control-plane pairing (round 7): `go test ./cmd/cli -run '^(TestInitHeadscaleControlURL|TestInitAcceptedConfigIsAcceptedByConnect)$' -count=1` -> failed before the fix in six subtests, because init wrote `TS_AUTHKEY=hskey-abcdef` with only a commented `# TS_CONTROL_URL=` and reported success while `config.Merge` rejected that same file. Passed after `writeConfig` validates with `config.ValidateControlPlane` before writing and emits `TS_CONTROL_URL` / `control_url` when `--control-url` is given.
- Effective env layer (round 8): `go test ./cmd/cli -run '^TestInitValidatesTheEffectiveEnvLayer$' -count=1` -> all four subtests failed before the fix, seeded with the reviewer's own reproduction (a pre-existing `.env`): a stale but valid `TS_CONTROL_URL` stayed the effective value under the flag, a malformed preserved `TS_CONTROL_URL` was accepted for both `hskey-` and `tskey-` keys, and a malformed preserved `TS_TARGET` was carried into the config `connect` rejects. Green after `buildEnvContent` writes the validated target and control URL and `writeConfig` validates a preserved control URL when no flag is given. Every case starts from a populated directory — the state the round-8 review proved the earlier tests could not reach.
- Mutation proof for the round-8 fix: (a) dropping `values["TS_TARGET"] = f.Target` -> 2 subtests red; (b) dropping the `TS_CONTROL_URL` write -> 1 red; (c) making `effectiveControlURL` skip the preserved read -> 2 red. All three restored and the package re-ran green.
- `.env` parser consolidation: `go test ./internal/config/envfile ./internal/config -count=1` and `go test ./cmd/cli -count=1` -> passed; `envfile.Values` is exercised by both the loader and the init round-trip, so a parse change moves both.
- Mutation proof for the pairing fix: (a) deleting the `config.ValidateControlPlane` call in `writeConfig` turns both tests red; (b) removing the `TS_CONTROL_URL=` emission turns both red; both mutations were restored and the package re-ran green. A generated Headscale config is therefore accepted by the same merge connect runs, and a refused pairing writes no file at all.
- Doc claim for the pairing (round 7 over-claim): `go test ./cmd/cli -run '^TestManagedCredentialDocumentation$' -count=1` -> passed; `configuration.md` now states the `hskey-*` control-plane requirement and the refusal, `cli-reference.md` documents `init --control-url`, and the doc guard pins both strings so the claim cannot drift away from the behavior again.
- Guard against the doc example class: `bash scripts/check-doc-authkey.sh` -> `OK (71 files)` with the new `init --auth-key-file ... --control-url` example present.
- Init UX regressions: `go test ./cmd/cli -run '^(TestValidateAuthKeyPreservesLoginURLHint|TestPrintNextSteps|TestManagedCredentialDocumentation)$' -count=1` -> passed; profile output alone includes managed credential onboarding, and the security audit documents structural validation.
- Static checks: `go vet ./cmd/cli ./internal/credential ./internal/config` -> passed.
- CLI regression set: `go test ./cmd/cli -run 'TestAuth|ProfileCredential|BrowserAuthKeyFileOverrides|TestNewRootCmdContainsProductionCommands|TestManagedCredentialDocumentation' -count=1` -> passed.
- Build/vet: `go build ./...` and `go vet ./...` -> passed.
- Lint: `golangci-lint run ./cmd/cli/... ./internal/credential/... ./internal/config/...` -> 0 issues.
- Site: `npm ci --ignore-scripts && npm run build` -> 5 pages built; existing i18n/404 warnings only.
- Full local config suite: `go test ./internal/config -count=1` -> passed on Windows after #392 removed the reserved-port dependency.
- Linux race dependency #386 is already in the review base; `-race` was not rerun in this Windows-only closure pass.
- No regressions in the directly changed packages: yes.

### Feature verification command sweep

The round-4 sweep ran every command with `-v`. Final remediation replaces f4's hand-maintained regex with full relevant package suites.

- f1 -> `TestHardenCredentialPathUsesOwnerOnlyWindowsACLs`, `TestStoreSetGetAndList`, `TestStoreRefusesOverwriteUnlessExplicit`, `TestStoreFailedAtomicReplacePreservesExistingCredential`, `TestStoreValidatesNamesAndKeysWithoutLeakingValues`, `TestStoreRemove`, `TestStoreUsesOwnerOnlyPermissionsOnUnix` (skipped on Windows), `TestCredentialStoreDirIsSiblingOfProfileStore`.
- f2 -> `TestStoreCredentialReference`, `TestStoreCredentialReferenceIsBackwardCompatibleAndPreserved`, `TestStoreClearCredentialAndListProfiles`, `TestStoreSetCredentialValidatesProfileAndCredential`.
- f3 -> `TestAuthCommandIsRegistered`, `TestAuthSetStoresMaskedCredentialAndBindsProfile`, `TestAuthSetRefusesOverwriteUnlessForced`, `TestAuthSetReadsExplicitStdinWithoutPrintingCredential`, `TestAuthListStatusAndRemoveNeverPrintSecrets`, `TestAuthRemoveProtectsSharedCredential`, `TestAuthSetRejectsHeadscaleKeyForSaaSProfile`, `TestAuthDefaultsUsePerUserPaths`, `TestAuthKeyFilePrecedence`, `TestAuthKeyFlags_WarnProcessList`.
- f4 -> full `./cmd/cli` and `./internal/config` suites. This includes connect/browser profile resolution, `TestConnectExplicitCredentialSourcesOverrideProfile`, `TestConnectAuthKeyFileOverridesMissingProfileCredential`, `TestConnectEnvOverridesMissingProfileCredential`, browser warning/file precedence, `TestPrecedenceProfileAuthKeyIsBelowEnvironmentAndFlags`, and config validation wiring.
- f5 -> `TestManagedCredentialDocumentation`.
- Sweep result -> f4 no longer depends on a hand-maintained test-name regex, eliminating the repeated omission class while keeping f1, f2, f3, and f5 focused on their package-specific contracts.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- The managed file name is `<validated-name>.key`; profile YAML stores only the validated name.
- Windows uses the built-in `icacls` utility to remove inheritance and grant only the current user; Unix uses `0700` directories and `0600` files.
- Browser keeps its stronger security boundary: an explicit key file wins, otherwise a managed profile credential wins and any `TS_AUTHKEY` value is ignored with a warning.

## Review dispositions

- Major (round 2) — `features.json` f1 used a broad `-run` expression that did not select credential-store behavior tests. Applied then and superseded by the comprehensive round-3 command below.
- Minor (theoretical) — credential write and profile binding are not one transactional operation. Deferred to #391, which already tracks transactional credential/profile updates; no code change in this review-gate fix.
- Minor (theoretical) — concurrent writers can race between overwrite detection and replacement. Deferred to #391, which already tracks credential-store concurrency; no code change in this review-gate fix.
- Major (round 3) — browser silently resolved a managed profile credential above `TS_AUTHKEY` while the general precedence documentation said otherwise. Applied: ADR-014 remains authoritative, the browser emits an explicit warning, the named test pins managed-over-environment behavior, and proposal/configuration documentation records the exception.
- Major (round 3) — atomic replacement had no deterministic failure-path proof. Applied: a per-store rename seam drives `TestStoreFailedAtomicReplacePreservesExistingCredential`, which asserts byte-identical preservation and no temporary-file residue.
- Major (round 3) — f1 omitted normal store operations, validation, removal, and rollback. Applied: the command now selects every named store behavior plus platform permission and sibling-path tests; a `ValidateName` mutation was observed failing the gate.
- Minor (git trailer) — declined: this session explicitly requires the Copilot trailer and forbids rewriting the existing branch history to remove it.
- Minor (stale blockers) — applied: BUG-024/#392 and BUG-023/#386 claims above now reflect the merged fixes and the validation actually run.
- Minor (effective Windows ACL proof) — deferred: the deterministic unit test verifies the exact owner-only `icacls` invocation and production fails if it cannot execute; effective ACL inspection is host-policy-dependent integration evidence, not a stable cross-platform unit test.
- Minor (shared-credential force overwrite) — deferred to #391, which covers serialized shared credential/profile transitions and overwrite concurrency.
- Minor (read-time permission enforcement) — deferred: this feature enforces owner-only permissions during every managed write and fails loudly on enforcement errors; detecting external ACL/mode tampering on every read is optional platform-specific defense in depth outside the accepted write contract.
- Minor (implicit credential naming) — deferred to #399, whose profile-first migration and documentation work must reconcile profile display names with managed credential identifiers.
- Major (round 4) — f4 did not select `TestConnectExplicitCredentialSourcesOverrideProfile`. Applied: f4 now selects it; the review's connect-precedence mutation made the corrected command fail in the `environment` subtest, and restoring production made it pass.
- Minor (round 4 warning branch) — applied: `TestBrowserAuthKeyFileOverridesProfileCredential` now captures stderr and asserts no ignored-environment warning. Removing the `authKeyFile != ""` guard made the test fail; restoring it made the test pass.
- Major (round 5 missing credential) — applied: `TestConnectEnvOverridesMissingProfileCredential` mirrors the file-source variant and pins the environment guard when the referenced managed credential file is absent.
- Major (round 5 f4 gate) — applied: f4 now runs the complete `cmd/cli` and `internal/config` suites. The config-precedence mutation makes this exact command red, and restored production is green.
- Minor (round 5 validation scope) — #398 owns auth-key validation semantics. The full f4 config suite now covers its config-layer wiring because that code is present in the reviewed range, without redefining UX-355 as the semantic owner.
- Major (final validator divergence) — applied: init now delegates structural key validation to `credential.ValidateKey`, matching config/connect and #398 semantics while retaining the login-URL remediation hint. Malformed prefixed keys are rejected before configuration is emitted.
- Minor (final onboarding guidance) — applied: profile initialization now gives the concise sequence `auth set --profile` then `connect --profile`; file-based config initialization keeps its existing `--auth-key-file` guidance and never shows profile-only instructions.- Round 8 (preserved `.env` layer) — **applied in scope.** YAML mode emits a `.env` beside the config, and `env > yaml` made a *preserved* `TS_TARGET` / `TS_CONTROL_URL` the effective value while `init` validated only the flags, so a populated directory still reached the round-7 outcome. `buildEnvContent` now writes both values from the flags it validated; `effectiveControlURL` reads a preserved `TS_CONTROL_URL` when no flag was given (a user's own control plane, which must be validated rather than clobbered) and `writeConfig` validates it with `config.ValidateControlPlane` before anything is written. `envfile.Values` is the single `.env` parser, replacing the copy `buildEnvContent` carried. The non-obvious-keys list in that function was also written twice; it is now one ordered slice, with unknown keys emitted in sorted order instead of map order.
- Round 8 (interactive wizard, Minor) — surfaced only: the wizard does not prompt for a control URL, so an `hskey-*` key typed at the prompt is refused with the flag's name and no in-session way to supply it. Fail-loud; a prompt is a UX change outside this review's gating scope.
- Round 8 (git trailer, read-time permissions, #391, #399, f1-regex durability) — carried unchanged from earlier rounds, no further action before archive.

- Round 7 (Headscale control-plane divergence) — **applied in scope.** `writeConfig` now validates the auth-key/control-plane pairing with `config.ValidateControlPlane` (the same validator `Merge` applies on read, exported for this purpose) before either writer runs, so init in env/yaml mode either emits a configuration connect accepts or refuses and writes nothing; `--control-url` is honored in env/yaml mode and emitted as `TS_CONTROL_URL` (env) / `control_url` (yaml). The `configuration.md` over-claim was replaced with the actual rule and pinned by the doc guard. #401 keeps its own four `--auth-key-file` items, which are unrelated to this pairing.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? no: ADR-015 and the executable tests capture the non-obvious security boundaries.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: `docs/adr/adr-015-profile-scoped-credential-management.md`
- [x] New pattern candidate for `00_meta/patterns/`? no: the provider/profile integration remains ts-bridge-specific.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/UX-355-managed-credentials/` -> `specs/archive/UX-355-managed-credentials/`
- [x] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
