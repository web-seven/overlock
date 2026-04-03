# Environments

An environment is a self-contained Crossplane control plane running on your machine or a remote server. It's a Kubernetes cluster with Crossplane already installed and ready to accept providers, configurations, and resources. Overlock handles all the Kubernetes setup for you — you just give it a name.

The most common use case is local development: you create an environment on your laptop, experiment freely, and delete it when you're done. Because environments are completely isolated, there's no risk of affecting shared or production clusters.

> [!NOTE]
> If you're new to Overlock, start with the [Getting Started guide](getting-started.md) to see how environments fit into the full development workflow before diving into the details here.

---

## Choosing an Engine

When you create an environment, you choose the engine that runs the underlying Kubernetes cluster. The right choice depends on what you're building:

| Engine | Best for |
|--------|----------|
| `kind` *(default)* | Quick local development, single-node, simplest setup |
| `k3s-docker` | Multi-node topologies, local and remote nodes, production-mirroring |
| `k3s` | Running k3s directly on Linux (no Docker wrapper) |
| `k3d` | k3s inside Docker, lightweight alternative to KinD |

> [!TIP]
> If you're just getting started, `kind` is the right choice. Switch to `k3s-docker` when you want to add extra nodes — either [local Docker containers](local-nodes.md) or [remote machines over SSH](remote-nodes.md).

---

## Creating an Environment

Pick a name and run:

```bash
overlock env create my-env
```

Overlock creates the Kubernetes cluster, installs Crossplane inside it, and switches your terminal's active Kubernetes context to the new environment. After about a minute, you're ready to start installing packages.

If you already know which packages you'll need, you can install them at creation time to save a round-trip:

```bash
overlock env create my-env \
  --configurations xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31 \
  --providers xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0
```

> [!TIP]
> Pre-installing packages at creation time is useful in scripts and CI pipelines where you want a fully ready environment in a single command.

### Using the k3s-docker engine

When you need to add nodes to your environment later, create it with `k3s-docker` from the start. You can't change the engine after creation:

```bash
overlock env create my-env --engine k3s-docker
```

Once this is done, you can expand the cluster by adding [local nodes](local-nodes.md) or [remote nodes](remote-nodes.md).

### Exposing HTTP and HTTPS ports

By default, Overlock maps the cluster's ingress to ports 80 and 443 on your machine. If those ports are already in use, pick different ones:

```bash
overlock env create my-env --http-port 8080 --https-port 8443
```

### Tuning reconciliation performance

If you're working with a large number of managed resources and reconciliation feels slow, you can increase the reconcile rate. The default is `1`, which is conservative:

```bash
overlock env create my-env --max-reconcile-rate 10
```

> [!NOTE]
> Higher reconcile rates consume more CPU. On a development laptop, a value between `5` and `10` is usually a good balance. See the [Crossplane configuration documentation](https://docs.crossplane.io/latest/concepts/crossplane/) for more background on what this controls.

---

## Stopping and Starting an Environment

When you're not actively using an environment, stop it to free up CPU and memory. Everything you've installed is preserved:

```bash
overlock env stop my-env
```

Resume exactly where you left off:

```bash
overlock env start my-env
```

If you use multiple environments and want to switch your active Kubernetes context at the same time:

```bash
overlock env start my-env --switch
```

---

## Upgrading Crossplane

When a new version of Crossplane is released and you want to update an existing environment without recreating it:

```bash
overlock env upgrade my-env
```

> [!WARNING]
> Upgrading Crossplane in place may cause brief disruption to running reconciliation loops. For critical development work, consider creating a fresh environment on the new version rather than upgrading in place.

---

## Deleting an Environment

When you're completely done with an environment:

```bash
overlock env delete my-env
```

Overlock shows a confirmation prompt before proceeding. To skip the prompt in scripts:

```bash
overlock env delete my-env --confirm
```

> [!WARNING]
> Deletion is permanent. The cluster and all resources inside it are removed. Your local source files and YAML manifests are unaffected.

---

## Where to Go From Here

With an environment created, the natural next step is to install packages. Head to the [Configurations guide](configurations.md) to install your first configuration, or the [Providers guide](providers.md) if you want to connect to a cloud platform right away.

If you want to expand your environment with additional nodes — for example, to isolate Crossplane components from workloads — see [Local Nodes](local-nodes.md) or [Remote Nodes](remote-nodes.md).

---

## Command Reference

### `overlock env create <name>`

Creates a new environment with the given name.

| Flag | Default | Description |
|------|---------|-------------|
| `--engine` | `kind` | Kubernetes engine: `kind`, `k3s`, `k3d`, `k3s-docker` |
| `--engine-config` | — | Path to an engine-specific config file |
| `--context` | — | Kubernetes context name to create or use |
| `--http-port` / `-p` | `80` | Local port to expose for HTTP traffic |
| `--https-port` / `-s` | `443` | Local port to expose for HTTPS traffic |
| `--providers` | — | Providers to install at creation time |
| `--configurations` | — | Configurations to install at creation time |
| `--functions` | — | Functions to install at creation time |
| `--cpu` | — | Maximum CPU each container node can use (e.g. `2`, `0.5`, `50%`) |
| `--max-reconcile-rate` | `1` | Number of resources Crossplane processes concurrently |
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

## Related Guides

- [Getting Started](getting-started.md) — end-to-end walkthrough from zero to a running resource
- [Local Nodes](local-nodes.md) — add worker nodes as Docker containers on your machine
- [Remote Nodes](remote-nodes.md) — connect real machines to your environment via SSH
- [Configurations](configurations.md) — install and develop Crossplane configurations
