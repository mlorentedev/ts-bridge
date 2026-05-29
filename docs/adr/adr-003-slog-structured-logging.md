---
id: "ADR-003-slog-structured-logging"
type: adr
status: accepted
date: "2024-06-01"
tags: [architecture, decision, observability, logging]
owner: manu
created: "2026-03-28"
---

# ADR-003: Use log/slog for Structured Logging

## Context
Early versions used `fmt.Printf` and `log.Printf` for output. As the project matured (v1.2.0), structured logging became necessary for:
- JSON output for SIEM/log aggregation tools
- Consistent key-value pairs (client, duration, bytes_tx, bytes_rx)
- Log level filtering (debug for tsnet internals, info for connections)

## Options Considered
1. **fmt.Printf / log.Printf**
    * *Pros:* Zero dependencies, simple.
    * *Cons:* No structure, no levels, no JSON, hard to parse programmatically.
2. **log/slog (stdlib, Go 1.21+)**
    * *Pros:* Stdlib (no dependency), structured key-value, text/JSON handlers, level filtering, handler extensibility.
    * *Cons:* Go 1.21+ required, slightly more verbose API.
3. **zerolog / zap**
    * *Pros:* Battle-tested, high performance, rich features.
    * *Cons:* External dependency (violates stdlib-first principle), overkill for this project's scale.

## Decision
We chose **log/slog** because it is stdlib (aligned with the zero-dependency philosophy), provides structured JSON/text output, and has log levels. The `TS_LOG_FORMAT=json` env var switches between text and JSON handlers.

## Consequences
- **Positive:** Zero external dependencies for logging, consistent structured output, JSON mode for production log pipelines, debug level for tsnet internals.
- **Negative:** Requires Go 1.21+ (acceptable — project already targets 1.21+).

## References
- https://pkg.go.dev/log/slog
- `main.go:114-129` — `initLogger()` implementation
