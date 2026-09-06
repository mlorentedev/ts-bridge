---
id: lesson-026-multi-dimensional-audits-4-agents-converge-fa
type: lesson
status: active
created: "2026-05-28"
owner: manu
tags: [ts-bridge, lesson, audit, agents, process, code-quality]
---

# Multi-dimensional audits: 4 agents converge faster than 1 agent on 4 passes

**Context:** Ran 4 parallel subagents (Scalability, UX, DX, SOLID) on the v1.7.0 codebase in one shot. Each prompt was self-contained with a specific reading list + output format constraint.

**Finding:** All four agents independently returned **PASS-WITH-GAPS** verdicts. The convergence is itself a signal: the codebase has consistent quality across dimensions (not lopsided e.g. "great scalability but terrible UX"). Cross-cutting findings emerged that no single audit would have caught:
- The same "stale CLAUDE.md" problem manifested as DX-1 (stale main.go line count) and UX-1 (README hardcoded port that contradicts auto-mode default) — the audits surfaced it as two facets of one underlying "docs lag code" issue.
- Scalability TOCTOU on MaxConnections + Half-close not honored both come from the same root: the original implementation focused on happy-path; under-load edge cases were deferred.

Single-agent multi-pass audits in series would have produced a similar findings list but missed the convergence signal. Cost: 4 parallel agents = ~3-4× one agent's tokens, but ~1× wall time and a free triangulation check.

**Rule:** For project maturity assessments, prefer parallel multi-dim agents over single-agent comprehensive sweeps. Watch for convergence: when verdicts and root causes overlap across dimensions, the underlying issues are systemic and worth fixing first.

**Tags:** `#audit` `#agents` `#process` `#code-quality`
