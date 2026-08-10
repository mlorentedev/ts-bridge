---
tags: [spec, tasks, templates]
created: "2026-08-10"
---

# Tasks - REFACTOR-278-cli-command-tree

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers**:
> - `[P]` - this task has no dependency on another unchecked task.
> - `[AC<n>]` - this task helps satisfy acceptance criterion `<n>` from `proposal.md`.

## Setup

- [x] Branch created from main: `mlorentedev-278-cli-command-tree`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions"

## Implementation

- [x] [AC1] [AC4] Replace the external command-tree replica with failing tests against `NewRootCmd()`.
- [x] [AC2] Add a failing test proving independently constructed trees do not share flag state.
- [x] [AC1] [AC2] [AC3] Implement root and subcommand constructors; remove command-registration side effects.
- [x] [AC1] [AC2] [AC4] Migrate internal tests from command singletons to fresh constructors.
- [x] [AC1] [AC3] Amend ADR-010 so its package-layout narrative matches the constructor-based production tree.

## Closing

- [x] Every acceptance criterion from `proposal.md` is covered by at least one test
- [x] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command
- [x] Type checks pass
- [x] Lint passes
- [x] No unrelated changes in the diff (no scope creep)
- [x] `verification.md` filled in
- [ ] PR opened referencing this spec folder

## Machine-readable features

This spec emits a sibling `features.json`. Each acceptance criterion maps to at least one feature with `id`, `behavior`, `verification`, `state`, and `evidence`.

**Pass-state gating:** the agent cannot write `"state": "passing"`; only the harness may set that state after capturing successful evidence.
