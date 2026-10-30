---
tags: [spec, tasks]
created: "2026-10-30"
---

# Tasks - HOST-005-host-config

> TDD order. One task = one focused commit. Tick as you go. Reorder freely while spec is in `draft` state; freeze once you start `implementing`.
>
> **Inline markers:** `[P]` = no dependency on another unchecked task. `[AC<n>]` = serves acceptance criterion `n` of `proposal.md`.

## Setup

- [x] Worktree and branch from master: `dotf wt add host-005-host-config --issue 458` -> `feat/host-005-host-config`
- [x] `proposal.md` is complete and acceptance criteria are testable
- [x] No open questions left in `proposal.md` "Risks / open questions" (owner decisions recorded 2026-10-30)

## Implementation — PR A (declared state read by `setup` and `check`)

### The file (`internal/host/file.go`, `internal/host/file_test.go`)

- [x] [P] [AC1] Failing table test `TestLoadFile_Schema`: a full valid file decodes to `File{Version:1, Host:{Services:[{rdp 3389}], Firewall rule, PreventSleep, ControlURL}}`; absent `version` is 1; `version: 2` errors naming version 1; unknown keys at top, `host`, a service, `firewall`, `power` are each named in the warning writer and the decode still succeeds. Run `go test ./internal/host -run TestLoadFile_Schema -count=1`. Expected: FAIL, `LoadFile` undefined.
- [x] [AC1] Implement `File`, `DeclaredService`, `LoadFile(path, goos string, warn io.Writer) (File, error)`: `os.ReadFile`, `yaml.Unmarshal` into a `yaml.Node`, walk it against the known-key sets, then decode. Pointers distinguish absent from empty (`Services` nil vs empty, `*string` control_url, `*bool` prevent_sleep). Same command. Expected: PASS.
- [x] [AC1] Failing table test `TestLoadFile_InvalidEntries`: kind `ftp`, port 0 and 70000, duplicate `rdp`, `services: []`, `firewall.rule: ""` and an unsafe rule, `control_url: ""`, `control_url: ftp://x`, a missing path, a directory path. Each error names the entry (`host.services[1].kind`, ...). Expected: FAIL.
- [x] [AC1] [AC6] Implement `validate`; reuse `sanitizeFirewallRule`; `control_url` must parse with scheme `http`/`https` and a host. Expected: PASS.
- [x] [AC4] Failing table test `TestLoadFile_OSCapability`: `rdp` on windows/linux ok; `ssh` on linux errors naming `linux` and `#454`; `vnc` on windows errors naming `#455`; any kind on darwin errors naming `#459`; `power` on linux and darwin errors; `power` on windows ok. Expected: FAIL.
- [x] [AC4] Implement the capability table (`supportedKinds`, `powerSupported` keyed by GOOS, with the ticket that widens each). Expected: PASS. `go test ./internal/host -count=1` all green.

### Resolution (`internal/host/config.go`, `internal/host/config_test.go`)

- [ ] [P] [AC5] Failing table test `TestResolve_PerOSDefaults`: with no file, `Resolve("windows"|"linux"|"darwin", File{}, Flags{})` declares `[{rdp 3389}]`, `[{rdp 3389}]`, `[{ssh 22}]`; `Port` stays 3389 on every OS (legacy field). Expected: FAIL, `Resolve` undefined.
- [ ] [AC5] Implement `Config.Services`, `Config.ControlURL`, `defaultsFor(goos)`, `Resolve(goos, file, flags)`; `Merge(flags)` becomes `Resolve(runtime.GOOS, File{}, flags)` so existing tests keep passing. Expected: PASS, plus `go test ./internal/host -run TestMerge -count=1` PASS.
- [ ] [AC2] Failing table test `TestResolve_Precedence`: for the rdp port, firewall rule and sleep, each layer beats the one below (`flags > env > file > default`); `TS_HOST_RDP_PORT` and `--port` rewrite the declared `rdp` service's port; `--no-sleep`/`TS_HOST_NO_SLEEP` override `power.prevent_sleep` (inverse); `control_url` comes from the file only. Expected: FAIL.
- [ ] [AC2] Implement `applyFile` between defaults and env, and the final `rdp` service/`Port` sync. Expected: PASS.

### The commands (`cmd/cli/host.go`, `cmd/cli/host_test.go`)

- [ ] [AC2] Failing test `TestHostSetupCmd_FirewallRuleFlagUnset`: with `TS_HOST_FIREWALL_RULE=EnvRule` and no flag, the fake `hostSetup` receives `EnvRule`. Expected: FAIL (`Tailscale-RDP-Ingress`, the flag's non-empty default).
- [ ] [AC2] Give `host setup --firewall-rule` an empty default and keep the default in help text. Expected: PASS.
- [ ] [AC1] [AC2] Failing tests `TestHostSetupCmd_Config` / `TestHostCheckCmd_Config`: `--config file.yaml` declaring port 3390 and rule `FileRule` reaches the fake as `Port 3390`, `FirewallRule FileRule`; an invalid file returns exit 1 and the fake is never called; a missing path errors naming it; unknown keys print a warning on stderr; `.env` in the working dir still beats the file. Expected: FAIL (`--config` resolves to the root's reserved flag and is ignored).
- [ ] [AC1] [AC2] Add a local `--config` to `setup` and `check`, `loadHostFile(cmd)`, and resolve with `host.Resolve(hostGOOS, file, flags)`. Expected: PASS.
- [ ] [AC4] Failing cmd test: `host setup --config` with `ssh` declared, as linux, exits 1 naming `linux` before the fake runs. Expected: PASS once the previous task lands; if it already passes, keep it as the wiring guard.
- [ ] [P] [AC3] Failing test `TestHostPackage_DoesNotImportUserStores` (`internal/host/imports_test.go`): parse the package's non-test files with `go/parser` and assert no import of `ts-bridge/internal/profile` or `ts-bridge/internal/config`. Expected: PASS today (guard), then FAIL when deliberately adding the import locally, then revert.
- [ ] [AC3] Test `TestHostCmds_NeverReadUserStore`: elevated (`HostSetupCmdForTest(..., elevated=true)`), `HOME`/`XDG_STATE_HOME`/`XDG_CONFIG_HOME`/`LOCALAPPDATA`/`APPDATA`/`USERPROFILE` pointed at a temp "root home" holding a valid `profiles.yaml` whose default profile sets `control_url: https://poison.invalid` and a `credentials/` file; `setup` and `check` with a `--config` declaring no `control_url` reach the fake with `ControlURL == ""` and print no `poison`. Expected: PASS (regression guard).

### The descriptor (`internal/host/step.go`, `cmd/cli/host.go`)

- [ ] [AC6] Failing tests: `checkFor` and `setupFor` copy `cfg.ControlURL` into `CheckResult.ControlURL` / `SetupResult.ControlURL`; `checkJSON`, `printCheckText` and `printSetupSummary` emit a `tsb://` descriptor whose control plane is that URL (`cp=` per `internal/profile`). Expected: FAIL.
- [ ] [AC6] Add `ControlURL` to both results and pass it to `buildDescriptorLine`. Expected: PASS.

### PR A close

- [ ] Help text of `setup`/`check` documents `--config` and the precedence; `go test -race ./...`, `GOOS=windows go vet ./...`, `GOOS=darwin go vet ./...`, `golangci-lint run`, `gosec ./...` green.
- [ ] `bats scripts/tests/smoke.bats` (Git Bash) unchanged and green.

## Implementation — PR B (`host init` writes the file)

- [ ] [AC7] Failing tests `TestHostInit_RequiresConfig`: without `--config`, interactive and non-interactive both fail before prompting, naming `--config`. Expected: FAIL (writes `./.env`).
- [ ] [AC7] Failing table test `TestHostInit_WritesYAML`: per goos (`windows`, `linux`), non-interactive flags and interactive answers produce a YAML file that `host.LoadFile(path, goos)` loads with no warning and `Resolve` maps back to the same port, rule and sleep; no `.env` is created; `power` appears on windows only; file mode `0644` on Unix. `darwin` prints "not applicable" and writes nothing. Expected: FAIL.
- [ ] [AC7] Rewrite `host_init.go` to build a `host.File` from the answers and marshal it; remove `writeHostEnv`, `stripHostSection`, `hasNonBlank` and their test export. Expected: PASS.
- [ ] [AC8] Failing table test `TestHostInit_Overwrite`: existing file + non-interactive without `--force` refuses ("already exists (use --force to overwrite)"); interactive "n" aborts; interactive "y" and `--force` overwrite; a directory at the path and (Unix) a `0000` file are refused even with `--force`, and the existing content is untouched. Expected: FAIL.
- [ ] [AC8] Reuse `ensureWritable` after a readability probe (`os.Open` + close; a non-`ErrNotExist` stat/open error refuses). Expected: PASS.
- [ ] [AC3] Extend `TestHostCmds_NeverReadUserStore` to `host init --config`. Expected: PASS.
- [ ] Docs: `site/src/content/docs/cli-reference.md` (host init/setup/check `--config`), `site/src/content/docs/configuration.md` (host file schema + precedence), `README.md` host section, G3/G5 rows in `docs/design/host-client-parity.md`.
- [ ] `go test -race ./...`, cross-GOOS vet, `golangci-lint run`, `gosec ./...`, `smoke.bats` green.

## Closing

- [ ] Every acceptance criterion from `proposal.md` is covered by at least one test
- [ ] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command
- [ ] Lint passes
- [ ] No unrelated changes in the diff (no scope creep)
- [ ] `verification.md` filled in
- [ ] PRs opened referencing this spec folder (`Closes #458` on PR B only)
