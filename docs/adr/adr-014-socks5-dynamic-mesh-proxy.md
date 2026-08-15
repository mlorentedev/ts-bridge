---
id: "adr-014"
type: adr
status: proposed
date: "2026-08-14"
tags: [architecture, networking, socks5, multi-target, kubelab, ssh, kubectl]
owner: manu
---

# ADR-014: SOCKS5 Dynamic Mesh Proxy for Multi-Target Connectivity

## Context

`ts-bridge` forwards a single TCP target per instance (`connect --target host:port`). While this is ideal for point-to-point bridging (e.g. dedicated RDP desktop access), managing a multi-node cluster (such as a Headscale kubelab mesh) requires connecting to dozens of distinct endpoints:

1. **SSH (`:22`)** to every compute and control plane node.
2. **Kubernetes API (`:6443`)** to K3s API servers (staging, hub, prod).
3. **Internal HTTP/gRPC endpoints** across ephemeral cluster services.

Running N separate `ts-bridge` processes or configuring static port ranges per node introduces operational friction, configuration sprawl, and breaks standard tooling ergonomics (`~/.ssh/config`, `kubeconfig`).

### Key Constraints

- **Zero-Admin Client:** Must run entirely in userspace via `tsnet` without requiring root or Administrator privileges to create OS network devices (TUN/TAP).
- **Tooling Compatibility:** Standard development tools (`ssh`, `kubectl`, `curl`, `git`) must work with existing configuration patterns.
- **Headscale / Tailscale SaaS Portability:** Must function across both self-hosted Headscale control planes and Tailscale SaaS.

---

## Decision

**Implement a SOCKS5 dynamic proxy listener mode within `ts-bridge`.**

1. **Architecture & Routing:**
   - `ts-bridge` binds a local SOCKS5 listener (default `127.0.0.1:1080`).
   - When a client issues a `CONNECT` command for `host:port` (either Tailscale IP `100.64.0.x:22` or MagicDNS hostname `ace1:6443`), `ts-bridge` resolves and dials the destination dynamically using the `tsnet.Server.Dial(ctx, "tcp", target)` interface.
   - Data is forwarded bidirectionally between the local SOCKS5 client connection and the remote tsnet connection.

2. **Tooling Recipes:**
   - **OpenSSH:** Configured via `ProxyCommand` in `~/.ssh/config`:
     ```ssh-config
     Host *.kubelab 100.64.0.*
         ProxyCommand nc -X 5 -x 127.0.0.1:1080 %h %p
         User root
     ```
   - **Kubectl:** Configured either globally or per-cluster via `proxy-url`:
     ```yaml
     apiVersion: v1
     kind: Config
     clusters:
     - cluster:
         server: https://100.64.0.10:6443
         proxy-url: socks5://127.0.0.1:1080
       name: kubelab-staging
     ```
     Or via environment variable: `export HTTPS_PROXY=socks5://127.0.0.1:1080`.

3. **Control-Plane ACL Contract:**
   - The ephemeral `ts-bridge` node connects with an auth key carrying a dedicated tag (e.g. `tag:operator-bridge`).
   - The Headscale/Tailscale ACL policy explicitly permits `tag:operator-bridge` to access `tag:cluster-node` on `:22` and `:6443`.

---

## Options Evaluated and Rejected

| Option | Description | Rejection Rationale |
|---|---|---|
| **1. Multi-instance with `--port-range`** | Launch N bridges with static port allocations per node (e.g. `33022` $\rightarrow$ `node1:22`, `33023` $\rightarrow$ `node2:22`). | Inflexible: requires manual mapping per node, breaks when nodes are added or renamed, requires maintaining port mapping tables in documentation. |
| **2. Virtual TUN/TAP device & OS routing** | Create a virtual network interface on the client OS and route the entire `100.64.0.0/10` subnet through it. | **Violates core design constraint:** requires local Administrator/root permissions to create virtual network interfaces in the host OS. |
| **3. SOCKS5 Dynamic Proxy (Chosen)** | Listen on a single local SOCKS5 port and dynamically dial arbitrary mesh targets via `tsnet.Server.Dial`. | Standard RFC 1928 protocol, supported by SSH, kubectl, and curl; zero OS privileges needed. |

---

## Consequences

### Positive
- A single running `ts-bridge` process provides access to the entire tailnet mesh.
- No local administrative privileges required.
- Seamless compatibility with `~/.ssh/config` and `kubectl` cluster definitions.
- Automatic support for new nodes joining the mesh without restarting the bridge.

### Negative / Trade-offs
- Client applications must support SOCKS5 (or proxy commands). Applications lacking proxy support cannot route traffic through the bridge.
- Control plane ACLs must be maintained to grant the bridge node necessary destination permissions.
