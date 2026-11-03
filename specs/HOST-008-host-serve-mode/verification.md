---
tags: [spec, verification, templates]
created: "2026-10-30"
---

# Verification - HOST-008-host-serve-mode

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [ ] AC1 (enrol, READY enrolled=true, live SSH/RDP handshake) -> commit `<hash>` / test `<name>` / live run
- [ ] AC2 (restart without the key, enrolled=false, one node) -> commit `<hash>` / test `<name>` / live run
- [ ] AC3 (only declared ports answer) -> commit `<hash>` / test `<name>`
- [ ] AC4 (bootstrap and control-session loss, retry, recovery) -> commit `<hash>` / test `<name>` / live run
- [ ] AC5 (host.mesh keys, state_dir and auth_key_file guards) -> commit `<hash>` / test `<name>`
- [ ] AC6 (host check published, trusted responder) -> commit `<hash>` / test `<name>`
- [ ] AC7 (untagged node refused) -> commit `<hash>` / test `<name>`

## Test status

- Test suite: `<command> -> <output / coverage %>`
- Manual smoke test: what was exercised, what was observed
- No regressions in existing test suite: yes / no (if no, document)

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

-
-

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [ ] Lesson for the repo's `docs/lessons/`? <yes: path / no: reason>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes: path / no: reason>
- [ ] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. <yes: path / no: reason>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/HOST-008-host-serve-mode/` -> `specs/archive/HOST-008-host-serve-mode/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
