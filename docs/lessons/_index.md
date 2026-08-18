---
id: ts-bridge-lessons-index
type: index
status: active
created: "2026-05-10"
owner: manu
tags: [ts-bridge, lessons, index]
---

# Lessons Learned Index

| # | Date | Title | File | Tags |
|---|---|---|---|---|
| 001 | 2026-05-01 | 2026-08-08 | [[docs/lessons/lesson-001-2026-08-08\|lesson-001-2026-08-08.md]] |  |
| 002 | 2026-05-01 | 2026-08-07 | [[docs/lessons/lesson-002-2026-08-07\|lesson-002-2026-08-07.md]] |  |
| 003 | 2026-05-01 | 2026-08-05 | [[docs/lessons/lesson-003-2026-08-05\|lesson-003-2026-08-05.md]] |  |
| 004 | 2026-05-01 | 2026-07-10 | [[docs/lessons/lesson-004-2026-07-10\|lesson-004-2026-07-10.md]] |  |
| 005 | 2026-05-01 | 2026-06-24 | [[docs/lessons/lesson-005-2026-06-24\|lesson-005-2026-06-24.md]] |  |
| 006 | 2026-05-01 | 2026-06-13 | [[docs/lessons/lesson-006-2026-06-13\|lesson-006-2026-06-13.md]] |  |
| 007 | 2026-05-01 | 2026-06-12 | [[docs/lessons/lesson-007-2026-06-12\|lesson-007-2026-06-12.md]] |  |
| 008 | 2026-05-01 | 2026-03-13 | [[docs/lessons/lesson-008-2026-03-13\|lesson-008-2026-03-13.md]] |  |
| 009 | 2026-05-01 | 2026-03-07 | [[docs/lessons/lesson-009-2026-03-07\|lesson-009-2026-03-07.md]] |  |
| 010 | 2026-05-01 | 2026-02-27 | [[docs/lessons/lesson-010-2026-02-27\|lesson-010-2026-02-27.md]] |  |
| 011 | 2026-05-01 | 2026-02-26 | [[docs/lessons/lesson-011-2026-02-26\|lesson-011-2026-02-26.md]] |  |
| 012 | 2026-05-01 | 2026-02-24 | [[docs/lessons/lesson-012-2026-02-24\|lesson-012-2026-02-24.md]] |  |
| 013 | 2026-03-07 | Managing Cyclomatic Complexity in Go projects | [[docs/lessons/lesson-013-managing-cyclomatic-complexity-in-go-projects\|lesson-013-managing-cyclomatic-complexity-in-go-projects.md]] | `go`, `refactoring`, `clean-architecture`, `ci-cd`, `linter` |
| 014 | 2026-03-13 | Corporate TLS Inspection Breaks Headscale TCP Passthrough | [[docs/lessons/lesson-014-corporate-tls-inspection-breaks-headscale-tcp\|lesson-014-corporate-tls-inspection-breaks-headscale-tcp.md]] | `headscale`, `tailscale`, `corporate-firewall`, `tls-inspection`, `networking`, `traefik`, `tcp-passthrough` |
| 015 | 2026-05-01 | 2026-03-16 | [[docs/lessons/lesson-015-2026-03-16\|lesson-015-2026-03-16.md]] |  |
| 016 | 2026-05-01 | Headscale Migration Abandoned — Tailscale SaaS is the Correct Solution for Corporate Networks | [[docs/lessons/lesson-016-headscale-migration-abandoned-tailscale-saas-\|lesson-016-headscale-migration-abandoned-tailscale-saas-.md]] | `headscale`, `tailscale`, `corporate-firewall`, `tls-inspection`, `decision` |
| 017 | 2026-05-01 | 2026-05-18 | [[docs/lessons/lesson-017-2026-05-18\|lesson-017-2026-05-18.md]] |  |
| 018 | 2026-05-01 | Ephemeral mode mandates auth-key rotation on every client (operational reality) | [[docs/lessons/lesson-018-ephemeral-mode-mandates-auth-key-rotation-on-\|lesson-018-ephemeral-mode-mandates-auth-key-rotation-on-.md]] | `tailscale`, `ephemeral`, `auth-key`, `operational` |
| 019 | 2026-05-01 | tsnet.Server.Up() partial-start: must Close() on error or Windows cleanup leaks | [[docs/lessons/lesson-019-tsnet-server-up-partial-start-must-close-on-e\|lesson-019-tsnet-server-up-partial-start-must-close-on-e.md]] | `go`, `tsnet`, `windows`, `file-locking`, `error-handling` |
| 020 | 2026-05-01 | tsnet.Server self-heals across DERP/magicsock transients via awaitRunning | [[docs/lessons/lesson-020-tsnet-server-self-heals-across-derp-magicsock\|lesson-020-tsnet-server-self-heals-across-derp-magicsock.md]] | `tsnet`, `tailscale`, `self-healing`, `design-investigation` |
| 021 | 2026-05-01 | gosec standalone vs golangci-lint: `//nolint:gosec` is not portable | [[docs/lessons/lesson-021-gosec-standalone-vs-golangci-lint-nolint-gose\|lesson-021-gosec-standalone-vs-golangci-lint-nolint-gose.md]] | `go`, `gosec`, `linter`, `ci`, `cross-tool-compatibility` |
| 022 | 2026-05-01 | First-iteration SDD overhead is fixed; second iteration is trivially cheap | [[docs/lessons/lesson-022-first-iteration-sdd-overhead-is-fixed-second-\|lesson-022-first-iteration-sdd-overhead-is-fixed-second-.md]] | `sdd`, `process`, `workflow`, `meta` |
| 023 | 2026-05-01 | Pinning the linter version surfaces accumulated deferred debt | [[docs/lessons/lesson-023-pinning-the-linter-version-surfaces-accumulat\|lesson-023-pinning-the-linter-version-surfaces-accumulat.md]] | `linter`, `golangci-lint`, `technical-debt`, `ci` |
| 024 | 2026-05-01 | Half-close + idleConn embedding: type assertions need unwrap | [[docs/lessons/lesson-024-half-close-idleconn-embedding-type-assertions\|lesson-024-half-close-idleconn-embedding-type-assertions.md]] | `go`, `interfaces`, `embedding`, `net-conn` |
| 025 | 2026-05-01 | TOCTOU on atomic counters: CAS loop or step-and-rollback | [[docs/lessons/lesson-025-toctou-on-atomic-counters-cas-loop-or-step-an\|lesson-025-toctou-on-atomic-counters-cas-loop-or-step-an.md]] | `go`, `atomics`, `concurrency`, `toctou`, `race-conditions` |
| 026 | 2026-05-01 | Multi-dimensional audits: 4 agents converge faster than 1 agent on 4 passes | [[docs/lessons/lesson-026-multi-dimensional-audits-4-agents-converge-fa\|lesson-026-multi-dimensional-audits-4-agents-converge-fa.md]] | `audit`, `agents`, `process`, `code-quality` |
| 027 | 2026-05-18 | vault_health is project-scoped — cross-project wikilinks need markdown link form | [[docs/lessons/lesson-027-vault-health-is-project-scoped-cross-project-\|lesson-027-vault-health-is-project-scoped-cross-project-.md]] | `vault`, `obsidian`, `tooling`, `wikilinks`, `hive` |
| 028 | 2026-08-13 | Windows file not found portable check | [[docs/lessons/lesson-028-windows-file-not-found-portable-check\|lesson-028-windows-file-not-found-portable-check.md]] |  |
| 029 | 2026-08-13 | os.Chdir error check in tests | [[docs/lessons/lesson-029-os-chdir-error-check-in-tests\|lesson-029-os-chdir-error-check-in-tests.md]] |  |
