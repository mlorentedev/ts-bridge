# Documentation

Project-bound knowledge for `ts-bridge`, kept in-repo (docs-as-code). The *operate/build* layer lives here so it is versioned with the code and readable by any agent in-context — no external knowledge store required.

- [`adr/`](adr/) — Architecture Decision Records (the *why* behind decisions)
- [`runbooks/`](runbooks/) — operational procedures (deploy, RDP host setup, multi-device operations, audits)
- [`troubleshooting/`](troubleshooting/) — known errors, security audit, release issues
- [`lessons.md`](lessons.md) — accumulated gotchas and post-mortems
- [`qa-coverage.md`](qa-coverage.md) — automated smoke-test coverage matrix (what CI checks vs. what needs a human)

The *decide/position* layer (roadmap, prestudy, strategy) and session memory live in the maintainer's cross-project knowledge store and are intentionally not committed here.
