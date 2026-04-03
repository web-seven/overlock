# Remote Nodes

## What is it

A remote node is a separate Linux machine — a server, a VM, or a cloud instance — that joins your Overlock environment as a worker node. Overlock connects to it over SSH, installs the necessary components, and adds it to the cluster automatically.

Remote nodes behave just like any other Kubernetes worker, but they run on real hardware or a real VM instead of a Docker container on your laptop.

Traffic between nodes is encrypted automatically using WireGuard — you don't need to set that up yourself.

---

## When would I use it

You're building a Crossplane setup that needs to be tested on real infrastructure before going to production. For example:

- You want to run Crossplane's engine components (providers, functions) on a dedicated server, while your development machine handles workloads.
- You're preparing a multi-host cluster that mirrors your production topology.
- You want to offload CPU-heavy providers to a more powerful remote machine.

---

## How to use it

### Prerequisites

The remote machine needs:
- A Linux OS
- Docker installed and running
- An SSH user that can run Docker commands (typically `root`, or a user with `sudo`)
- Accessible from your machine on the SSH port (default: 22)

### Step 1 — Create the environment

Remote nodes require the `k3s-docker` engine:

```bash
overlock env create my-env --engine k3s-docker
```

### Step 2 — Add the remote node

```bash
overlock env node create my-remote-node \
  --environment my-env \
  --host 192.168.1.100
```

Overlock SSHs into the machine, sets up the k3s agent, and joins it to your cluster. The `--host` flag is the IP address or hostname of the remote machine.

### Step 3 — Specify what the node is for

Use `--scopes` to control what runs on the remote node:

```bash
# Dedicate the remote node to Crossplane engine components
overlock env node create my-remote-node \
  --environment my-env \
  --host 192.168.1.100 \
  --scopes engine
```

Available scopes:
- `engine` — Crossplane, providers, functions, cert-manager, and policy components
- `workloads` — user workloads and application pods

### Step 4 — Use a custom SSH key or user

By default Overlock uses `~/.ssh/id_rsa` and the `root` user. Override these if needed:

```bash
overlock env node create my-remote-node \
  --environment my-env \
  --host 192.168.1.100 \
  --user deploy \
  --key ~/.ssh/my-deploy-key \
  --port 2222
```

### Remove a remote node

```bash
overlock env node delete my-remote-node \
  --environment my-env \
  --host 192.168.1.100
```

Overlock removes the node from the cluster and cleans up the containers on the remote machine.

When you delete the entire environment, remote node containers are found and cleaned up automatically.

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

## Related guides

- [Local Nodes](local-nodes.md) — add nodes locally using Docker containers
- [Environments](environments.md) — create and manage environments
