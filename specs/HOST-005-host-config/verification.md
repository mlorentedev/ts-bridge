---
tags: [spec, verification, templates]
created: "2026-10-30"
---

# Verification - HOST-005-host-config

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [ ] AC1 (schema, loader and `--config`) -> commit `<hash>` / test `<name>`
- [ ] AC2 (precedence, `--firewall-rule` unset) -> commit `<hash>` / test `<name>`
- [ ] AC3 (no per-user store read, C9) -> commit `<hash>` / test `<name>`
- [ ] AC4 (per-OS capability errors) -> commit `<hash>` / test `<name>`
- [ ] AC5 (per-OS defaults) -> commit `<hash>` / test `<name>`
- [ ] AC6 (`control_url` in the descriptor) -> commit `<hash>` / test `<name>`
- [ ] AC7 (`host init` writes the YAML) -> commit `<hash>` / test `<name>`
- [ ] AC8 (`host init` overwrite and unreadable-path refusal) -> commit `<hash>` / test `<name>`

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
- [ ] Folder moved: `specs/HOST-005-host-config/` -> `specs/archive/HOST-005-host-config/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
