# Environments

## What is it

An environment is a self-contained Crossplane control plane running on your machine (or a remote server). It's a Kubernetes cluster with Crossplane already installed and configured, ready to accept providers, configurations, and resources.

You don't need to set up Kubernetes manually — Overlock handles all of that for you.

---

## When would I use it

You want to test a new Crossplane configuration on your laptop before deploying it to a shared or production cluster. You create a local environment, experiment freely, then delete it when you're done — no cleanup required on any shared system.

---

## How to use it

### Create an environment

Pick a name and run:

```bash
overlock env create my-env
```

Overlock creates a local Kubernetes cluster and installs Crossplane inside it. When the command finishes, the environment is ready and your terminal's Kubernetes context has been switched to it.

If you want to pre-install packages at creation time, pass them as flags:

```bash
overlock env create my-env \
  --configurations xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31 \
  --providers xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0
```

### Stop an environment

When you're not using it, stop the environment to free up CPU and memory:

```bash
overlock env stop my-env
```

The cluster is paused but not deleted. Everything you've installed stays in place.

### Start an environment

Resume where you left off:

```bash
overlock env start my-env
```

### Delete an environment

When you're completely done with an environment and want to remove it:

```bash
overlock env delete my-env
```

This removes the cluster and all resources inside it. There's a confirmation prompt by default to prevent accidental deletion.

### Upgrade Crossplane

To update the Crossplane version running inside an environment:

```bash
overlock env upgrade my-env
```

---

## Command Reference

### `overlock env create <name>`

Creates a new environment with the given name.

| Flag | Default | Description |
|------|---------|-------------|
| `--engine` | `kind` | Kubernetes engine to use: `kind`, `k3s`, `k3d`, `k3s-docker` |
| `--engine-config` | — | Path to an engine-specific config file |
| `--context` | — | Kubernetes context name to create or use |
| `--http-port` / `-p` | `80` | Local port to expose for HTTP traffic |
| `--https-port` / `-s` | `443` | Local port to expose for HTTPS traffic |
| `--providers` | — | Providers to install at creation time |
| `--configurations` | — | Configurations to install at creation time |
| `--functions` | — | Functions to install at creation time |
| `--cpu` | — | Maximum CPU each container node can use (e.g. `2`, `0.5`, `50%`) |
| `--max-reconcile-rate` | `1` | How many resources Crossplane processes at once — increase if reconciliation feels slow |
| `--create-admin-service-account` | `false` | Create a cluster-admin service account |
| `--admin-service-account-name` | — | Name for the admin service account |
| `--mount-path` | — | Host path to bind-mount into the cluster |
| `--container-path` | `/storage` | Path inside the container to mount to |

### `overlock env delete <name>`

Deletes the environment and all resources inside it.

| Flag | Default | Description |
|------|---------|-------------|
| `--engine` | `kind` | Engine type used when the environment was created |
| `--confirm` / `-c` | `false` | Skip the confirmation prompt |

### `overlock env stop <name>`

Stops a running environment without deleting anything.

| Flag | Default | Description |
|------|---------|-------------|
| `--engine` | `kind` | Engine type |

### `overlock env start <name>`

Starts a stopped environment.

| Flag | Default | Description |
|------|---------|-------------|
| `--engine` | `kind` | Engine type |
| `--switch` / `-s` | `false` | Also switch your active Kubernetes context to this environment |

### `overlock env upgrade <name>`

Upgrades Crossplane inside an existing environment.

| Flag | Default | Description |
|------|---------|-------------|
| `--engine` | `kind` | Engine type |
| `--context` | — | Kubernetes context name |
| `--create-admin-service-account` | `false` | Create a cluster-admin service account |
| `--admin-service-account-name` | — | Name for the admin service account |

---

## Related guides

- [Local Nodes](local-nodes.md) — add more worker nodes to your environment
- [Remote Nodes](remote-nodes.md) — connect machines via SSH as cluster nodes
- [Getting Started](getting-started.md) — end-to-end walkthrough
