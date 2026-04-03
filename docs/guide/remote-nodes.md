# Remote Nodes

A remote node is a separate Linux machine — a physical server, a VM, or a cloud instance — that joins your Overlock environment as a worker node. Overlock connects to it over SSH, installs the necessary k3s agent components, and adds it to your cluster automatically. Once joined, it behaves exactly like any other Kubernetes worker node.

Traffic between nodes is automatically encrypted using WireGuard, so you don't need to set up a VPN or manage any network security yourself.

> [!NOTE]
> Remote nodes require the `k3s-docker` engine. You cannot add remote nodes to an environment created with `kind`, `k3s`, or `k3d`. Plan your engine choice before creating the environment — you can't change it afterwards.

---

## Why Use Remote Nodes?

Remote nodes are the bridge between local development and production-ready validation.

**Production topology mirroring** — In production you likely run Crossplane engine components on dedicated infrastructure, separate from workloads. Remote nodes let you replicate that topology during development and verify that your configurations, providers, and scheduling rules work correctly in a distributed setup before they go live.

**Offloading heavy providers** — Some providers are computationally intensive. Moving them to a dedicated remote machine with more CPU and RAM means your development laptop stays responsive, and you get a more realistic picture of provider performance.

**Multi-host integration testing** — If you're building a Crossplane configuration that manages infrastructure across multiple machines or sites, remote nodes let you test that scenario in a controlled environment without standing up a full production cluster.

---

## Prerequisites

Before adding a remote node, make sure the target machine has:

- A Linux operating system
- Docker installed and running
- An SSH user with permission to run Docker commands (typically `root`, or a user with passwordless `sudo`)
- SSH access from your machine on the target's SSH port (default: 22)
- No firewall blocking traffic between your machine and the remote on the ports WireGuard uses

> [!TIP]
> The simplest setup is a VPS or cloud VM with a public IP, Docker installed, and SSH access as `root`. Overlock handles everything else automatically.

---

## Step 1 — Create the Environment

Remote nodes require the `k3s-docker` engine:

```bash
overlock env create my-env --engine k3s-docker
```

---

## Step 2 — Add the Remote Node

With the environment running, add the remote machine:

```bash
overlock env node create my-remote-node \
  --environment my-env \
  --host 192.168.1.100
```

Overlock connects to the machine via SSH, installs the k3s agent, configures WireGuard for secure inter-node communication, and registers the node in your cluster. This typically takes 30–60 seconds. When the command returns, run `kubectl get nodes` to confirm the node is ready.

---

## Step 3 — Assign a Scope to the Node

Use `--scopes` to control what Kubernetes will schedule on the remote node. This is what lets you mirror a production topology where different machines have different roles.

```bash
# Dedicate the remote node to Crossplane engine components
overlock env node create my-remote-node \
  --environment my-env \
  --host 192.168.1.100 \
  --scopes engine
```

Available scopes:

- `engine` — Crossplane, providers, functions, cert-manager, and other control plane components
- `workloads` — user-created workloads and application pods

> [!NOTE]
> Scopes work by applying labels and taints to the node. The `engine` scope adds a taint that repels non-engine workloads, so only components that explicitly tolerate the taint will schedule there. This is important to set up correctly if you want true isolation.

---

## Step 4 — Customize SSH Connection Settings

By default, Overlock connects as `root` on port 22 using `~/.ssh/id_rsa`. Override any of these if your setup is different:

```bash
overlock env node create my-remote-node \
  --environment my-env \
  --host 192.168.1.100 \
  --user deploy \
  --key ~/.ssh/my-deploy-key \
  --port 2222
```

> [!TIP]
> Using a dedicated deploy key for Overlock SSH access is a good security practice, especially in team environments. Generate one with `ssh-keygen -t ed25519 -f ~/.ssh/overlock-deploy` and add the public key to the remote machine's `authorized_keys`.

---

## Step 5 — Limit Resource Usage

If you want to prevent the remote node from using all available CPU:

```bash
overlock env node create my-remote-node \
  --environment my-env \
  --host 192.168.1.100 \
  --cpu 4
```

---

## Removing a Remote Node

When you no longer need the remote node:

```bash
overlock env node delete my-remote-node \
  --environment my-env \
  --host 192.168.1.100
```

Overlock gracefully removes the node from the cluster and cleans up the k3s containers on the remote machine.

> [!NOTE]
> When you delete the entire environment (`overlock env delete my-env`), Overlock automatically finds and cleans up any remote node containers as part of the deletion process. You don't need to manually delete remote nodes before deleting the environment.

---

## Command Reference

### `overlock env node create <name>` (remote)

| Flag | Default | Description |
|------|---------|-------------|
| `--environment` | *(required)* | Name of the environment to join |
| `--engine` | `k3s-docker` | Must be `k3s-docker` for remote nodes |
| `--host` | — | IP address or hostname of the remote machine |
| `--user` | `root` | SSH username |
| `--port` | `22` | SSH port |
| `--key` | `~/.ssh/id_rsa` | Path to the SSH private key |
| `--scopes` | — | Node role: `workloads`, `engine`, or both |
| `--cpu` | — | Maximum CPU this node can use |
| `--taints` | — | Kubernetes taints to apply to the node |

### `overlock env node delete <name>` (remote)

| Flag | Default | Description |
|------|---------|-------------|
| `--environment` | *(required)* | Name of the environment |
| `--engine` | `k3s-docker` | Engine type |
| `--host` | — | IP address or hostname of the remote machine |
| `--user` | `root` | SSH username |
| `--port` | `22` | SSH port |
| `--key` | `~/.ssh/id_rsa` | Path to the SSH private key |

---

## Related Guides

- [Local Nodes](local-nodes.md) — add extra nodes using Docker containers on your local machine
- [Environments](environments.md) — create and manage the overall environment lifecycle
- [Getting Started](getting-started.md) — end-to-end walkthrough
