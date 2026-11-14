---
tags: [spec, verification]
created: "2026-11-14"
id: "AGENT-001-local-control-plane"
type: spec-verification
status: active
owner: manu
---

# Verification - AGENT-001-local-control-plane

## Evidence

- [x] AC1 -> `TestStartRefusesASecondAgent`, `TestStartAfterCloseSucceeds`, `TestStartReplacesAStaleSocket`,
  `TestStartNeverRemovesANonSocket`
- [x] AC2 -> `TestPathsFor`, `TestDefaultPathsIgnoreTheEnvironment`, `TestRunUserUsable`, `TestPrepareDirModes`,
  `TestVerifyDirRefusesAForeignOwner`, `TestSocketModeIsOwnerOnly`, `TestStartRefusesASocketPathAtTheLimit`
- [x] AC3 -> `TestAuthorize` (fake uid/SID), `TestListenerRefusesAPeerThatIsNotTheOwner`,
  `TestListenerKeepsServingAfterARefusal`, `TestPeerIdentityIsTheCurrentUser`
- [x] AC4 -> `TestClientRefusesAnUntrustedEndpoint`
- [x] AC5 -> `TestMetaAndStatusOverTheRealTransport`, `TestErrorsUseTheEnvelope`, `TestAgentCommandServesUntilInterrupted`
- [ ] AC6 to AC9 -> later PRs (see `tasks.md`)

## Test status

PR 1, local (macOS arm64, Go 1.27.3): `go test -race ./...` green; `golangci-lint run` 0 issues;
`gosec ./...` (v2.30.0) 0 issues; `GOOS=windows` and `GOOS=linux` `go vet ./...` clean. Mutations, each
killed by a test: `Authorize` always true; socket left at the umask; client server check skipped; client
directory check skipped; a refused starter rebinding the socket before giving up.

PR 1, CI at `dec3716` (#793): `test` (Linux, `-race`), `test-macos`, `test-windows`, `lint`, `security`,
`hygiene`, `smoke` and the six-target `build-matrix` all green.

## Decisions made during implementation

- **PR 1 is over the ~300-line cap (about 475 executable lines, 40 of them the mechanical `oslock`
  move).** The server's peer check and the client's server check are two halves of one trust
  boundary (ADR-018 §2); split across PRs, each half would be reviewed without the other. Windows was
  split out instead, which is the cut that removes code nobody can run locally.
- **The `/run/user/<uid>` probe requires mode 0700 exactly**, the mode systemd-logind creates, and
  falls back to `/tmp/ts-bridge-<uid>` otherwise; the fallback directory gets the same checks.
- **The client checks the socket directory with no tightening** (`checkPrivateDir`): only the agent
  creates or repairs it, and a client that tightened a directory it found loose would be trusting it.

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons/`? to decide after PR 5
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-018 records the architecture; the ADR-007 table is updated in PR 1
- [ ] New pattern candidate for `00_meta/patterns/`? no

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/AGENT-001-local-control-plane/` -> `specs/archive/AGENT-001-local-control-plane/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
