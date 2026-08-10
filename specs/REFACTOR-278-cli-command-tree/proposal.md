---
id: "REFACTOR-278-cli-command-tree"
type: spec
status: verifying # draft | implementing | verifying | archived
created: "2026-08-10"
issue: "mlorentedev/ts-bridge#278"
tags: [spec, proposal]
template_version: "1.0"
---

# REFACTOR-278: Cli Command Tree

## Why

<!-- from issue #278: refactor(cli): rootCmd is unreachable from tests — extract a command-tree constructor -->

The global Cobra tree is assembled through `init()`, so tests cannot instantiate production with clean state and instead exercise tautological replicas. Without this refactor, `cmd/cli` remains opaque to coverage and mutation testing, and real command tests leak flag state between invocations.

## What

`cmd/cli.NewRootCmd()` returns a fresh production command tree on every call; `Execute()` delegates to a newly constructed tree; command registration and flag values are invocation-local rather than package-init side effects.

## Out of scope

- Resolving the four tautological tests tracked by #279.
- Adding exhaustive `RunE` coverage; that remains in #182.
- Changing flags, output, config precedence, or dependencies.

## Risks / open questions

- Preserve the exact command set, flag names, defaults, shorthands, and error/usage behavior.
- Migrate internal tests that currently depend on `connectCmd` or `rootCmd`.
- `NewRootCmd()` isolates Cobra state only. Existing `Runner`, `LoggerInit`, build variables, and logging globals remain out of scope, so constructed trees are not guaranteed to execute concurrently.
- No blocking open questions remain.

## Acceptance criteria

- [x] `NewRootCmd()` constructs the complete production tree with the current commands, flags, defaults, shorthands, and error/usage behavior.
- [x] Mutating args or flag state on one constructed tree does not affect a subsequent tree.
- [x] Command singletons and command-registration `init()` functions are removed, and `Execute()` executes a newly constructed tree.
- [x] External CLI tests exercise the production tree without `newTestCommand`; targeted tests, the full race suite, lint, and build pass without dependency changes.

## References

- Bitácora board: issue #278
- Related ADRs: `docs/adr/adr-010-cli-package-layout.md`, `docs/adr/adr-013-cli-tests-in-go.md`
- Related patterns: `00_meta/patterns/pattern-spec-driven-development.md`, `00_meta/patterns/pattern-testing-standards.md`
