---
id: "adr-015"
type: adr
status: accepted
date: "2026-09-29"
issue: "mlorentedev/ts-bridge#355"
tags: [architecture, credentials, profiles, security, tailscale, headscale]
owner: manu
---

# ADR-015: Profile-Scoped Credential Management

## Context

`ts-bridge` currently accepts a node auth key from `--auth-key-file`,
`--auth-key`, or `TS_AUTHKEY`. The secure path still requires the operator to
create a file manually, set its permissions, remember its location, and pass it
on every invocation. That is not a professional first-run or rotation flow.

The immediate case is an already-created Tailscale SaaS auth key for the
`office` profile. The broader design must also serve Headscale and multiple
profiles without placing secret values in `profiles.yaml` or shareable `tsb://`
descriptors (ADR-011 and ADR-012).

The control planes expose different administrative APIs:

- Tailscale creates machine auth keys with an API access token or an
  OAuth-derived access token carrying the `auth_keys` scope.
- Headscale creates pre-auth keys with local administrative access or a
  Headscale API key and a user identity.

In both systems, the administrative credential and the generated node auth key
are different security classes. A node auth key cannot mint its replacement.
The shared design is therefore the local credential lifecycle, not a universal
control-plane API.

## Constraints

1. No secret value may enter `profiles.yaml` or a `tsb://` descriptor.
2. New credential-onboarding flows use masked input or stdin and never place a
   secret in argv, stdout/stderr, logs, or agent transcripts.
3. Storage is per-user, requires no administrator privileges, and works on
   Windows, Linux, and macOS.
4. Multiple named credentials can coexist and profiles select them by name.
5. Credential precedence remains explicit:
   `--auth-key-file` > deprecated `--auth-key` > `TS_AUTHKEY` > profile
   credential. The inline flag remains temporarily for backward compatibility,
   keeps its process-list warning, and is removed only after `auth set` ships
   and documented callers have migrated.
6. Onboarding works offline with an auth key that already exists.
7. Replacement is explicit and atomic; no silent overwrite is allowed.
8. A control-plane administrative credential is transient and is not persisted
   by default.
9. Tailscale and Headscale integrations live behind provider-specific adapters.
10. Status, list, and remove operations never reveal the secret value.

## Options Considered

### A. Managed per-user credential files with profile references

`ts-bridge auth set --profile <name>` reads an existing key through masked
input, creates the per-user credential store, enforces owner-only permissions,
writes atomically, and stores only a credential reference in the local profile.

**Pros:** portable, no new dependency, no manual file handling, composes with
ADR-012, and works offline with both control planes.

**Cons:** file permissions are defense in depth, not protection from another
process running as the same user.

**Door:** Type 2. The storage backend can later be replaced behind the same
credential-store interface.

### B. Native OS credential stores

Use Windows Credential Manager, macOS Keychain, and Secret Service through a
cross-platform keyring dependency.

**Pros:** platform-native UX and encrypted storage where the platform provides
it.

**Cons:** new dependency and three platform behaviors; Secret Service is not
universally available on headless Linux; migrations and support burden exceed
the measured requirement.

**Door:** Type 1 for the initial public contract because migration and platform
fallback semantics become user-visible.

### C. Persist an administrative API credential and mint a key automatically

Store a Tailscale OAuth secret or Headscale API key and generate node keys as
needed.

**Pros:** fully unattended rotation.

**Cons:** stores a credential with a much larger blast radius than the node key
it replaces. Provider scopes, ownership, tags, user IDs, and endpoints differ.

**Door:** Type 1 and rejected as the default.

### D. Replace auth keys with persistent interactive tsnet login

Persist tsnet identity after a browser login instead of using ephemeral nodes.

**Pros:** no recurring auth-key rotation.

**Cons:** changes the existing ephemeral cleanup and state model, leaves durable
tailnet nodes, and does not cover unattended or Headscale onboarding uniformly.

**Door:** Type 1 and out of scope for credential onboarding.

### E. Keep environment variables and manually managed files

**Pros:** no implementation.

**Cons:** repeats the current failure: manual ACLs, path switching, environment
inheritance, and no profile-level credential association.

**Door:** rejected; #355 exists because this UX is inadequate.

## Decision

Adopt **Option A**, with provider-backed creation and rotation as a separate,
optional layer.

### Phase 1: managed onboarding and profile association

Add a provider-neutral credential store and these CLI operations:

```text
ts-bridge auth set [<credential>] --profile <profile>
ts-bridge auth status [<credential> | --profile <profile>]
ts-bridge auth list
ts-bridge auth remove [<credential> | --profile <profile>]
```

`auth set`:

1. Loads the named profile and uses its control URL to validate the key prefix.
2. Reads the key through masked terminal input (or stdin for explicit
   automation); there is no `--key` flag.
3. Creates the credential directory automatically.
4. Writes through a same-directory temporary file, enforces owner-only
   permissions, then atomically renames it.
5. Refuses an existing credential unless the user explicitly confirms
   replacement or supplies `--force`.
6. Updates the local profile with a non-secret credential reference. The
   optional positional argument is the credential name; when omitted, the
   profile name is used. An explicit name allows several profiles to share one
   managed credential without duplicating secret files.
7. Prints only the profile, provider/control-plane classification, and storage
   status.

The initial file-backed location is derived from the same per-user base
directory as `ProfileStorePath()`:

- Windows: `%LOCALAPPDATA%\ts-bridge\credentials\`
- Linux: `$XDG_STATE_HOME/ts-bridge/credentials/`, falling back to
  `~/.local/state/ts-bridge/credentials/`
- macOS: `~/Library/Application Support/ts-bridge/credentials/`

In implementation terms, `CredentialStoreDir()` is
`filepath.Join(filepath.Dir(ProfileStorePath()), "credentials")`. The directory
is therefore a sibling of `profiles.yaml` and `state/` on every platform.
Credential names are validated identifiers, never raw path components supplied
unchecked.

`profiles.yaml` gains only:

```yaml
profiles:
  office:
    target: acemagic-office:45000
    credential: office
```

The secret file and its path are never exported through `tsb://`.

### Resolution contract

- `connect --profile <name>` and `browser --profile <name>` use one shared
  resolver and consume the profile's managed credential when no higher
  precedence source was supplied.
- `init --profile <name>` may bind a credential reference (or direct the user
  to `auth set`) but does not read the secret merely to write non-secret profile
  configuration. This intentionally narrows #355's earlier wording that `init`
  should "consume" the credential.
- Existing profiles without `credential` remain valid and continue through the
  legacy explicit/env credential sources.
- ADR-015 supersedes #368's proposed storage of a machine-local auth-key **file
  path** in the profile. The profile stores a managed credential name instead;
  the credential store owns the path.
- SaaS and Headscale profiles retain separate credential references and state
  identities. Credential resolution must not collapse their state directories
  or control-plane selection.

### Phase 2: optional provider-backed creation and rotation

Add a narrow provider interface that returns a node credential to the same
store:

```text
ts-bridge auth create --profile <profile> --provider tailscale
ts-bridge auth rotate --profile <profile>
```

Tailscale and Headscale adapters own their API-specific request shapes. The
administrative credential is read transiently through masked input, stdin, or
an explicit protected file and is discarded after the API call. It is not
stored by the credential store. Rotation writes and validates the new node key
before replacing the old file; revocation of the old provider key is a
separate explicit step when supported.

- The Tailscale adapter requires an API/OAuth access token with `auth_keys`
  authority and validates OAuth tag ownership before requesting reusable,
  ephemeral, preauthorized, expiry, and tag capabilities.
- The Headscale adapter requires the profile control URL, a Headscale API key,
  and an explicit user identity; reusable, ephemeral, expiration, and ACL tags
  are sent explicitly rather than inheriting Headscale's one-hour/single-use
  defaults.

Phase 2 is not required to solve the current onboarding case: an operator who
already created a key uses `auth set`.

## Rejected Alternatives and Reopen Triggers

| Alternative | Rejected because | Reopen trigger |
|---|---|---|
| Native keychain as the only backend | Adds dependency and platform divergence before a measured requirement | Compliance requirement, measured file-store incident, or a portable stdlib-quality backend |
| Persist administrative API credentials | Excessive blast radius | A non-interactive rotation requirement with a scoped external secret manager |
| Persistent tsnet identity | Changes ephemeral-node semantics | Separate ADR explicitly replacing the ephemeral state model |
| Manual key files / `.env` | Current UX and rotation failure | Never as the preferred path; retained only for backward compatibility |

## Consequences

### Positive

- The current user can paste the already-created key once into a masked prompt;
  `ts-bridge` creates and protects the storage automatically.
- `connect --profile office` no longer needs a manually managed
  `--auth-key-file`.
- Multiple SaaS and Headscale credentials coexist without secret leakage into
  shareable configuration.
- Future key creation/rotation reuses the same store and profile contract.

### Negative

- Owner-only files do not protect against malicious processes running under the
  same user identity.
- Adding a `credential` profile field requires migration-safe schema handling
  and precedence tests.
- Provider-backed rotation remains additional work after Phase 1.

### Neutral

- Existing `--auth-key-file`, `--auth-key`, and `TS_AUTHKEY` flows remain
  backward compatible and retain higher precedence.
- This ADR narrows the credential architecture but does not select a permanent
  administrative-secret manager.

## Implementation Tracking

- #355: Phase 1 credential store and `auth set/status/list/remove`.
- #368: profile-scoped credential reference and resolution.
- #383: Phase 2 Tailscale/Headscale `auth create/rotate` adapters.
- #183: resume the ACEMAGIC E2E after Phase 1 can onboard the existing SaaS key.

## References

- [ADR-001](adr-001-tsnet-userspace.md) — no-admin userspace invariant.
- [ADR-005](adr-005-headscale-compat.md) — SaaS/Headscale parity.
- [ADR-008](adr-008-cli-architecture.md) — secrets excluded from YAML.
- [ADR-011](adr-011-shareable-connection-profile.md) — descriptors remain secret-free.
- [ADR-012](adr-012-config-profiles-model.md) — named local profiles and store path.
- Tailscale API v2 `POST /tailnet/{tailnet}/keys`.
- Headscale pre-auth key and remote API documentation.
- Vault patterns `pattern-secrets-security` and `pattern-secrets-rotation`.
