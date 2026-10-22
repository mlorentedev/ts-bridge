---
spec: "BUG-031-derp-socks-proxy"
verdict: "PASS"
reviewed_sha: "c64060ca5ddbb2a54514fb30d0a3080dbe189254"
reviewer: "nan/mimo-v2.6-flash"
date: "2026-10-20"
---

## Adversarial review

**Scope**: BUG-031-derp-socks-proxy (spec review round 1, launched by `dotf spec review`; reviewer `nan/mimo-v2.6-flash` drawn from `harness/reviewer-pool.json`)

**Sources**:
- `specs/BUG-031-derp-socks-proxy/{proposal,tasks,verification}.md`, `features.json`, `review-request.json`
- Diff: `git diff 06cfb8781a65b4a320961b425d10010216a98932...HEAD` (single commit `c64060c`), full scope read
- Pinned SDK source in module cache: `tailscale.com@v1.104.1/derp/derphttp/derphttp_client.go` (`dialNodeUsingProxy`), `net/tshttpproxy/tshttpproxy.go` (`SetProxyFunc`), `feature/useproxy`; `golang.org/x/net@v0.59.0/proxy`

### Spec and task alignment

- **Root cause verified against pinned source, not taken on trust.** `derphttp.Client.dialNodeUsingProxy` (v1.104.1) writes an HTTP `CONNECT` for *any* non-`https` proxy URL — so the old hook's `socks5h://` address was spoken as HTTP to OpenSSH's SOCKS listener, producing exactly the recorded pre-fix failures (`incompatible SOCKS version`, malformed response `"\x05\xff"`). The diagnosis in `proposal.md` and `docs/lessons/lesson-038-2026-10-20.md` matches the dependency source line-for-line.
- **AC1** proven: `TestControlProxySDKConsumersHelper` runs in an isolated subprocess (the hook is process-global), drives the *actual* `derphttp.NewRegionClient` to an authenticated `ServerInfoMessage` plus a Go HTTPS client with verification on (fixture root pool, `MinVersion` TLS1.2), through a real synthetic SOCKS server that asserts the original destination *hostname* (remote DNS). Ran `go test ./internal/bootstrapssh -run TestControlProxySDKConsumers -count=1` fresh in this session → ok.
- **AC2** proven: `TestControlProxyRejectsUnrelatedDestinations` (three request shapes incl. `control.test.evil` suffix attack, asserts the SOCKS dialer is never reached), `TestControlProxyHTTP` (ordinary HTTP forwarding + remote DNS), `TestControlProxyUpstreamFailure` (explicit 502), selector/fallback cases in `bootstrap_test.go`. All fresh → ok.
- **AC3** proven: `TestControlProxyCancel`, `TestControlProxyCleanupActiveTunnel` (both cancellation entry points close hijacked tunnels), `TestControlProxyCleanupCanceledStart`, `TestConfigureControlProxyInstallsHook` (failed install leaves the installed hook unchanged). `relayControlProxy` preserves the hijacked buffer (`TestControlProxyBufferedCONNECT`, mutation-verified by the author and consistent with my reading).
- **AC4** proven: `go test -count=1 ./...` → every package ok in this session; `go build ./...` exit 0; `go vet ./internal/bootstrapssh ./cmd/cli` clean; `go list -deps ./cmd/ts-bridge` contains none of the four test-only modules nor `derp/derpserver` (confirmed independently). Live DERP acceptance is explicitly deferred to #441 and **not claimed** — the proposal, tasks and verification all say so consistently; Gitea not claimed.
- **Contract freshness**: I recomputed `dotf`'s normalised digests (`normaliseContract`: CRLF fold, checkbox fold, frontmatter-status fold, features `state`/`evidence` strip + Go-canonical JSON) — all three match the digests recorded in `review-request.json` at launch. The contract set is unchanged since launch.
- **Task honesty**: two boxes remain unchecked (`lint` pending CI's pinned v2.14.0, `PR opened` — `gh pr list` confirms no PR yet). Both are accurately disclosed, not silently skipped. No `[AGENT-DRAFT]`/`[AGENT-SUGGESTION]` tags in any spec file.
- **Docs/handoff**: ADR-005 amendment accurately describes the adapter, its cancellation ownership and the test-only deps; `docs/runbooks/guide-control-bootstrap.md` documents the new listener and its unauthenticated-loopback warning; `docs/lessons/lesson-038-2026-10-20.md` + `_index.md` entry exist.
- **Scope**: diff matches the proposal. Two disclosed side-changes: (a) the readiness-test measurement repair (documented in tasks AC4 + verification "Decisions made" + lesson-038), (b) `golang.org/x/net` promoted to direct plus four SDK-pinned test-only indirects (justified in proposal, verified absent from the production binary).
- **Mutation battery — re-run by me, not trusted from the table**: `EqualFold`→`Contains` reddens `TestControlProxyRejectsUnrelatedDestinations`; removing the `context.AfterFunc` in `controlProxyConnect` reddens `TestControlProxyCleanupActiveTunnel`; removing `context.WithTimeout` in `waitUntilReady` reddens `TestStartEnforcesReadyTimeoutDuringDial` (the retention proof for the deliberately weakened readiness test). All three mutations applied, observed red, and restored; tree clean at `c64060c` afterwards.
- **Readiness-test weakening (retention bar)**: exact test + detected failure named (deadline enforcement in `waitUntilReady`); stronger remaining proof *shown* (M3 mutation, red in this session, `count=20` green after restore); production code untouched by the repair (test-only); rationale recorded in verification.md decisions + lesson-038; focused command run. Bar met.
- **Fallback path**: no recursion risk — the fallback calls stdlib `http.ProxyFromEnvironment` (plain env lookup); `feature.HookProxyFromEnvironment` is consumed only by Tailscale packages, so the one-shot `tshttpproxy` hook cannot be re-entered from the fallback.
- **Security checklist over the diff**: no injection surface (CONNECT target goes to length-prefixed SOCKS encoding, no shell/SQL/path handling); no secrets; scope check (`EqualFold` on `r.URL.Hostname()`) is applied before any dial, including the non-CONNECT forward path; loopback-only bind; `ReadHeaderTimeout` set; relay uses bounded `io.Copy` buffers, both copy goroutines terminate on either side closing (no leak/deadlock path found); all new functions are under 40 lines with CC ≤ 10 by inspection.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | THEORETICAL | lifecycle | A second `ConfigureControlProxy` call *before any SDK proxy lookup* silently replaces the hook instead of failing: `tshttpproxy.SetProxyFunc` errors only when `config != nil`, not when `proxyFunc` is already set — so the first proxy's listener stays alive-but-unreferenced (bounded by run-context cancellation, correctly scoped, so impact is one stray loopback listener for the run). Not reachable in current wiring: `startConfiguredBootstrap` invokes it once per run, and after the SDK's first lookup the config is set, which is the failing path that *is* tested. | `tshttpproxy.go` `SetProxyFunc` source read from module cache; single-call wiring at `cmd/cli/run.go:309` | `TestConfigureControlProxyInstallsHook` covers the config-set failure only; the silent-replace path is UNTESTED | code (optional install-once guard) or follow-up ticket — disposition in `verification.md` |
| Minor | SPECULATIVE | resilience | If `server.Serve` ever exits on a permanent listener error at runtime, the one-shot `tshttpproxy` hook keeps advertising the dead proxy URL until run-context cancellation; control traffic then fails connection-refused. This is fail-loud with an `slog.Error` from `serveControlProxy`, consistent with ADR-005's fail-loud stance; no recovery path is possible without replacing the one-shot hook. | code read of `serveControlProxy` + `SetProxyFunc` one-shot semantics | Covered indirectly by `TestControlProxyCancel` (listener teardown); the runtime-death scenario itself is UNTESTED | none required — accepted fail-loud; record disposition |
| Question | — | scope | Hostname-only CONNECT scoping means any *port* on the control hostname is reachable through the tunnel, and the adapter is unauthenticated on loopback. Both are explicitly accepted in `proposal.md` ("Limit requests to the configured control hostname") and ADR-005's existing local-use warning, and the adapter is strictly narrower than the pre-existing OpenSSH SOCKS listener it sits in front of (arbitrary hosts). | proposal "What"/risks sections; ADR-005 amendment | `TestControlProxyRejectsUnrelatedDestinations` (host scope as specified) | none — spec-compliant; recorded for audit |

No Blocker or Major findings. The REAL defect class this spec exists to fix (protocol mismatch at the SDK boundary) is pinned by a named pre-fix-red/post-fix-green regression test re-verified in this session.

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | A | All four ACs verified against named passing tests; negative/abuse cases (suffix host, denied dial, upstream failure, both cancellation entries, buffered CONNECT) covered; root cause confirmed against pinned SDK source. |
| Verification       | A | Every `features.json` command, the full suite, vet, build, production-deps check and all three mutation claims re-run fresh in this session with outputs read; lint/`-race` deferred to CI and disclosed (host toolchain limits). |
| Scope              | B | Diff matches the proposal; two side-changes (readiness-test repair, four test-only indirects) are disclosed and justified but were not named in `proposal.md` itself. |
| Reliability        | B | All error paths handled and tested (502, hijack failure, canceled start, idempotent close); two theoretical runtime edges remain (silent hook replace, dead-hook-after-Serve), both non-blocking. |
| Maintainability    | A | Functions < 40 lines, CC ≤ 10, comments explain the non-obvious (buffered bytes, caller-owned deadline, child cancellation context), no dead code. |
| Handoff-readiness  | A | ADR-005 amendment, runbook, indexed lesson, decisions log and concrete next steps (#441, PR pending) all present; contract set untouched by this review. |

**Aggregation**: no D, no C → all B or above.

### Verdict

**PASS**

No blockers or majors; two THEORETICAL/SPECULATIVE minors, each with a disposition line, plus one recorded spec-compliant question. Rubric all B+.

### Recommended next steps

Route for disposition in `verification.md` (contract set stays closed — do **not** edit `proposal.md`/`tasks.md`/`features.json` under this verdict; an edit invalidates it):

- Disposition the two Minor findings: apply (install-once guard), ticket, or decline with a reason each.
- Open the PR referencing this spec folder, then tick the two remaining `tasks.md` boxes (`lint` via the CI `lint` job pinned at v2.14.0, and the PR box) — ticking checkboxes is folded by the digest normaliser and does not stale this review.
- On PR open, disposition CI `lint`, `test -race` (Linux) and `security` (gosec) results; that closes the deferred checks disclosed in `verification.md`.
- Keep live DERP acceptance (#441) and Gitea (#442) explicitly out of this spec's claims, as the spec itself does.
