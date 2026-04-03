# Getting Started with Overlock

Overlock is a command-line tool that makes it easy to spin up Crossplane environments on your laptop or in the cloud, install packages, and manage resources — without needing to manually configure Kubernetes or Crossplane from scratch.

This guide walks you through the most common first-day flow: setting up an environment, installing a configuration, and creating a resource.

---

## Before You Begin

Make sure you have the following installed:

- Docker (running)
- `overlock` CLI — run `overlock --version` to confirm it's working

---

## Step 1 — Create an Environment

An environment is a local Kubernetes cluster with Crossplane already installed. You create one with a single command:

```bash
overlock env create my-first-env
```

Overlock will:
1. Create a Kubernetes cluster using KinD (Kubernetes in Docker)
2. Install Crossplane inside it
3. Set it as your active cluster context

This takes about a minute. When it finishes, your environment is ready to use.

> Want to learn more about environment options? See the [Environments guide](environments.md).

---

## Step 2 — Install a Configuration

A configuration is a bundle of Crossplane resources — composite resource definitions (XRDs), compositions, and dependencies — that adds new capabilities to your environment.

```bash
overlock cfg apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
```

Overlock installs the configuration and waits for it to become ready.

> Learn more: [Configurations guide](configurations.md)

---

## Step 3 — Check What's Installed

You can list everything that has been installed:

```bash
overlock cfg list
overlock prv list
overlock fnc list
```

---

## Step 4 — Create a Resource

Once a configuration is installed, it registers new resource types in your cluster. You can create one using a YAML file:

```bash
overlock res apply -f my-resource.yaml
```

Or list what's available:

```bash
overlock res list
```

> Learn more: [Resources guide](resources.md)

---

## Step 5 — Stop and Start Your Environment

When you're done for the day, stop your environment to free up resources:

```bash
overlock env stop my-first-env
```

Start it again later:

```bash
overlock env start my-first-env
```

---

## Step 6 — Clean Up

When you no longer need the environment, delete it:

```bash
overlock env delete my-first-env
```

---

## What's Next

Now that you know the basics, explore the individual feature guides:

- [Environments](environments.md) — create, stop, start, delete
- [Local Nodes](local-nodes.md) — add more nodes to your environment
- [Remote Nodes](remote-nodes.md) — connect real machines via SSH
- [Configurations](configurations.md) — install and manage Crossplane configurations
- [Providers](providers.md) — install and manage Crossplane providers
- [Functions](functions.md) — install and manage Crossplane functions
- [Registries](registries.md) — manage local and remote package registries
- [Resources](resources.md) — work with composite resources
- [Plugins](plugins.md) — extend Overlock with custom commands
