---
id: "CFG-002"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-06-23"
issue: "ts-bridge#213"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CFG-002: Host-emitted shareable connection profile

> **Naming**: file lives at `<repo>/specs/<feature-id>/proposal.md`. `<feature-id>` is `AREA-NNN-slug` (e.g. `TOOL-001-secret-drift`).

## Why

<!-- from issue #213: CFG-002: Host-emitted shareable connection profile — propagate detected service port to clients -->

ts-bridge clients have no way to learn a host's real service port. The host detects it — `host setup` reads the RDP port from the registry (e.g. `45000`) — but the client hardcodes/guesses `TS_TARGET=host:3389` and `discover` defaults `--port` to `3389`. When a host's RDP port differs from 3389 (a common hardening step), `connect` dials the wrong port and fails with `connection refused`, forcing the operator to reverse-engineer the port by hand. Without this, every non-default-port host is a manual debugging session.

## What

1. `host setup` (and `host check`) emit a shareable connection descriptor that includes the **detected** service port — e.g. `tsb://acemagic-office:45000?cp=saas` — printed and/or written to a file.
2. The client gains an **import** path that materializes a named profile from the descriptor (composing with the #185 profiles model), so `connect --profile <name>` dials the correct port with no manual entry.
3. `discover` captures the correct port for the selected host (prompt / value carried by the descriptor) instead of silently defaulting to `3389`.

## Out of scope

- **Taildrop / control-plane push transfer (T2).** Investigated and demoted in ADR-011 (gated on `CapabilityFileSharing`, Headscale-fragile); at most a later Tailscale-only convenience, not this PR.
- **Option A (publish port via Tailscale posture attributes) and Option C (force a fixed RDP port).** Both rejected in ADR-011; not implemented here.
- **The full #185 named-profiles / multi-control-plane engine.** CFG-002 emits and imports a descriptor and writes the minimal profile entry it needs; it does not build the complete structured-config model.

## Risks / open questions

- **[RESOLVED] Descriptor format & versioning.** Two encodings of one descriptor: a copy-paste token `tsb://host:port?cp=<saas|control-url>` and a file form (`.tsbprofile`) that is a #185 profile fragment. Evolution is **additive / tolerant-reader first** — unknown params/keys are ignored (per `otpauth://` and connection-string practice); a `v` / `descriptor_version` field is **optional, default 1**, bumped only on a breaking change. The scheme stays stable (never version via the scheme).
- **[RESOLVED] Coupling to #185.** CFG-002 ships a minimal profile store using the **#185-shaped schema** (`profiles: { <name>: { target, control_url } }`) plus a `descriptor_version` (apiVersion-style). The client is a **tolerant reader** (ignores unknown keys), so a profile written by #185 with extra fields does not break CFG-002 and vice versa — no migration, and CFG-002 ↔ #185 can land in either order. Validated against schema-evolution practice (forward compatibility / expand-contract).
- **[non-blocking] Descriptor is non-secret.** It carries host + port + control-plane, no auth key — so the transfer channel needs no confidentiality. Confirm nothing sensitive leaks in (e.g. a Headscale control URL is infra-identifying but not a secret).

## Acceptance criteria

- [ ] `host setup` / `host check` output includes a descriptor that carries the **detected** port — unit-tested with the 45000 case (mocked registry `PortNumber=45000` → descriptor string contains `45000`, not `3389`).
- [ ] Importing a descriptor produces a profile whose target uses the descriptor's port; `connect --profile <name>` dials that port — round-trip test (`import "tsb://h:45000?cp=saas"` → profile target == `h:45000`).
- [ ] Descriptor round-trips losslessly (`parse(emit(x)) == x`) including a version field; a malformed descriptor is rejected with a clear, non-panicking error.
- [ ] Backward compatibility: existing `TS_TARGET=host:port` keeps working with no profile present (existing connect path unchanged).
- [ ] Cross-platform emission: the non-Windows `host` paths (Linux/macOS) also produce a descriptor with their detected port (not Windows-only).
- [ ] Control-plane parity: a descriptor with `cp=<Headscale URL>` imports and connects identically to `cp=saas` (closes the ADR-005 loop).
- [ ] Idempotent import: importing the same descriptor twice yields a single, uncorrupted profile.
- [ ] `.env.example`, `README`, and `docs/` document the `import` flow and the updated `discover --port` behavior.

## References

- Bitácora board: ts-bridge#213 (CFG-002)
- Related ADR: `docs/adr/adr-011-shareable-connection-profile.md`
- Related patterns: `00_meta/patterns/<pattern>.md` (if any)
- Related issue: #185 (CFG-001 profiles model — composes)
