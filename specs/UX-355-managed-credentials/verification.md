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
- Static checks: `go vet ./cmd/cli ./internal/credential ./internal/config` -> passed.
- CLI regression set: `go test ./cmd/cli -run 'TestAuth|ProfileCredential|BrowserAuthKeyFileOverrides|TestNewRootCmdContainsProductionCommands|TestManagedCredentialDocumentation' -count=1` -> passed.
- Build/vet: `go build ./...` and `go vet ./...` -> passed.
- Lint: `golangci-lint run ./cmd/cli/... ./internal/credential/... ./internal/config/...` -> 0 issues.
- Site: `npm ci --ignore-scripts && npm run build` -> 5 pages built; existing i18n/404 warnings only.
- Full local config suite: `go test ./internal/config -count=1` -> passed on Windows after #392 removed the reserved-port dependency.
- Linux race dependency #386 is already in the review base; `-race` was not rerun in this Windows-only closure pass.
- No regressions in the directly changed packages: yes.

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
