---
id: lesson-022-first-iteration-sdd-overhead-is-fixed-second-
type: lesson
status: active
created: "2026-05-01"
owner: manu
tags: [ts-bridge, lesson, sdd, process, workflow, meta]
---

# First-iteration SDD overhead is fixed; second iteration is trivially cheap

**Context:** First adoption of `pattern-spec-driven-development.md` in ts-bridge (REL-003 + ARCH-004) within a single session. The first spec required ~30 min of scaffolding (proposal + tasks + verification + ts-bridge/CLAUDE.md opt-in pointer); the second spec took <5 min by copying REL-003's structure.

**Finding:** SDD's cost is amortized across specs within a project. The expensive setup is the first time:
- Reading the pattern + templates
- Adapting templates to the project's existing standards (TDD, conventional commits)
- Adding the project-level opt-in pointer
- Scaffolding the first \`specs/<id>/\` and getting comfortable with the proposal/tasks/verification rhythm

After that, opening a new spec is a copy-and-tweak of the previous one. The "thinking tool" payoff (catching ARCH-005 → ARCH-004 collapse during Q3 analysis, before any code was written) was already realized in the first spec.

**Rule:** Don't evaluate SDD by the cost of the first spec — it's amortized. Evaluate by whether the proposal-first discipline caught at least one bad assumption before code was written. In this session: yes, twice (REL-003 chose A1-per-direction over A2-shared-state with rationale; ARCH-004 confirmed ARCH-005 collapse before opening a redundant PR).

**Tags:** `#sdd` `#process` `#workflow` `#meta`
