---
tags: [spec, tasks]
created: "2026-11-06"
id: "SEC-391-store-lock"
type: spec-tasks
status: active
owner: manu
---

# Tasks - SEC-391-store-lock

The issue is split in two PRs of at most ~300 executable lines each.

## PR 1 - lock, atomic save, auth transitions (this PR, `Part of #391`)

### Setup

- [x] Branch from master: `fix/issue-391-store-lock`
- [x] `proposal.md` complete; lock primitive decided (reuse `x/sys`, already direct)

### Implementation

- [x] [AC1] Failing test: 24 concurrent `Set` calls lose profiles and can corrupt the YAML (reproduced red)
- [x] [AC1] [AC5] `lockStore` (waiting lock), `Store.Update`/`Tx`; every public mutator inside `Update`
- [x] [AC1] [AC4] Atomic `save` with a retrying rename; tests for a failed replace, an abandoned
  temporary file and the retry
- [x] [AC1] [AC4] Cross-process tests through the test binary: a held lock, a killed holder, three
  writer processes
- [x] [AC2] [AC3] [AC4] `BindCredential` with tests: control URL moved, Headscale key on SaaS,
  unknown profile, association failure (restore and remove), one winner among eight creators
- [x] [AC3] [AC4] `UnbindCredential` with tests: reference added while waiting, last reference,
  forced removal, deletion failure, rerun after an interrupted removal
- [x] [AC3] `auth set` and `auth remove` call them; `auth set` re-reads the profile under the lock
- [x] [AC4] [AC5] CLI tests: the `control_url` change during the key prompt keeps the stored key;
  mutating commands queue behind a held lock, read-only ones do not
- [x] Amend the ADR-007 package table (`profile` imports `secretfile`)

### Closing

- [x] `go test -race ./...`, `golangci-lint run`, `gosec ./...`, `GOOS=windows go vet ./...`, `GOOS=linux go vet ./...`
- [x] Mutation: no lock, no control URL re-check, no key restore, references cleared before the
  key is deleted - each fails a test
- [ ] `verification.md` evidence filled once CI is green

## PR 2 - remaining gaps (not started)

- [ ] [AC1] `init --profile` without `--force`: move the absence check inside the lock
  (`Tx.Configure`), with a test of two concurrent creators
- [ ] [AC3] Report orphan keys and dangling references (`auth list` marks them) with a repair hint
- [ ] After PR #664 lands, fold `CheckCredentialShare`/`AssociateCredential` into `BindCredential`
