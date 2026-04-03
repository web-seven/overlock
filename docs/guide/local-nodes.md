# Local Nodes

By default, an Overlock environment is a single-node Kubernetes cluster — one container running both the control plane and any workloads. That's perfect for most development work. But sometimes you want a more realistic topology: separating Crossplane engine components from user workloads, simulating how your cluster will look in production, or testing that your scheduling constraints work correctly.

A local node is an extra Docker container on the same machine that joins your cluster as an additional worker. You can add as many as you need, and you control what each one is responsible for using node scopes.

> [!NOTE]
> Local nodes require the `k3s-docker` engine. If you created your environment with the default `kind` engine, you'll need to create a new environment with `--engine k3s-docker` to use this feature. You can't change the engine of an existing environment.

---

## Why Add Local Nodes?

The most common reasons to add extra nodes during development:

**Topology isolation** — Run Crossplane's engine components (providers, functions, cert-manager) on a dedicated node, and keep user workloads on a separate node. This mirrors how a production cluster might be structured and lets you verify that your Kubernetes scheduling rules (node selectors, tolerations, affinity) work as intended.

**Resource isolation** — A CPU-hungry provider running alongside everything else can slow down your development machine noticeably. Put it on a dedicated node with a CPU cap and it can't starve the rest of your environment.

**Multi-node testing** — Some Crossplane features behave differently in multi-node clusters. If you're building something that needs to validate cross-node behavior, local nodes let you do that without extra hardware.

---

## Creating an Environment Ready for Extra Nodes

If you're starting fresh, create your environment with the `k3s-docker` engine:

```bash
overlock env create my-env --engine k3s-docker
```

> [!TIP]
> You can also pre-install packages at creation time to get a fully ready environment in one command: `overlock env create my-env --engine k3s-docker --configurations xpkg.upbound.io/...`

---

## Adding a Local Node

Once the environment is running, add a node:

```bash
overlock env node create my-extra-node --environment my-env
```

Overlock creates a new Docker container, configures it as a k3s agent, and joins it to the cluster. The node will appear in `kubectl get nodes` within a few seconds.

### Assigning a scope to the node

Node scopes are labels and taints applied to the node that tell Kubernetes what can be scheduled there. Overlock supports two built-in scopes:

- `workloads` — for user workloads and application pods
- `engine` — for Crossplane itself, providers, functions, cert-manager, and other infrastructure components

```bash
# A node dedicated to Crossplane engine components
overlock env node create engine-node --environment my-env --scopes engine

# A node dedicated to user workloads
overlock env node create workload-node --environment my-env --scopes workloads
```

> [!TIP]
> If you don't specify a scope, the node is a general-purpose worker with no special labels or taints. Adding `--scopes engine` is the most useful configuration for development: it lets you move Crossplane's components off the control plane node and test that your providers behave correctly when scheduled with appropriate tolerations.

### Limiting CPU usage

To prevent a node from consuming all available CPU on your machine:

```bash
overlock env node create engine-node \
  --environment my-env \
  --scopes engine \
  --cpu 2
```

The `--cpu` value can be a number of cores (`2`), a decimal fraction (`0.5`), or a percentage (`50%`).

### Mounting a host directory

If your workloads need access to files on your machine — for example, a local package directory or test fixtures — bind-mount it into the node:

```bash
overlock env node create my-node \
  --environment my-env \
  --mount /path/on/host:/path/in/container
```

---

## Removing a Node

When you no longer need the extra node:

```bash
overlock env node delete my-extra-node --environment my-env
```

The Docker container is stopped and removed, and the node is gracefully removed from the cluster.

> [!NOTE]
> Any pods that were running on the deleted node will be rescheduled to remaining nodes by Kubernetes, subject to normal scheduling rules. If a pod was tainted to only run on the deleted node's scope, it may go `Pending` until you add another node with the same scope.

---

## Command Reference

### `overlock env node create <name>`

Adds a new local node to an existing environment.

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

## Related Guides

- [Remote Nodes](remote-nodes.md) — add nodes from physical servers or VMs over SSH
- [Environments](environments.md) — create and manage the overall environment lifecycle
- [Getting Started](getting-started.md) — end-to-end walkthrough
