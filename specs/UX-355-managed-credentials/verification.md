---
tags: [spec, verification, templates]
created: "2026-09-29"
---

# Verification - UX-355-managed-credentials

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [ ] Criterion 1 -> `internal/credential` store tests cover atomic set and overwrite behavior; CLI onboarding remains pending.
- [x] Criterion 2 -> `TestStoreCredentialReference`, backward-compatibility/preservation, clear/list, and validation tests.
- [ ] Criterion 3 -> command integration pending.
- [ ] Criterion 4 -> auth command integration pending.
- [x] Criterion 5 -> `TestCredentialStoreDirIsSiblingOfProfileStore`, Unix mode test, and Windows `icacls` argument test.

## Test status

- Foundation packages: `go test ./internal/credential ./internal/profile` -> passed.
- Path contract: `go test ./internal/config -run TestCredentialStoreDirIsSiblingOfProfileStore -count=1` -> passed.
- Static checks: `go vet ./internal/credential ./internal/profile ./internal/config` -> passed.
- Full local config suite is currently blocked by BUG-024: Windows reserves the test's hardcoded 62000 range.
- No regressions in the directly changed packages: yes.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- The managed file name is `<validated-name>.key`; profile YAML stores only the validated name.
- Windows uses the built-in `icacls` utility to remove inheritance and grant only the current user; Unix uses `0700` directories and `0600` files.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [ ] Lesson for the repo's `docs/lessons/`? <yes: path / no: reason>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes: path / no: reason>
- [ ] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. <yes: path / no: reason>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/UX-355-managed-credentials/` -> `specs/archive/UX-355-managed-credentials/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
