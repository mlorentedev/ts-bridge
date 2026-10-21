---
id: "adr-016"
type: adr
status: accepted
owner: manu
date: "2026-10-15"
issue: "mlorentedev/ts-bridge#441"
tags: [architecture, profiles, cli, ssh, lifecycle]
created: "2026-10-15"
---

# ADR-016: Self-Contained Foreground Profile Connections

## Status

Accepted by the operator on 2026-10-15. Implementation is tracked by #441.

## Context

Managed credentials (ADR-015) and OpenSSH control-plane bootstrap (ADR-005)
already exist. However, a named profile stores only target, control URL and
credential reference. A daily Windows/Git Bash connection still requires a
launcher selecting a separate YAML file for bootstrap, hostname, state and
health settings. This is configuration fragmentation, not a missing auth-key
provider.

### Constraints

| ID | Constraint | Origin |
| --- | --- | --- |
| C1 | Daily operation is one named-profile command, without a maintained launcher | Operator request |
| C2 | Profile settings are non-secret; reuse the managed credential; no Gitea token | ADR-012/015, operator |
| C3 | OpenSSH owns keys, agents, host verification and interactive unlock | ADR-005, operator |
| C4 | No DNS, hosts-file, system proxy or unrelated-profile changes | ADR-014, operator |
| C5 | Repeated startup cannot create a duplicate or take over another process | Operator |
| C6 | Destination-service evidence is distinct from startup/health readiness | Operator |
| C7 | Keep explicit configuration precedence and legacy profiles supported | ADR-012/015, #417 |

### Reference audit

| Reference | Evidence | Relevant distinction |
| --- | --- | --- |
| Local patterns | Corporate-network constraints; decision-persistence pattern | Routing restrictions do not imply SSH-agent ownership |
| Existing TCP profiles | `internal/profile/store.go`, `cmd/cli/connect_profile_test.go` | Profile storage is narrower than YAML runtime configuration |
| Existing SSH/YAML flow | `internal/bootstrapssh/bootstrap.go`, `internal/config/bootstrap_test.go` | Bootstrap is explicit and interactive; it already precedes tsnet startup |
| Browser/SOCKS flow | ADR-014, `cmd/cli/browser.go` | Browser must not inherit a fixed TCP target |
| OpenSSH | Upstream `ssh_config.5` and `ssh-add.c` | IdentityFile authentication does not universally require an agent |

The intersection supports persisted local runtime settings, explicit ownership,
and layered verification. Mandatory agent management and background reuse do
not follow from those references.

## Options Considered

| Option | Door | C1 | C2 | C3 | C4 | C5 | C6 | C7 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| A: complete profiles, foreground, reject duplicate | Type 2 | yes | yes | yes | yes | yes | explicit probe | yes |
| B: background supervisor with reuse | Type 1 public lifecycle | yes | yes | yes | yes | requires new ownership/recovery | explicit probe | additional contract |
| C: remove launcher but retain separate YAML selection | Type 2 | no | yes | yes | yes | unchanged | explicit probe | yes |
| D: automatically manage SSH agents and keys | Type 1 credential ownership | incomplete | added risk | no | platform-dependent | incomplete | incomplete | additional contract |

## Decision

Choose **A**. Extend local profiles with non-secret bootstrap SSH/SOCKS,
hostname, state directory, local bind and health settings. Provide a supported
one-time import from the existing non-secret YAML configuration, preserving
the target and managed credential unless explicitly updated. A daily command
then needs only `connect --profile <name>`.

Preserve `flags > env > YAML > profile > defaults`. Profile-provided runtime
settings are defaults, not hidden overrides. Profiles do not become shareable
machine-specific descriptors: `tsb://` remains unchanged.

Runtime state directories are allowed as non-secret machine-local settings.
This does not permit credential file paths or secret values in profiles.
Updating a target or importing a descriptor preserves locally configured
runtime settings and credential associations.

Keep the process in the foreground. Acquire OS-backed per-profile ownership
before runtime/child creation, release it on process exit, and reject duplicates
with an actionable error. Do not use port reachability or an unverified PID as
proof of ownership. Different profile names get separate ownership resources;
actual listener conflicts fail explicitly and never cause takeover.

System OpenSSH continues to resolve identities, agents and terminal prompts.
Report the selected client when diagnosing failure. An absent agent alone is
not a fatal condition when an identity file can authenticate interactively.
Do not capture, store or automate passphrases.

`READY` and `/health/ready` retain their startup meaning. Acceptance additionally
requires a real destination-service probe preserving TLS identity and requiring
no application token.

## Rationale

The naive launcher removal only hides the same split configuration. Completing
the existing profile model solves that cause without adding a supervisor or a
credential manager. Foreground duplicate rejection is reversible and was
explicitly selected instead of background reuse.

## Rejected Alternatives and Reopen Triggers

- B: reconsider only when a concrete consumer requires a successful second
  invocation to reuse an unattended session; specify ownership and recovery first.
- C: reconsider only if the one-profile-only daily invocation requirement is removed.
- D: reconsider only under a separate approved SSH-credential lifecycle requirement.
- Hash-selected TCP ownership ports and persistent PID-only locks are rejected:
  collisions and stale ownership cannot satisfy C5 reliably.

## Consequences

### Positive

- No hand-maintained daily launcher or repeated runtime flags.
- Reuses existing managed credentials, bootstrap transport, CLI and tests.
- Explicit profile ownership, interactive SSH and real-service evidence.

### Negative

- The local profile schema grows and needs update/import preservation tests.
- OS-backed locking needs platform-specific implementations and regression tests.
- Abrupt parent exit can still leave an SSH child (#376); a new attempt must
  fail on its occupied port, not kill or reuse it.

### Neutral

- #399's environment migration, #417's broader source diagnostics, #376's
  child containment, and #183's extended mesh validation remain distinct work.
- Constraints and audit live here, not in a new vault operate-layer document
  (#412). The implementation plan is the spec; task status stays on bitacora.

## References

- #441, #399, #417, #376, #183.
- [ADR-005](adr-005-headscale-compat.md), [ADR-012](adr-012-config-profiles-model.md),
  [ADR-014](adr-014-socks5-dynamic-mesh-proxy.md),
  [ADR-015](adr-015-profile-scoped-credential-management.md).
- https://github.com/openssh/openssh-portable/blob/master/ssh_config.5
- https://github.com/openssh/openssh-portable/blob/master/ssh-add.c
- `specs/CFG-005-self-contained-connections/`.
