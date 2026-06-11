---
id: "adr-009"
type: adr
status: proposed
date: "2026-06-10"
tags: [architecture, decision, toolchain, go, tsnet]
owner: manu
---

# ADR-009: Update Go Toolchain to 1.25 and tsnet to Latest

## Context

Current pinned versions:
- Go: 1.24 (toolchain go1.24.0) — Go 1.25 released March 2026
- tailscale.com/tsnet: v1.80.0 (February 2026)

AGENTS.md references "Go 1.25+" but go.mod says 1.24 — documentation drift that causes confusion for contributors and AI agents.

## Options Considered

1. **Stay on Go 1.24 / tsnet v1.80.0** — Stable, known working. But accumulating security risk and CI drift.
2. **Update to Go 1.25 + latest tsnet** — Current stable. Go guarantees backward compatibility within 1.x. tsnet API has proven remarkably stable (lessons.md: v1.60→v1.80 required zero code changes).
3. **Update Go only, keep tsnet pinned** — Partial update, leaves tsnet drift unaddressed.

## Decision

**Update to Go 1.25 and latest tsnet in a single PR.**

Go 1.25 brings improved iterators, crypto/tls improvements, and compiler optimizations. The risk is low: Go's compatibility promise and tsnet's track record both support a clean upgrade.

## Consequences

### Positive

- Current toolchain, current security patches
- AGENTS.md matches reality
- CI uses the same Go version as local development

### Negative

- Minor risk of tsnet API changes (historically zero, but not guaranteed)
- go.sum churn from dependency updates

### Neutral

- No code changes expected — this is a go.mod + go.sum update only
- CI image must be updated to use Go 1.25

## References

- lessons.md 2026-02-27: "tsnet API is remarkably stable"
- https://go.dev/doc/go1.25
- Issue #39