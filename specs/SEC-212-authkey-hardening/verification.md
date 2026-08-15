---
id: "SEC-212-authkey-hardening"
type: spec
status: draft
created: "2026-08-14"
issue: "ts-bridge#212"
tags: [spec, verification, security, hardening]
template_version: "1.0"
---

# Verification - SEC-212: Auth Key Security Hardening

## Evidence Checklist

- [x] `go test -race ./...` passing.
- [x] `golangci-lint run` clean (0 issues).
- [x] `gosec ./...` clean (0 issues).
- [x] Verification of `ts-bridge init --help` and generated configs security notes.
- [x] Documentation updated across README.md, .env.example, runbooks, and security audit.
