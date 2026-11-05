---
tags: [spec, verification, templates]
created: "2026-10-30"
---

# Verification - HOST-005-host-config

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] AC1 (schema, loader and `--config`) -> #519 `4363338`, #554 `ecace2e` / tests `TestLoadFile_Schema`, `TestLoadFile_InvalidEntries`, `TestHostSetupCmd_Config`, `TestHostCheckCmd_Config`
- [x] AC2 (precedence, `--firewall-rule` unset) -> #554 / tests `TestResolve_Precedence`, `TestHostSetupCmd_FirewallRuleFlagUnset`, `TestHostSetupCmd_Config` (`.env beats the file`)
- [x] AC3 (no per-user store read, C9) -> #554, PR B / tests `TestHostPackage_DoesNotImportUserStores`, `TestHostCmds_NeverReadUserStore` (setup, check, serve, init; direct and through the root command; malformed store)
- [x] AC4 (per-OS capability errors) -> #519, #554 / tests `TestLoadFile_OSCapability`, `TestHostSetupCmd_ConfigOSCapability`, `TestHostSetupCmd_ConfigInvalidOnDarwin`
- [x] AC5 (per-OS defaults) -> #554 / test `TestResolve_PerOSDefaults`
- [x] AC6 (`control_url` in the descriptor) -> #554 / tests `TestCheckFor_ControlURL`, `TestSetupFor_ControlURL`, `TestHostCmds_DescriptorControlURL`, `TestPrintSetupSummary_ControlURL`; `setup --json` -> #587 / test `TestSetupJSON_DescriptorControlURL`
- [x] AC7 (`host init` writes the YAML) -> PR B / tests `TestHostInit_RequiresConfig`, `TestHostInit_WritesYAML`
- [x] AC8 (`host init` overwrite and unreadable-path refusal) -> PR B / test `TestHostInit_Overwrite`. Its unreadable-file case cannot run on Windows (no read bit) or as root, so the authoritative run is the CI `test` job (Linux, non-root); it also passed unprivileged in WSL on 2026-11-03
## Test status

- Test suite: `go test ./... -count=1` -> all 14 packages ok (Windows, 2026-11-03); `go vet ./...` clean for windows, linux, darwin; `features.json` f1-f8 pass; `TestHostInit_*` pass on Linux in WSL.
- Manual smoke test: none on a live host; the host file is exercised end to end by `host init` -> `LoadFile` -> `Resolve` in `TestHostInit_WritesYAML`.
- No regressions in existing test suite: yes

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- PR A split into A1 (loader, #519) and A2 (resolution, `--config` on setup/check, #554) for the ~300 LOC cap.
- `host init` loads the file it wrote before reporting success; that read-back found `key: null` from `yaml.Marshal` and led to `omitempty` on the optional keys.
- `--firewall-rule`/`--no-sleep` on a non-Windows host are refused rather than written, since the loader would refuse the file they produce.
- A declared RDP port no step applies (Windows check, Linux xrdp) is deferred to #557.
- Review round 1 (nan/deepseek-v4-flash, FAIL on `70251c9`): the documented paths failed on a fresh host because the parent did not exist (now created `0755`), and the breaking `host init` change lacked a release marker. The plan was to retitle PR #579 `feat(host)!` with a `BREAKING CHANGE:` footer, but #579 merged as plain `feat(host):` (`3e1a3e2`); round 2 caught it. Minors applied in the same push: parent help and setup summary wording, `--no-sleep` help, macOS before `--config`, the resolved values on `host check --verbose`, no path in the file header.
- Review round 2 (nan/deepseek-v4-flash, FAIL on `42c777b`), two Majors: `setup --json` carried no descriptor (fixed in #587) and the release marker was missing (#602 carries the `BREAKING CHANGE:` footer and a README upgrade note, so the next release is 2.0.0). Minors: refusing `--firewall-rule`/`--no-sleep` off Windows departs from the proposal's "flags keep today's behaviour"; it is kept, because the loader would refuse the file those flags produce. macOS returning before the `--config` check is the ordering round 1 asked for, so AC7's "fails without it" holds on Windows and Linux only. `runHostSetup` at 101 lines is ticketed as #588.
- `host init` now writes a temporary file in the same directory, loads it back, and renames it over the path, so a file that fails the read-back never replaces a good one (review Q2).
- Schema drift, recorded rather than re-contracted: `proposal.md` lists five host keys, and `LoadFile` also accepts `host.mesh`, which HOST-008 added and documents (`specs/HOST-008-host-serve-mode/`, `configuration.md`).
- Accepted risk kept from the proposal: elevated `host setup` still reads `./.env` from the working directory; `host check --verbose` now logs the resolved values, so the winning layer is observable (review Q1).

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-049-2026-10-30.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: the host file is ADR-017 §1, already accepted; nothing new to decide
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: read-back-after-write is project-local so far (one occurrence)

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/HOST-005-host-config/` -> `specs/archive/HOST-005-host-config/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [x] Promotions above executed (if any)
