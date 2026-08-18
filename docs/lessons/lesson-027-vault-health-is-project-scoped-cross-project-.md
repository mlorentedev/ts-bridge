---
id: lesson-027-vault-health-is-project-scoped-cross-project-
type: lesson
status: active
created: "2026-05-18"
owner: manu
tags: [ts-bridge, lesson, vault, obsidian, tooling, wikilinks, hive]
---

# vault_health is project-scoped — cross-project wikilinks need markdown link form

**Context:** Running /vault-doctor on ts-bridge after 43 days of dormancy. Initial pass showed 27 unresolved wikilinks; after fixing the within-project relative-path cases (e.g. a relative-path `90-lessons` wikilink rewritten to a bare-name `90-lessons` wikilink), 7 cross-project links remained flagged — including bare wikilinks like an `adr-013-vpn-consolidation` wikilink (target lives in kubelab) and even fully-qualified vault-root wikilinks like a `kubelab/00-context` vault-root wikilink.
**Problem:** vault_health (Hive MCP) reports cross-project wikilinks as broken regardless of syntax variant. Obsidian's UI resolves the vault-root-path wikilink form correctly, but the project-scoped checker can't follow it. Authors hit this every time they cite an ADR or context note from a sibling project — the checker complaint outlives the actual breakage.
**Solution:** Adopt a two-form convention: within-project links stay as bare-name wikilinks (e.g. a `90-lessons` or `security-audit` wikilink — Obsidian's flat-namespace resolves them across subdirectories); cross-project links become **relative markdown links** with explicit `.md` extension (e.g. a `[kubelab](relative/path/00-context.md)` or `[ADR-013](relative/path/adr-013-vpn-consolidation.md)` link). Markdown links are filesystem paths — vault_health treats them like any relative path lookup, so it can follow them out of the project namespace.

> Note: this lesson describes a knowledge-store (Obsidian vault) tooling quirk, retained verbatim from the project's history. It does not apply to this repo's `docs/` tree (no wikilinks here); the wikilink/path forms above are rendered as prose to keep the docs free of live store references.
**Tags:** `#vault` `#obsidian` `#tooling` `#wikilinks` `#hive`
