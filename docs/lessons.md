---
id: "ts-bridge-lessons"
type: lesson
status: active
tags: [learning, pattern]
created: "2026-02-23"
owner: manu
---

# ts-bridge: Lessons Learned

> Append new lessons in reverse-chronological order. Use a `## YYYY-MM-DD` heading per session, then one bullet per fact — lead with a **bold rule** when the fact generalizes.

## 2026-06-24

- **A pinned linter's embedded type-checker is a moving compatibility surface — bump it when you bump the deps it must analyze**: golangci-lint **v1.62.2** (pinned in `ci.yml`) ships a Go type-checker too old to analyze **tailscale v1.100**, so Dependabot #199 (tailscale v1.80→v1.100) failed `lint` with *phantom* `typecheck` errors on **dependency** symbols (`certStateStore.ReadState`, `client.BuildTailnetURL`, `c.Do`) that aren't in our code — while `build`, `test`, and `test-windows` stayed green against the same bump. The tell: only `lint` is red and the "undefined" symbols belong to a dependency, not the repo. Fix is not to touch our code but to migrate the lint toolchain (golangci-lint v1→v2, action v4→v9), which #199 and #201 were both blocked behind. Rule: when a dep bump fails *only* lint on dependency symbols, suspect type-checker skew, not a code defect.
- **golangci-lint v1→v2 is a schema migration, not just a version bump — and the linter *merge* can silently widen scope**: v2 requires `version: "2"`, moves `linters-settings`→`linters.settings`, `issues.exclude-dirs`→`linters.exclusions.paths`, `issues.exclude-rules`→`linters.exclusions.rules`, and replaces `issues.exclude-use-default: true` with `linters.exclusions.presets` (`comments`, `common-false-positives`, `legacy`, `std-error-handling`). The trap: v2 **merged** `gosimple` (S1\*) and `stylecheck` (ST1\*) into `staticcheck`, so a v1 config that enabled only `staticcheck` (SA\*) will, after a naive migration, start running style/simplify checks it never ran before. Preserve the original scope with `linters.settings.staticcheck.checks: [all, -S1*, -ST1*, -QF1*]`. Rule: after a linter consolidation, pin the check set to what was actually enabled before, or the "migration" quietly becomes a new-rules PR.

## 2026-06-13

- **`Closes #N` binds only to the *first* number in a comma list — repeat the keyword per issue**: PR #122 fixed BUG-011 through BUG-019 and its squash-commit footer read `Closes #102, #106, #107, #108, #109, #111, #112`. On merge, **only #102 auto-closed**; the other six stayed open. GitHub's closing keyword attaches to the single issue number immediately after it — `Closes #102, #106` parses as "close #102" plus a bare mention of #106. The 2026-06-12 lesson's prescribed form is correct but was misapplied here. Rule: put **`Closes #N` on its own line, one per issue** (repeating the keyword — `Closes #102, Closes #106` — also works), and after merge confirm every issue actually closed. This is the documented lesson failing in practice one day later — adherence, not knowledge, was the gap.
- **A PR's `Closes` footer is not evidence the bug was fixed — verify against the diff**: PR #122's footer listed `#111` (BUG-016, YAML unknown-field warning → structured logger), but `internal/config/yaml.go` was never modified — the warning still goes to `os.Stderr` (`yaml.go:43`). Had the comma-list gotcha above *not* swallowed it, #111 would have auto-closed as a false positive. BUG-017 (#113, banner `%-14s`) was likewise in the PR title's "through BUG-019" range but `main.go` was untouched. When reconciling a batch PR, check each issue's named file/symbol against the actual changeset before closing. (This session: verified-and-closed #106/#107/#108/#109/#112; kept #111/#113 open with a note.)
- **Sanitize shell-command arguments against an allowlist before interpolation, per shell** (BUG-012): `ts-bridge host` built a Windows firewall rule name by interpolating user input directly into a PowerShell/`netsh` command, enabling command injection via `--firewall-rule`. Fix: `sanitizeFirewallRule` whitelists permitted characters before the value reaches the shell (mirrors the existing `sanitizeHostnameLabel` approach). Rule: any value that crosses into a shell (`powershell -Name`, `netsh`, `cmd /c`) must be validated against an allowlist — not ad-hoc escaped, since escaping rules differ per shell and are easy to get subtly wrong.
- **Validate numeric/duration flags at the flag, env, *and* merge layer — a value enters config through three doors** (BUG-006): `--dial-retries -1` passed flag parsing, env parsing, and `Merge()` without rejection, yielding nonsensical retry behavior. CLI flag, env var, and YAML→`Merge()` are three independent entry points; validating only one leaves the others open. Fix: reject negative dial-retries at all three. While there, `Merge()` exceeded the 40-line guideline and was split into focused helpers, and four near-identical tests collapsed into one table-driven test with 6 subcases.

## 2026-06-12

- **PR body must include "Closes #N" for auto-close on merge**: When creating a PR that resolves issues, include `Closes #N` (one per line) in the PR body. GitHub auto-closes those issues when the PR merges. Without this, issues stay open even after the fix is merged and must be closed manually. Multiple issues: `Closes #1, Closes #2, Closes #3`. This is standard GitHub convention and should be in every agent's PR creation checklist.

## 2026-03-13

- **Corporate TLS inspection breaks Headscale TCP passthrough**: `tailscale up --login-server=https://vpn.kubelab.live` hangs with `wsarecv: connection forcibly closed`. Corporate network has transparent TLS inspection (no proxy configured, invisible). Firewall MITMs all TLS, decrypts, validates content is HTTP. Headscale serves Noise protocol (binary, not HTTP) → firewall kills connection. Same VPS IP with standard HTTPS (mlorente.dev via Traefik HTTP routing) works fine. SSH works (not TLS). Alternate ports (8443) also fail. **Diagnostic fingerprint**: same IP, HTTPS site works, VPN fails, no proxy in `netsh winhttp show proxy`, SSH works. **Rule**: Before migrating a device to Headscale, verify the network allows direct TLS without inspection. Corporate DPI blocks Noise protocol. Use Tailscale SaaS from corporate networks (CDN relay IPs are trusted by firewalls).
- **Windows SSH key permissions: icacls `(RW)` is invalid — use `(M)`**: After setting a key to read-only with `icacls /grant:r "(R)"`, trying to restore write access with `"(RW)"` fails with "Invalid parameter". The correct permission mask is `(M)` (Modify), which includes read+write. `(R)` is sufficient for SSH to accept the key; `(M)` is needed when the file also needs to be written (e.g., `ssh-keygen -p`).
- **Windows ssh-agent requires admin to start**: On corporate machines without admin rights, `Start-Service ssh-agent` fails if the service is Disabled. Workaround: remove the passphrase from the private key with `ssh-keygen -p` (set new passphrase to empty). This eliminates the need for ssh-agent entirely. Requires `(M)` permissions on the key file first.
- **`ssh-keygen -l` vs `-y` may show different comments on Windows**: `-l` reads the comment from the unencrypted header of the OpenSSH private key file; `-y` derives the public key from the private key material and may show a different comment. Trust `-y` output for the actual public key. When fingerprints differ between local key and dotfiles pubkey, verify with `-y` before assuming a key mismatch.
- **age-encrypted SSH key + SSH passphrase = two layers**: The dotfiles sensitive key (`id_ed25519.secret.age`) is age-encrypted AND the inner OpenSSH key is passphrase-protected. After `age -d`, the resulting file still requires a passphrase for SSH operations. This is intentional (defense in depth) but must be accounted for when setting up on a new machine without ssh-agent.
- **Hetzner VNC console corrupts long pastes**: Pasting a full SSH public key (`ssh-ed25519 AAAA...`) via the Hetzner web console garbles characters — the resulting key in `authorized_keys` has a different fingerprint and SSH rejects it silently. Diagnosis: `ssh-keygen -l -f ~/.ssh/authorized_keys` and compare with `SHA256:` from `ssh -v`. Fix: use `curl https://github.com/<user>.keys >> ~/.ssh/authorized_keys` instead — avoids paste entirely.
- **Upload public keys to GitHub as standard practice**: `github.com/<user>.keys` exposes all public keys registered on the account. Any server can add them with a single `curl` — no paste, no corruption, works from anywhere. Upload once per machine. Name the key descriptively (e.g. `dell-work`, `msi-personal`) so you know which machine to revoke when needed.
- **SSH key comment can be changed without regenerating**: `ssh-keygen -c -C "new-comment" -f ~/.ssh/id_ed25519` updates the comment in-place. Useful when a key was generated on one machine (e.g. `manu@msi`) and is now used on another (`dell-work`). Does not change the key material or fingerprint.
- **`$env:VAR` syntax only works in PowerShell, not cmd.exe**: `ssh-keygen -y -f "$env:USERPROFILE\.ssh\id_ed25519"` fails in `cmd.exe` with "No such file or directory". Use `%USERPROFILE%` in cmd. The PowerShell prompt shows `~>` while cmd shows `C:\Users\...>`.
- **dotfiles key sync mismatch**: `ssh/id_ed25519.pub` and `sensitive/id_ed25519.secret.age` can get out of sync if the machine that last ran `dotfiles-sync` had a different key. Always verify with `ssh-keygen -l -f sensitive/...` after decrypting on a new machine. The safest recovery is `curl github.com/<user>.keys` to the server rather than relying on key fingerprint matching.

## 2026-03-07

- **Data race in bidirectional copy when only one direction blocks**: `proxyConnections` launched a goroutine for `rx` and blocked on `tx`. When `tx` finished, `return tx, rx` read `rx` non-atomically while the `rx` goroutine might still be writing via `atomic.AddInt64`. The race detector only caught this when new tests exercised the function directly with `net.Pipe()`. Fix: add `sync.WaitGroup` so both directions complete before returning byte counts. Root cause: the original `go copyConn(...)` pattern assumed `closeAll()` would synchronize both sides, but `sync.Once` only guarantees one call — it doesn't block the caller until the other goroutine exits.
- **`net.Pipe` close produces `"io: read/write on closed pipe"`**: When testing proxy logic with `net.Pipe()` instead of real TCP sockets, the pipe-specific error string is not classified by `isExpectedCloseError`. This causes spurious error metrics in tests. Fix: add `"closed pipe"` to the expected close error list — it's the pipe analog of `ECONNRESET`.
- **Starlight requires `content.config.ts` for mixed .md/.mdx sites**: When scaffolding an Astro/Starlight docs site that uses both `.md` and `.mdx` files, the `content.config.ts` file is mandatory even if it only re-exports the Starlight defaults. Without it, Astro silently fails to load content collections. This applies to any Starlight project, not just ts-bridge.

## 2026-02-27

- **Never work directly on `master` when release-please is active**: While pushing Headscale compatibility work directly on `master`, `git push` was rejected because release-please had merged two PRs (v1.3.0, v1.3.1) in the meantime. The result: merge conflicts across 4 files (.env.example, main.go, main_test.go, README.md) requiring manual resolution. Fix: adopt GitHub Flow — all work on feature branches, `master` is protected with required status checks and PR-only merges. This prevents the developer and release-please bot from racing on the same branch.
- **Headscale minimum_version kills silently**: Headscale v0.28.0 requires client `minimum_version=v1.74`. tsnet v1.60.0 fails during Noise handshake with `reading response header: EOF` — no clear version rejection message. Spent 2 days debugging Traefik routing (which was also broken) before discovering the version mismatch as the final blocker. The clue was in Headscale logs: `Clients with a lower minimum version will be rejected minimum_version=v1.74`.
- **tsnet API is remarkably stable**: Upgraded from v1.60.0 to v1.80.0 (20 minor versions, ~1 year of development) with zero code changes. The `tsnet.Server` struct fields, `Up()`, `Dial()`, and `Close()` methods are identical. Only `go.mod` and `go.sum` changed.
- **Traefik TCP passthrough requires 5 syntax differences from HTTP**: `tcp:` not `http:`, `HostSNI()` not `Host()`, `tls.passthrough: true`, `address:` not `url:`, no `healthCheck:` or `middlewares:`. Easy to get wrong when copying HTTP router config.
- **Multi-layer debugging**: When a single symptom (EOF) has multiple root causes stacked (Traefik Upgrade stripping + version incompatibility), fixing one layer reveals the next. Always re-test from scratch after each fix.

## 2026-02-26

- **Minimal .env reduces user friction**: reduced `.env.example` from 10 vars to 2 required + 2 commented optional. Users only need `TS_AUTHKEY` and `TS_TARGET` to start. Advanced vars live in the README reference table — power users find them, beginners aren't overwhelmed.
- **RDP host configuration is the #1 support surface**: Windows Home can't host RDP (hard Microsoft limit), NLA with passwordless Microsoft accounts silently fails, and Tailscale's WFP firewall rules can conflict with third-party antivirus. Documenting these upfront in README and runbook prevents most user confusion.
- **TS_CONTROL_URL previewed in .env.example**: even though HC-001 code isn't implemented yet, showing the commented variable in `.env.example` primes users for the upcoming Headscale feature and establishes the naming convention early.

## 2026-02-24

- **Windows permission semantics differ from Unix**: strict `0700` assertions are not portable on Windows filesystems, so tests must gate permission checks by OS.
- **gosec G115 can trigger on safe-looking modulo math**: avoid `int -> uint32` casts in index calculations; use same-width arithmetic to satisfy static analysis and keep intent explicit.
- **RDP-driven disconnects can be noisy but normal**: `wsarecv ... forcibly closed by the remote host` commonly appears when the remote side ends the session; classify as expected close to reduce false-warning logs.
- **Low-friction defaults increase adoption**: default auto mode + minimal `.env` + bootstrap scripts significantly reduce setup friction across client devices.

### [2026-03-07] Managing Cyclomatic Complexity in Go projects
**Context:** Refactoring main.go into multiple internal packages and implementing Graceful Shutdown.
**Problem:** Large single files (main.go > 600 lines) led to high cyclomatic complexity (19), causing golangci-lint failures (threshold 15) and making logic difficult to test or maintain. Initial implementation of Graceful Drain increased complexity further.
**Solution:** 1. Split main.go into internal packages: internal/config, internal/proxy, internal/health, internal/telemetry. 
2. Decoupled metrics into internal/telemetry to avoid circular dependencies between health and proxy.
3. Extracted complex setup/teardown logic into helper functions (initTailscale, handleShutdown, drainActiveConnections) reducing cyclomatic complexity from 19 down to 7.
4. Used interface-based mocking (Dialer) to enable unit testing of core proxy logic without real network dependencies.
**Tags:** `#go` `#refactoring` `#clean-architecture` `#ci-cd` `#linter`

### [2026-03-13] Corporate TLS Inspection Breaks Headscale TCP Passthrough
**Context:** Migrating a corporate Windows workstation from Tailscale SaaS to self-hosted Headscale (vpn.kubelab.live). The VPS uses Traefik TCP passthrough with SNI routing so Headscale handles TLS termination and the Noise protocol directly.
**Problem:** tailscale up --login-server=https://vpn.kubelab.live hung indefinitely. Health check showed: fetch control key: wsarecv: connection forcibly closed. Diagnosis took multiple steps: (1) Headscale container running and healthy, (2) TLS certs valid (expire May 2026), (3) Traefik TCP passthrough config correct, (4) curl from VPS through full Traefik path worked (TLS 1.3 OK), (5) curl from Windows failed with schannel: failed to receive handshake, (6) mlorente.dev (same VPS IP, Traefik HTTP routing) worked, (7) direct IP also failed, (8) alternate port 8443 also failed, (9) no proxy configured (netsh/env/registry all empty), (10) SSH on port 22 worked fine. Root cause: corporate network has transparent TLS inspection (inline firewall, not a configured proxy). The firewall MITMs all TLS connections, decrypts traffic, and validates it is HTTP. For mlorente.dev, Traefik terminates TLS and serves standard HTTP — firewall allows it. For vpn.kubelab.live, Traefik does TCP passthrough to Headscale, which serves the Tailscale Noise protocol (binary, not HTTP) after TLS — firewall detects non-HTTP content and kills the connection. SSH works because it uses its own encryption protocol that the firewall cannot MITM.
**Solution:** From corporate networks with transparent TLS inspection, Headscale TCP passthrough is fundamentally incompatible. Options: (1) Use Tailscale SaaS from corporate networks (their relays use Cloudflare/Akamai IPs that firewalls whitelist), (2) SSH tunnel workaround (ssh -N -L 9443:localhost:8443 deployer@VPS_IP + hosts file entry + TS_CONTROL_URL with tunnel port), (3) Test Headscale migration from non-corporate networks (home workstation msi is already on the mesh at 100.64.0.1), (4) Request IT exception for the VPS IP on port 443. Rule: Before planning Headscale migration for a device, verify the network allows direct TLS to the control plane. Corporate networks with DPI/TLS inspection will block the Noise protocol even though standard HTTPS to the same IP works. The giveaway is: same IP, mlorente.dev works, vpn.kubelab.live doesn't, no proxy configured, SSH works.
**Tags:** `#headscale` `#tailscale` `#corporate-firewall` `#tls-inspection` `#networking` `#traefik` `#tcp-passthrough`


## 2026-03-16

### Headscale Migration Abandoned — Tailscale SaaS is the Correct Solution for Corporate Networks

**Context:** Multi-day effort (Feb 25 – Mar 16) to consolidate all VPN nodes under a single Headscale control plane (ADR-013). The code worked (ts-bridge v1.4.0 with `TS_CONTROL_URL`), Traefik TCP passthrough was configured, tsnet was upgraded to v1.80.0, and registration was verified (100.64.0.11, ephemeral cleanup working).

**Problem:** Corporate networks have transparent TLS inspection (inline firewall, not a configured proxy). The firewall MITMs all TLS, decrypts, validates content is HTTP. Headscale serves Noise protocol (binary, not HTTP) after TLS → firewall kills connection. Same VPS IP with standard HTTPS works fine. SSH works (not TLS). Alternate ports also fail. No proxy configured. Diagnostic fingerprint: same IP, HTTPS site works, VPN fails, SSH works.

**Decision:** Abandon Headscale migration for corporate-network devices. Use Tailscale SaaS for all Windows hosts. The 3 PCs (acemagic-office, acemagic-lab-1, acemagic-lab-2) are configured with native Tailscale on the host side + ts-bridge client from both Linux and Windows workstations. All access is via RDP.

**Why this is the right call:**
1. Tailscale SaaS relay IPs are CDN/Cloudflare addresses — corporate firewalls whitelist them.
2. The `TS_CONTROL_URL` feature stays in ts-bridge for non-corporate use cases (homelab, personal network).
3. Headscale consolidation (ADR-013) remains valid for homelab nodes where there's no TLS inspection.
4. Zero operational complexity — Tailscale SaaS handles key rotation, relay infrastructure, and NAT traversal.

**Lesson:** Before planning any self-hosted VPN migration for corporate devices, verify the network allows direct TLS to the control plane. Transparent TLS inspection is invisible (no proxy settings, no certificates to install) but kills any non-HTTP protocol tunneled over TLS. The only reliable diagnostic is: "same IP, HTTPS works, binary protocol doesn't, SSH works."

**Artifacts archived:**
- `90_archive/ts-bridge/guide-headscale-migration.md`
- `90_archive/ts-bridge/guide-windows-host-to-headscale.md`
- `90_archive/ts-bridge/guide-windows-host-headscale-fresh.md`

**Tags:** `#headscale` `#tailscale` `#corporate-firewall` `#tls-inspection` `#decision`

## 2026-05-18

### Ephemeral mode mandates auth-key rotation on every client (operational reality)

**Context:** v1.5.0 client on a corporate Windows machine failed with `tsnet.Up: backend: invalid key: API key does not exist` two months after the `.env` was last edited. Initial misdirection: assumed Headscale misconfiguration (`TS_CONTROL_URL` missing). Vault correction: the 3 acemagic-* PCs are explicitly on Tailscale SaaS (see lesson [2026-03-16]).

**Root cause:** `main.go` hardcodes `Ephemeral: true` on `tsnet.Server`, and auto-mode wipes the state dir on shutdown. Result: every bridge startup is a *fresh node registration* that re-consumes the auth key. There is no "already registered, no need to update" path. When the key expires, revokes, or hits its single-use cap, **every client breaks simultaneously** and needs the new key in its local `.env`. The host machines on native Tailscale are unaffected — they use persistent state.

**Diagnostic flow that worked:**
1. Error literal `API key does not exist` → control plane has no record of this key. Three possibilities, all server-side: (a) expired, (b) revoked, (c) single-use already consumed.
2. Check `.env` mtime vs Tailscale auth-key TTL. If `.env` mtime + key TTL < today → expired.
3. Confirm at `https://login.tailscale.com/admin/settings/keys` — if status is *Expired*/*Revoked* or the key is absent, that is the bug.
4. Generate replacement: **Reusable** + **Ephemeral**, max TTL (90d for Tailscale SaaS). Push new value to every client `.env`.

**Rule:** With `Ephemeral: true` (the current ts-bridge default and the design intent), auth-key rotation is not optional — it's a recurring operational task tied to the key TTL. The TTL is a hard ceiling: max 90d on Tailscale SaaS. A scheduled reminder at TTL−7d is the lightweight mitigation; the heavier path is a small automation that monitors the Tailscale API and emits a new key.

**Tags:** `#tailscale` `#ephemeral` `#auth-key` `#operational`

### tsnet.Server.Up() partial-start: must Close() on error or Windows cleanup leaks

**Context:** Alongside the auth failure, v1.5.0 emitted `failed to cleanup ephemeral state directory ... tailscaled.log1.txt: The process cannot access the file because it is being used by another process` (5 retries, all fail). Root cause was in `initTailscale` (PR #18).

**Problem:** `tsnet.Server.Up(ctx)` is **not** all-or-nothing. By the time it returns an error, it has already spawned background goroutines (logger, control client, NetMon, magicsock) and opened `tailscaled.log*.txt` for writing. The previous code returned the error without calling `server.Close()`. The deferred `cleanupEphemeralStateDir` then tried to `os.RemoveAll` the temp dir, but Windows file locks are *mandatory* (unlike POSIX advisory locks), so `unlinkat` on the log file fails for every retry until the process exits. On Linux the same code path succeeds because `unlink()` on an open file works.

**Fix:** Call `server.Close()` on the error path *before* the cleanup defer fires. This releases the file handles synchronously.

```go
status, err := server.Up(ctx)
if err != nil {
    _ = server.Close() // release tsnet workers holding tailscaled.log*
    return nil, fmt.Errorf("tailscale init failed (control=%s): %w", controlURLForError(cfg.ControlURL), err)
}
```

**Rule (generalized):** Any Go constructor that *might* have spawned background goroutines or opened OS handles before returning an error must be paired with an explicit `Close()` on the error path, even if a later API audit would suggest "we never started anything." The contract for `tsnet.Server.Up()` is implicit: if you call it, you must call `Close()` *regardless of return value*.

**Secondary UX win added in the same PR:** the error wrap now includes the effective control URL — `tailscale init failed (control=https://controlplane.tailscale.com (default)): ...` — so the operator immediately sees whether the key was sent to SaaS or a custom plane. Diagnosis time on the next occurrence drops from ~30min to ~5s.

**Diagnostic pattern matching:** Added `diagnoseTailscaleInitError` helper that maps known tsnet error substrings to actionable `WARN` log lines with a `remediation` field. Fail-silent on unknown patterns to avoid log noise. Table-driven tests in `main_test.go`.

**Tags:** `#go` `#tsnet` `#windows` `#file-locking` `#error-handling`

### tsnet.Server self-heals across DERP/magicsock transients via awaitRunning

**Context:** ARCH-004 spec required answering "does tsnet self-heal the control-plane tunnel after a drop?" before designing the reconnect layer. The vault spec doc allotted ≤30 min for this investigation.

**Finding:** Reading `tailscale.com/tsnet@v1.80.0/tsnet.go:168-206`, every `Dial()` call invokes `awaitRunning(ctx)` which blocks on `ipn.Notify` state transitions until the backend returns to `ipn.Running`. DERP relay drops and magicsock reconnects are handled at lower layers transparently — by the time `Dial()` returns, the tunnel is healthy. Only terminal states (`Stopped`, `NeedsMachineAuth`) surface as the error `"tsnet: backend in state %v"` from `awaitRunning`. Net effect: a retry layer at the `Dialer` interface covers virtually all transient failures, and a dedicated control-plane supervisor (Tier 2 in the ARCH-004 design) is unnecessary.

**Rule:** Before designing a supervisor for any tsnet-based component, verify what `awaitRunning` already gives you. Tsnet's lifecycle reads "blocks until Running or terminal" — that's already a supervisor in disguise. Saved ~50% of the ARCH-004 implementation surface.

**Tags:** `#tsnet` `#tailscale` `#self-healing` `#design-investigation`

### gosec standalone vs golangci-lint: `//nolint:gosec` is not portable

**Context:** ARCH-004 PR #24 CI failed on `gosec` with G404 (weak RNG). The inline `//nolint:gosec` directive that suppresses golangci-lint findings did NOT suppress the standalone gosec runner.

**Why:** This project's CI runs both `golangci-lint-action@v4` (which includes a gosec module) AND `securego/gosec@v2.23.0` as a separate step. The two runners read different exclusion syntaxes:
- `golangci-lint`-style: `//nolint:gosec` (line above or end-of-line)
- standalone `gosec`-style: `// #nosec G404 -- reason` (end-of-line)

The standalone gosec runner only respects `#nosec`. golangci-lint's gosec respects both, so `#nosec` is the portable choice when both runners exist.

**Rule:** For any project that runs gosec both via golangci-lint AND as a separate CI step, use `// #nosec` everywhere. Document the reason inline. `.golangci.yml` exclusions only apply to the golangci-lint pass and do not bind the standalone runner.

**Tags:** `#go` `#gosec` `#linter` `#ci` `#cross-tool-compatibility`

### First-iteration SDD overhead is fixed; second iteration is trivially cheap

**Context:** First adoption of `pattern-spec-driven-development.md` in ts-bridge (REL-003 + ARCH-004) within a single session. The first spec required ~30 min of scaffolding (proposal + tasks + verification + ts-bridge/CLAUDE.md opt-in pointer); the second spec took <5 min by copying REL-003's structure.

**Finding:** SDD's cost is amortized across specs within a project. The expensive setup is the first time:
- Reading the pattern + templates
- Adapting templates to the project's existing standards (TDD, conventional commits)
- Adding the project-level opt-in pointer
- Scaffolding the first \`specs/<id>/\` and getting comfortable with the proposal/tasks/verification rhythm

After that, opening a new spec is a copy-and-tweak of the previous one. The "thinking tool" payoff (catching ARCH-005 → ARCH-004 collapse during Q3 analysis, before any code was written) was already realized in the first spec.

**Rule:** Don't evaluate SDD by the cost of the first spec — it's amortized. Evaluate by whether the proposal-first discipline caught at least one bad assumption before code was written. In this session: yes, twice (REL-003 chose A1-per-direction over A2-shared-state with rationale; ARCH-004 confirmed ARCH-005 collapse before opening a redundant PR).

**Tags:** `#sdd` `#process` `#workflow` `#meta`

### Pinning the linter version surfaces accumulated deferred debt

**Context:** PR #33 pinned `golangci-lint` from `version: latest` to `v1.62.2`. Two things happened that wouldn't have happened with the floating version:
1. `gosimple S1008` fired on a pre-existing `if X { return true }; return false` block in `reconnect.go` that had survived weeks under `latest`.
2. `gocyclo` raised the bar from whatever `latest` resolved to and rejected `LoadConfig` at complexity 16 (after a new env-var validation pushed it over 15).

**Finding:** "Floating linter version" is a quiet form of technical-debt absorption — your local sees one thing, CI sees another, and rules drift in/out as upstream releases. Pinning forces every contributor (and every CI run) to see the same finding set. The PR that introduces the pin will fail CI on the existing accumulated drift — that's the *correct* failure, not a regression.

**Rule:** When introducing a linter pin, expect 1-3 immediate findings on existing code. Plan to fix them in the same PR (or a small `chore:` precursor) — they're real issues the looser version was hiding. The cost is paid once.

**Tags:** `#linter` `#golangci-lint` `#technical-debt` `#ci`

### Half-close + idleConn embedding: type assertions need unwrap

**Context:** Adding `CloseWrite()` support to `proxyConnections` (ARCH-004-follow-on) hit a subtle Go gotcha. `idleConn struct{ net.Conn; ... }` embeds the `net.Conn` *interface*, not a concrete type. Embedded interfaces do NOT promote concrete methods from whatever implementation is held — so even if the underlying `*net.TCPConn` has `CloseWrite()`, the wrapper does not satisfy `interface{ CloseWrite() error }`.

**Finding:** The type assertion `dst.(halfCloser)` against the wrapped conn returns `(_, false)`. Two cleanups possible:
1. Add `CloseWrite()` method to `idleConn` that delegates — but then the wrapper "claims" the capability even for inner conns that lack it, and the test path silently no-ops.
2. Unwrap before asserting: `if ic, ok := c.(*idleConn); ok { c = ic.Conn }; if cw, ok := c.(halfCloser); ok { ... }`. Correct, honest. Costs 2 lines.

Chose unwrap. Production code reflects truth; test path falls back to full `Close()` (which is what `net.Pipe` requires anyway).

**Rule:** When wrapping `net.Conn` via interface embedding, any optional interface method (`CloseWrite`, `SetDeadline` family that the inner may or may not support) needs an explicit unwrap-before-assert at the call site. Embedding promotes interface methods (those on `net.Conn` itself) but not extra ones on concrete implementations. The Go FAQ and stdlib `net/http` source both follow this pattern.

**Tags:** `#go` `#interfaces` `#embedding` `#net-conn`

### TOCTOU on atomic counters: CAS loop or step-and-rollback

**Context:** `AcceptLoop` previously did `GetActiveConnections() >= max` check, then `AddActiveConnection(1)` inside `handleConn` — two separate atomic ops. Under accept-storm load, multiple goroutines could observe `cur < max` simultaneously and all increment past the cap.

**Finding:** Two correct patterns to fix this in Go's `sync/atomic`:
1. **CAS loop** (chosen): `for { cur := Load; if cur >= max { return false }; if CompareAndSwap(cur, cur+1) { return true } }`. Spins until either succeeds or limit hits. Bounded retries since the value can only grow.
2. **Step-and-rollback**: `n := Add(1); if n > max { Add(-1); return false }; return true`. Always increments first; rolls back on overflow. Simpler but the brief over-increment can be visible to other readers (`GetActiveConnections` returns `max+1` for a moment).

CAS loop preserves the invariant `ActiveConnections <= max` always. For a metric people inspect via HTTP (`/metrics`), this matters — a step-and-rollback can show `cap+1` to scrapers mid-rollback.

**Rule:** When the counter is observable (exposed metric, used for back-pressure decisions, etc.), prefer CAS loop. When it's purely internal (just want eventual correctness), step-and-rollback is fine and slightly cheaper. Either way, the original `check; then; act` pattern is wrong under any concurrency.

**Tags:** `#go` `#atomics` `#concurrency` `#toctou` `#race-conditions`

### Multi-dimensional audits: 4 agents converge faster than 1 agent on 4 passes

**Context:** Ran 4 parallel subagents (Scalability, UX, DX, SOLID) on the v1.7.0 codebase in one shot. Each prompt was self-contained with a specific reading list + output format constraint.

**Finding:** All four agents independently returned **PASS-WITH-GAPS** verdicts. The convergence is itself a signal: the codebase has consistent quality across dimensions (not lopsided e.g. "great scalability but terrible UX"). Cross-cutting findings emerged that no single audit would have caught:
- The same "stale CLAUDE.md" problem manifested as DX-1 (stale main.go line count) and UX-1 (README hardcoded port that contradicts auto-mode default) — the audits surfaced it as two facets of one underlying "docs lag code" issue.
- Scalability TOCTOU on MaxConnections + Half-close not honored both come from the same root: the original implementation focused on happy-path; under-load edge cases were deferred.

Single-agent multi-pass audits in series would have produced a similar findings list but missed the convergence signal. Cost: 4 parallel agents = ~3-4× one agent's tokens, but ~1× wall time and a free triangulation check.

**Rule:** For project maturity assessments, prefer parallel multi-dim agents over single-agent comprehensive sweeps. Watch for convergence: when verdicts and root causes overlap across dimensions, the underlying issues are systemic and worth fixing first.

**Tags:** `#audit` `#agents` `#process` `#code-quality`

### [2026-05-18] vault_health is project-scoped — cross-project wikilinks need markdown link form
**Context:** Running /vault-doctor on ts-bridge after 43 days of dormancy. Initial pass showed 27 unresolved wikilinks; after fixing the within-project relative-path cases (e.g. a relative-path `90-lessons` wikilink rewritten to a bare-name `90-lessons` wikilink), 7 cross-project links remained flagged — including bare wikilinks like an `adr-013-vpn-consolidation` wikilink (target lives in kubelab) and even fully-qualified vault-root wikilinks like a `kubelab/00-context` vault-root wikilink.
**Problem:** vault_health (Hive MCP) reports cross-project wikilinks as broken regardless of syntax variant. Obsidian's UI resolves the vault-root-path wikilink form correctly, but the project-scoped checker can't follow it. Authors hit this every time they cite an ADR or context note from a sibling project — the checker complaint outlives the actual breakage.
**Solution:** Adopt a two-form convention: within-project links stay as bare-name wikilinks (e.g. a `90-lessons` or `security-audit` wikilink — Obsidian's flat-namespace resolves them across subdirectories); cross-project links become **relative markdown links** with explicit `.md` extension (e.g. a `[kubelab](relative/path/00-context.md)` or `[ADR-013](relative/path/adr-013-vpn-consolidation.md)` link). Markdown links are filesystem paths — vault_health treats them like any relative path lookup, so it can follow them out of the project namespace.

> Note: this lesson describes a knowledge-store (Obsidian vault) tooling quirk, retained verbatim from the project's history. It does not apply to this repo's `docs/` tree (no wikilinks here); the wikilink/path forms above are rendered as prose to keep the docs free of live store references.
**Tags:** `#vault` `#obsidian` `#tooling` `#wikilinks` `#hive`
