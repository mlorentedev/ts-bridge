---
id: "CFG-001"
type: spec
status: draft
created: "2026-06-26"
issue: "ts-bridge#185"
tags: [spec, proposal, config, profiles]
template_version: "1.0"
---

# CFG-001: Config And Profiles Model

<!-- from issue #185: Config & profiles model: structured config with named profiles (CFG-001, supersedes ADR-002) -->

## Why

`ts-bridge` solo puede representar un destino a la vez — el `.env` es una configuración plana sin concepto de perfil. Cuando el usuario opera dos tailnets distintos (`work` → Tailscale SaaS, `kubelab` → Headscale auto-hosted), tiene que mantener dos copias de `.env` y intercambiarlas manualmente, con riesgo de usar la auth key equivocada en el tailnet equivocado. Si no se implementa, el path hacia multi-tailnet (issue #186) y el uso diario con dos redes queda bloqueado por fricción operativa.

## What

`ts-bridge init --profile <name>` escribe un bloque en `~/.config/ts-bridge/config.yaml` con los parámetros no-secretos del perfil (control URL, local addr, target). `ts-bridge connect --profile <name>` carga ese perfil y lo fusiona con la precedencia `flags > env > profile > defaults`. Sin `--profile`, el comportamiento actual (`.env`) queda intacto — backward compatible.

## Out of scope

- Secrets (`tskey-*` / `hskey-*`) en el config file — permanecen en `--auth-key-file` / `.env`.
- UI para listar/borrar perfiles (`ts-bridge profile list/rm`) — segunda PR.
- Multi-target simultáneo (issue #186) — CFG-001 es el prerequisito, no el feature completo.

## Risks / open questions

- **[RESOLVED]** Path de `config.yaml`: usar `os.UserConfigDir()` (Go stdlib) + subdir `ts-bridge/` — devuelve `%APPDATA%` en Windows y `$XDG_CONFIG_HOME` (o `~/.config`) en Linux/macOS. Sin `//go:build` por plataforma.
- **[RESOLVED]** Compatibilidad `config.yaml` ↔ descriptor `tsb://`: representaciones distintas con propósito distinto (persistencia local vs. URL de un solo uso para compartir). El conversor ya existe: `ts-bridge import <tsb://...> --profile <name>` (PR #228) escribe del descriptor al perfil. No se necesita conversor adicional.
- **[OPEN]** `goconst` puede dispararse al añadir nuevas string literals en el mismo paquete — extraer constantes desde el principio para no romper CI lint.

## Acceptance criteria

- [ ] `ts-bridge init --profile work` escribe el perfil en `config.yaml`; re-ejecutar con `--overwrite` actualiza; sin `--overwrite` falla con error claro.
- [ ] `ts-bridge connect --profile work` carga el perfil; un flag `--target` explícito lo sobreescribe (precedencia `flags > profile`).
- [ ] Sin `--profile`, el comportamiento actual con `.env` es idéntico — test de regresión pasa sin cambios.
- [ ] Secrets ausentes del `config.yaml` generado — test que verifica que ningún campo contiene `tskey-` o `hskey-`.
- [ ] ADR-012 redactado; `adr-002-single-binary-no-config.md` marcado como `status: deprecated` en su frontmatter.

## References

- Bitácora board: ts-bridge#185
- Related ADR: `docs/adr/adr-002.md` (superseded by this feature)
- Related spec: `specs/archive/CFG-002/` (host-emitted profiles, downstream of this)
