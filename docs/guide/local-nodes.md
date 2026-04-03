# Local Nodes

## What is it

A node is a worker machine in your Kubernetes cluster. By default, an environment comes with the nodes it needs to run. But with the `k3s-docker` engine, you can add extra nodes — either local (running as Docker containers on your machine) or remote (running on another machine over SSH).

A **local node** is a Docker container on the same machine as your environment, added as an extra worker in the cluster.

---

## When would I use it

You want to simulate a multi-node cluster on your laptop — for example, to test that your workloads schedule correctly when separated from the Crossplane engine components. Local nodes let you create that separation without needing any extra hardware.

Another common use: isolating resource-heavy Crossplane providers onto a dedicated node with a CPU cap, so they don't slow down the rest of your machine.

---

## How to use it

First, make sure your environment was created with the `k3s-docker` engine:

```bash
overlock env create my-env --engine k3s-docker
```

Then add a local node:

```bash
overlock env node create my-extra-node --environment my-env
```

By default this node is scoped to handle workloads. You can specify what the node is used for with `--scopes`.

### Pinning workloads to a node scope

Node scopes are labels and taints applied to the node that tell Kubernetes what can run there. The two built-in scopes are:

- `workloads` — for user workloads
- `engine` — for Crossplane, providers, functions, and system components

```bash
# Add a node for engine components only
overlock env node create engine-node --environment my-env --scopes engine

# Add a node for workloads only
overlock env node create workload-node --environment my-env --scopes workloads
```

### Limiting CPU usage

If you're running a heavy provider and want to prevent it from using all your CPU:

```bash
overlock env node create engine-node --environment my-env --scopes engine --cpu 2
```

### Mounting a local directory

To make files from your host machine available inside the node:

```bash
overlock env node create my-node --environment my-env \
  --mount /path/on/host:/path/in/container
```

### Remove a local node

```bash
overlock env node delete my-extra-node --environment my-env
```

---

## Command Reference

### `overlock env node create <name>`

Adds a new node to an existing environment.

| Flag | Default | Description |
|------|---------|-------------|
| `--environment` | *(required)* | Name of the environment to add the node to |
| `--engine` | `k3s-docker` | Engine type (must match the environment's engine) |
| `--scopes` | — | Node role: `workloads`, `engine`, or both |
| `--cpu` | — | Maximum CPU this node can use (e.g. `2`, `0.5`, `50%`) |
| `--mount` | — | Bind mount in the format `/host/path:/container/path` |
| `--taints` | — | Kubernetes taints to apply to the node |

### `overlock env node delete <name>`

Removes a node from an environment.

| Flag | Default | Description |
|------|---------|-------------|
| `--environment` | *(required)* | Name of the environment |
| `--engine` | `k3s-docker` | Engine type |

---

## Related guides

- [Remote Nodes](remote-nodes.md) — add nodes from other machines via SSH
- [Environments](environments.md) — manage the overall environment lifecycle
