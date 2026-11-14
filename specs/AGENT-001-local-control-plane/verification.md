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

- [ ] AC1 -> `TestStartRefusesASecondAgent`, `TestStartAfterCloseSucceeds`, `TestStartReplacesAStaleSocket`,
  `TestStartKeepsTheSocketOfARunningAgent`, `TestStartNeverRemovesANonSocket`
- [ ] AC2 -> `TestDefaultDir`, `TestPrepareDirModes`, `TestSocketModeIsOwnerOnly`, `TestStartRefusesALongSocketPath`
- [ ] AC3 -> `TestAuthorize` (fake uid/SID), `TestListenerRefusesAPeerThatIsNotTheOwner`,
  `TestListenerKeepsServingAfterARefusal`
- [ ] AC4 -> `TestStatusOverTheRealTransport`, `TestPeerIdentityIsTheCurrentUser`, `TestAgentCommandServesUntilInterrupted`
- [ ] AC5 to AC7 -> later PRs (see `tasks.md`)

## Test status

(filled at the head of each PR)

## Decisions made during implementation

(filled as they happen)

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons/`? to decide after PR 4
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: ADR-018 records the architecture; the ADR-007 table row is updated in PR 1
- [ ] New pattern candidate for `00_meta/patterns/`? no

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/AGENT-001-local-control-plane/` -> `specs/archive/AGENT-001-local-control-plane/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
