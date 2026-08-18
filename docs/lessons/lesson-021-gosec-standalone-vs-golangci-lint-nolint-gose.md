---
id: lesson-021-gosec-standalone-vs-golangci-lint-nolint-gose
type: lesson
status: active
created: "2026-05-01"
owner: manu
tags: [ts-bridge, lesson, go, gosec, linter, ci, cross-tool-compatibility]
---

# gosec standalone vs golangci-lint: `//nolint:gosec` is not portable

**Context:** ARCH-004 PR #24 CI failed on `gosec` with G404 (weak RNG). The inline `//nolint:gosec` directive that suppresses golangci-lint findings did NOT suppress the standalone gosec runner.

**Why:** This project's CI runs both `golangci-lint-action@v4` (which includes a gosec module) AND `securego/gosec@v2.23.0` as a separate step. The two runners read different exclusion syntaxes:
- `golangci-lint`-style: `//nolint:gosec` (line above or end-of-line)
- standalone `gosec`-style: `// #nosec G404 -- reason` (end-of-line)

The standalone gosec runner only respects `#nosec`. golangci-lint's gosec respects both, so `#nosec` is the portable choice when both runners exist.

**Rule:** For any project that runs gosec both via golangci-lint AND as a separate CI step, use `// #nosec` everywhere. Document the reason inline. `.golangci.yml` exclusions only apply to the golangci-lint pass and do not bind the standalone runner.

**Tags:** `#go` `#gosec` `#linter` `#ci` `#cross-tool-compatibility`
