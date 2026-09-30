---
tags: [spec, verification, templates]
created: "2026-09-29"
---

# Verification - UX-355-managed-credentials

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] Criterion 1 -> credential store tests plus `TestAuthSetStoresMaskedCredentialAndBindsProfile` and overwrite tests.
- [x] Criterion 2 -> `TestStoreCredentialReference`, backward-compatibility/preservation, clear/list, and validation tests.
- [x] Criterion 3 -> connect/browser profile credential tests plus config precedence tests.
- [x] Criterion 4 -> auth list/status/remove and shared-reference protection tests.
- [x] Criterion 5 -> `TestCredentialStoreDirIsSiblingOfProfileStore`, Unix mode test, and Windows `icacls` argument test.

## Test status

- Foundation packages: `go test ./internal/credential ./internal/profile` -> passed.
- Path contract: `go test ./internal/config -run TestCredentialStoreDirIsSiblingOfProfileStore -count=1` -> passed.
- Static checks: `go vet ./internal/credential ./internal/profile ./internal/config` -> passed.
- CLI regression set: `go test ./cmd/cli -run 'TestAuth|ProfileCredential|BrowserAuthKeyFileOverrides|TestNewRootCmdContainsProductionCommands|TestManagedCredentialDocumentation' -count=1` -> passed.
- Build/vet: `go build ./...` and `go vet ./...` -> passed.
- Lint: scoped `golangci-lint` -> 0 issues (one stale-worktree processor warning only).
- Site: `npm ci --ignore-scripts && npm run build` -> 5 pages built; existing i18n/404 warnings only.
- Full local config suite is currently blocked by BUG-024: Windows reserves the test's hardcoded 62000 range.
- Full Linux race validation depends on BUG-023 / PR #386, which fixes a pre-existing test-handler drain race affecting unrelated PRs.
- No regressions in the directly changed packages: yes.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- The managed file name is `<validated-name>.key`; profile YAML stores only the validated name.
- Windows uses the built-in `icacls` utility to remove inheritance and grant only the current user; Unix uses `0700` directories and `0600` files.
- Browser keeps its stronger security boundary: it accepts an explicit key file or managed profile credential, not the legacy environment key alone.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? no: ADR-015 and the executable tests capture the non-obvious security boundaries.
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: `docs/adr/adr-015-profile-scoped-credential-management.md`
- [x] New pattern candidate for `00_meta/patterns/`? no: the provider/profile integration remains ts-bridge-specific.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/UX-355-managed-credentials/` -> `specs/archive/UX-355-managed-credentials/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
