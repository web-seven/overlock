---
sidebar_label: Get Started
---

# Getting Started with Overlock

Overlock is a CLI tool that gives you a full Crossplane development environment on your laptop in minutes. Instead of manually standing up Kubernetes, installing Crossplane, wiring up registries, and managing packages, Overlock does all of that for you — so you can focus on building and testing your infrastructure abstractions.

This guide walks you through a complete, realistic development scenario: you'll create a local environment, install a community configuration to see how things fit together, then write and serve your own simple composition with live reload so changes appear in the cluster within seconds.

> [!NOTE]
> This guide assumes you're already familiar with Crossplane concepts like Composite Resource Definitions (XRDs), Compositions, and Providers. If you need a refresher, the [Crossplane concepts documentation](https://docs.crossplane.io/latest/concepts/) is the best place to start. Overlock doesn't change how those things work — it just makes the surrounding tooling much faster to work with.

---

## Before You Begin

You need two things installed and running:

- **Docker** — Overlock uses Docker to run your local Kubernetes cluster. Make sure the Docker daemon is running before you continue.
- **`overlock` CLI** — confirm it's available by running `overlock --version`.

> [!TIP]
> On macOS and Windows, Docker Desktop is the easiest way to get Docker running. On Linux, you can either install the Docker Engine package and add your user to the `docker` group (so you don't need `sudo`), or use Docker Desktop — see the note below for the extra step required.

> [!IMPORTANT]
> **Docker Desktop on Linux.** Overlock talks to Docker through the Docker Go SDK, which does not read Docker CLI contexts. If you use Docker Desktop on Linux, the `desktop-linux` context is active for the `docker` CLI but Overlock won't see the daemon. Export `DOCKER_HOST` so it points at the Docker Desktop socket before running Overlock:
>
> ```bash
> export DOCKER_HOST=unix://$HOME/.docker/desktop/docker.raw.sock
> ```
>
> Add the line to your shell profile (`~/.bashrc`, `~/.zshrc`) to make it permanent. The same variable works on macOS Docker Desktop, but is only required when the daemon socket isn't at the default `/var/run/docker.sock`.

---

## Step 1 — Create Your First Environment

An environment is a local Kubernetes cluster with Crossplane pre-installed. Think of it as your personal sandbox: you can install packages, create resources, break things, and throw it away when you're done — without touching any shared system.

Give it a name and run:

```bash
overlock env create my-first-env
```

Overlock will spin up a KinD (Kubernetes in Docker) cluster, install Crossplane inside it, and automatically switch your terminal's active Kubernetes context to point at the new environment. The whole process takes about a minute.

> [!NOTE]
> KinD is the default engine and is perfect for most development work. If you need a multi-node topology — for example, to separate Crossplane engine components from workloads, or to connect a remote machine — use the `k3s-docker` engine instead. See the [Environments guide](environments.md) for a full comparison.

Once the command returns, your environment is live. You can confirm Crossplane is running with:

```bash
kubectl get pods -n crossplane-system
```

---

## Step 2 — Install a Community Configuration

Before writing your own composition, it's worth installing a community configuration to see what a complete, working package looks like. The DevOps Toolkit `dot-application` configuration adds a high-level `AppClaim` resource type to your cluster — a realistic example of the kind of abstraction you'll be building.

```bash
overlock cfg apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
```

Overlock installs the configuration and waits until Crossplane reports it healthy before returning control to your terminal. Behind the scenes, Crossplane has registered new XRDs and loaded the compositions that back them.

> [!TIP]
> Configurations can declare dependencies on specific providers and functions. Crossplane resolves and installs those automatically when the configuration is applied. Run `overlock prv list` and `overlock fnc list` after the configuration is ready to see what was pulled in.

Confirm it's installed and look at what new resource types are now available:

```bash
overlock cfg list
overlock res list
```

You'll see `AppClaim` and the underlying composite type in the resource list. This is exactly what your own configuration will look like once you build and serve it.

---

## Step 3 — Set Up a Local Registry for Development

When you develop your own configuration, you need somewhere to push it so Overlock can install it into your environment. A local registry — running as a Docker container on your machine — is the fastest way to do this with no external accounts or internet access required.

```bash
overlock reg create --local --default
```

This starts a local OCI-compatible registry and marks it as the default destination for package operations.

> [!NOTE]
> You only need to create the local registry once. It persists across environment restarts, so you can reuse it for every project you work on. See the [Registries guide](registries.md) for the full local development cycle.

---

## Step 4 — Write a Simple Configuration Package

Now for the real work. You're going to create a minimal configuration package — an XRD and a Composition — and use Overlock's live-reload workflow to iterate on it quickly.

Create a directory for your package:

```bash
mkdir my-config && cd my-config
```

A Crossplane configuration package needs at minimum a `crossplane.yaml` metadata file, at least one XRD, and at least one Composition. Here's a simple example that defines a `Database` abstract resource type:

**`crossplane.yaml`**
```yaml
apiVersion: meta.pkg.crossplane.io/v1
kind: Configuration
metadata:
  name: my-config
spec:
  crossplane:
    version: ">=v1.14.0-0"
  dependsOn:
    - function: xpkg.upbound.io/crossplane-contrib/function-patch-and-transform
      version: ">=v0.7.0"
```

**`xrd.yaml`**
```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xdatabases.example.org
spec:
  group: example.org
  names:
    kind: XDatabase
    plural: xdatabases
  claimNames:
    kind: Database
    plural: databases
  versions:
    - name: v1alpha1
      served: true
      referenceable: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
                parameters:
                  type: object
                  properties:
                    size:
                      type: string
                      enum: [small, medium, large]
                  required: [size]
              required: [parameters]
```

**`composition.yaml`**
```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: Composition
metadata:
  name: xdatabases.example.org
spec:
  compositeTypeRef:
    apiVersion: example.org/v1alpha1
    kind: XDatabase
  mode: Pipeline
  pipeline:
    - step: patch-and-transform
      functionRef:
        name: function-patch-and-transform
      input:
        apiVersion: pt.fn.crossplane.io/v1beta1
        kind: Resources
        resources: []
```

> [!TIP]
> For a deep dive on XRDs and Compositions, see the [Crossplane Composite Resources documentation](https://docs.crossplane.io/latest/concepts/composite-resources/). Overlock doesn't change how these YAML files work — it just manages the packaging and deployment lifecycle around them.

---

## Step 5 — Serve Your Configuration with Live Reload

This is where Overlock's development workflow really shines. Instead of building, tagging, and pushing your package every time you make a change, you serve it directly from your local directory. Overlock watches your files, and every time you save a change it rebuilds the package, pushes it to your local registry, and reloads it in the cluster — usually in a matter of seconds.

From inside your package directory, run:

```bash
overlock cfg serve ./
```

Leave this running in one terminal. Open a second terminal to interact with your environment.

In the second terminal, confirm your new XRD registered correctly:

```bash
kubectl get xrds
```

You should see `xdatabases.example.org` in the list. Now try editing your composition — change something, save the file — and watch the serve output. Your updated composition is live in the cluster within seconds, no manual steps required.

> [!NOTE]
> The `serve` command requires a local registry to push packages to. If you haven't set one up yet, run `overlock reg create --local --default` first (Step 3).

---

## Step 6 — Create a Test Resource

With the configuration serving and your XRD registered, you can create an instance of your new resource type to verify the whole pipeline works.

Create a file called `test-database.yaml`:

```yaml
apiVersion: example.org/v1alpha1
kind: Database
metadata:
  name: test-db
  namespace: default
spec:
  parameters:
    size: small
  compositeDeletePolicy: Foreground
  writeConnectionSecretToRef:
    name: test-db-connection
```

Apply it:

```bash
overlock res apply -f test-database.yaml
```

Then watch its status:

```bash
kubectl get database test-db -o wide
```

> [!NOTE]
> For this minimal example, the composition pipeline doesn't provision any real infrastructure, so the resource will remain in a `Waiting` state. In a production-ready configuration you'd compose actual provider-managed resources in the pipeline. See the [Providers guide](providers.md) to add a cloud provider to your environment.

---

## Step 7 — Pause and Resume Your Environment

When you're done for the day, stop the environment to free up CPU and memory. Your installed packages and resources are preserved exactly as you left them:

```bash
overlock env stop my-first-env
```

Start it again the next morning and pick up where you left off:

```bash
overlock env start my-first-env
```

---

## Step 8 — Clean Up

When you're completely done with an environment and want to reclaim disk space, delete it:

```bash
overlock env delete my-first-env
```

> [!WARNING]
> Deleting an environment removes the cluster and everything inside it permanently — including any resources you've created. Your package source files and YAML manifests on your local filesystem are unaffected, but any state inside the cluster is gone. Make sure you've saved anything you need before deleting.

---

## What's Next

You've seen the full development loop: environment → registry → configuration development → live reload → resource creation. Each of these areas has more depth to explore:

- [Environments](environments.md) — engine options, multi-node setups, upgrading Crossplane
- [Configurations](configurations.md) — the full configuration development and publishing workflow
- [Providers](providers.md) — connect your environment to real cloud infrastructure
- [Functions](functions.md) — understand where composition functions fit and how to build your own
- [Resources](resources.md) — discover available resource types and manage the resource lifecycle
- [Registries](registries.md) — local and remote registry management for distributing packages
- [Local Nodes](local-nodes.md) — simulate multi-node cluster topologies on your laptop
- [Remote Nodes](remote-nodes.md) — join real machines to your environment over SSH
- [Plugins](plugins.md) — automate your team's workflow with custom Overlock commands
