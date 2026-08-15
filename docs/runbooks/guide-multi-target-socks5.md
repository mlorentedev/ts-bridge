---
id: "ts-bridge-multi-target-socks5"
type: runbook
status: active
tags: [runbook, socks5, ssh, kubectl, kubelab, headscale]
created: "2026-08-14"
owner: manu
---

# Multi-Target Mesh Connectivity via SOCKS5

> Operational guide for reaching an entire Tailscale/Headscale mesh from a non-admin workstation using `ts-bridge` in SOCKS5 proxy mode.

---

## 1. Running the SOCKS5 Bridge

Start `ts-bridge` pointing to your Headscale or Tailscale mesh in SOCKS5 proxy mode:

```bash
# Linux / macOS
./ts-bridge connect \
  --auth-key-file /run/secrets/authkey \
  --control-url https://vpn.kubelab.live \
  --socks5 127.0.0.1:1080

# Windows (PowerShell)
.\ts-bridge.exe connect `
  --auth-key-file C:\Users\user\.secrets\authkey `
  --control-url https://vpn.kubelab.live `
  --socks5 127.0.0.1:1080
```

Once running, the bridge listens on `127.0.0.1:1080` and dynamically forwards TCP connections to any destination on the mesh.

---

## 2. Tooling Configuration Recipes

### OpenSSH (`~/.ssh/config`)

Add a wildcard host block in your `~/.ssh/config`:

```ssh-config
# Match all mesh IP addresses
Host 100.64.0.* *.kubelab
    User root
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
    ProxyCommand nc -X 5 -x 127.0.0.1:1080 %h %p
```

On Windows without `nc`, use `connect-proxy` (included with Git for Windows):

```ssh-config
Host 100.64.0.* *.kubelab
    User root
    ProxyCommand connect -S 127.0.0.1:1080 %h %p
```

Usage:
```bash
ssh ace1.kubelab
ssh 100.64.0.15
```

---

### Kubernetes (`kubectl`)

#### Option A: Per-Cluster `proxy-url` in `kubeconfig`

Configure your `~/.kube/config` cluster entry with `proxy-url`:

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ...
    server: https://100.64.0.10:6443
    proxy-url: socks5://127.0.0.1:1080
  name: kubelab-staging
contexts:
- context:
    cluster: kubelab-staging
    user: admin
  name: kubelab-staging
current-context: kubelab-staging
```

#### Option B: Environment Variable

```bash
export HTTPS_PROXY=socks5://127.0.0.1:1080
kubectl get nodes
```

---

### HTTP / REST (`curl`)

```bash
curl --socks5-hostname 127.0.0.1:1080 http://100.64.0.25:8080/health
```

---

## 3. Headscale / Tailscale ACL Policy Contract

Ensure your control-plane ACL policy grants the bridge tag access to required ports on cluster nodes:

```json
{
  "acls": [
    {
      "action": "accept",
      "src": ["tag:operator-bridge"],
      "dst": [
        "tag:cluster-node:22",
        "tag:cluster-node:6443",
        "tag:cluster-node:80,443"
      ]
    }
  ]
}
```
