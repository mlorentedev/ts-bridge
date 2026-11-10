---
tags: [spec, verification]
created: "2026-11-06"
id: "SEC-391-store-lock"
type: spec-verification
status: active
owner: manu
---

# Verification - SEC-391-store-lock

## Evidence

- [ ] AC1 -> `TestConcurrentWritersLoseNoProfile` (24 goroutines), `TestConcurrentProcessesLoseNoProfile`
  (three writer processes beside this one), `TestUpdateHoldsOffAnotherWriterUntilItEnds`.
  The first was red on master before the lock: it left 2 to 4 of 24 profiles and once a YAML
  duplicate-key parse error.
- [ ] AC2 -> `TestBindCredentialExclusiveCreationHasOneWinner`.
- [ ] AC3 -> `TestBindCredentialRefusesAMovedControlPlaneBeforeReplacingTheKey`,
  `TestBindCredentialUndoesTheKeyWhenTheAssociationFails`,
  `TestUnbindCredentialKeepsReferencesWhenTheKeyCannotBeDeleted`,
  `TestUnbindCredentialRerunClearsAReferenceToADeletedKey`.
- [ ] AC4 -> the tests above plus `TestUnbindCredentialSeesAReferenceAddedWhileItWaits`,
  `TestAuthSetKeepsTheStoredKeyWhenTheControlURLMovesMidCommand`, `TestStoreLockAcrossProcesses`
  (killed holder), `TestAbandonedTemporaryFileDoesNotAffectTheStore`, `TestSaveFailureKeepsThePreviousStore`.
- [ ] AC5 -> `TestStoreLockAcrossProcesses` (reads succeed, a write returns the busy error),
  `TestAuthCommandsHonourTheStoreLock`.
- Mutants (each fails the named tests):
  - the store lock replaced by a private temp file: 6 profile tests;
  - the control URL comparison disabled: the CLI test and the profile test;
  - the previous key not restored: the association test;
  - references cleared before the key is deleted: the deletion test.

## Test status

Local run at the head of PR 1 (macOS), before the CI run:

- `go test -race ./...` exit 0 (`./internal/profile` 5.0 s, `./cmd/cli` 5.8 s, `./internal` 1.1 s).
- `golangci-lint run`: 0 issues. `gosec ./...`: 0 issues.
- `GOOS=windows go vet ./...` and `GOOS=linux go vet ./...` exit 0; the Windows test binary compiles.
- `check-features-shape`, `check-features-selectors`, `check-conflict-markers`, `check-lessons` and
  `check-ai-trailers origin/master HEAD` exit 0.
- Not run locally: the Windows tests (retrying rename, helper processes). The `test-windows` CI job
  is the evidence for those.

## Decisions made during implementation

- The brief's "standard library only" came from reading `x/sys` as an indirect dependency. It is
  direct, and the ownership lock already uses it; reused (see proposal, Decisions).
- `auth remove` resolves a profile's credential inside the lock. Resolving it first, as before,
  would act on a stale reference.
- The first `Get` in `auth set` stays outside the lock, so a missing profile fails before the user
  types a key; the transition reads it again.

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons/`? no: the lock and ordering are recorded in ADR-015's amendment
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: amendment to ADR-015
- [ ] New pattern candidate for `00_meta/patterns/`? no: one occurrence

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/SEC-391-store-lock/` -> `specs/archive/SEC-391-store-lock/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
