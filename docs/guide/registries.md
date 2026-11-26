# Registries

A registry is a storage location for Crossplane packages — providers, configurations, and functions. Every time you run `overlock cfg apply <url>` or `overlock prv install <url>`, Overlock pulls that package from a registry.

Overlock supports two kinds:

- **Local registry** — an OCI-compatible registry running as a Docker container on your machine, with no external accounts or internet access required. Essential for local package development.
- **Remote registry** — a hosted OCI registry such as Docker Hub, GitHub Container Registry, or your own self-hosted instance. Used for pulling published packages or distributing packages to your team.

---

## The Local Development Cycle

The most important thing registries enable is a fully self-contained development workflow. When you're building a configuration, provider, or function, the cycle looks like this:

1. Start a local registry
2. Build your package into an OCI archive
3. Load the archive into the local registry
4. Install from the local registry into your environment

Overlock's `serve` command (available for [configurations](configurations.md), [providers](providers.md), and [functions](functions.md)) automates steps 2–4 on every file change. But understanding the manual steps is useful when working with pre-built artifacts.

---

## Setting Up a Local Registry

Before you can develop packages locally, you need a local registry in the environment:

```bash
overlock reg create --local
```

This deploys an OCI registry into the environment (`registry.overlock.svc.cluster.local`) and makes Crossplane trust its certificate. `load` and `serve` commands create it automatically when it is missing.

Crossplane v2 has no default registry: every package reference must name its registry. Packages loaded by Overlock without a registry in their name are installed from the local registry, e.g. `my-provider:v0.1.0` becomes `registry.overlock.svc.cluster.local/my-provider:v0.1.0`.

### Redirecting Packages to the Local Registry

To develop a package that other packages depend on, redirect its upstream repository to the local registry with `--rewrite`:

```bash
overlock reg create --local --rewrite xpkg.crossplane.io/crossplane-contrib/
overlock prv load xpkg.crossplane.io/crossplane-contrib/provider-nop:v0.5.0 --path ./provider-nop.xpkg --apply
```

Packages and dependencies under `xpkg.crossplane.io/crossplane-contrib/` are now pulled from `registry.overlock.svc.cluster.local/crossplane-contrib/`: a Provider or Configuration that references `xpkg.crossplane.io/crossplane-contrib/provider-nop:v0.5.0` gets your local build, so the same manifests and `crossplane.yaml` dependencies work locally and on remote clusters.

`load` with a fully qualified name stores the package where its rewrite points (here `crossplane-contrib/provider-nop:v0.5.0` in the local registry) and `--apply` keeps the upstream name in `spec.package`; Crossplane shows the local source in `status.resolvedPackage`. A fully qualified name without a rewrite to the local registry is rejected, with the `--rewrite` to add. Names without a registry (e.g. `my-provider:v0.1.0`) are loaded and installed from the local registry directly, without rules.

Rewrites are Crossplane `ImageConfig` rules, managed by the Overlock operator from the `registries` of the [Environment](../environment/spec.md). `--rewrite` can be repeated and replaces the registry's previous rewrites; running `overlock reg create --local --rewrite ...` again updates them.

Confirm it's running:

```bash
overlock reg list
```

---

## Loading a Package into the Local Registry

Once you've built a package using `crossplane xpkg build` or equivalent tooling, push the resulting archive to your local registry:

```bash
overlock reg load-image \
  --registry my-local-registry \
  --path ./my-provider.tar \
  --name my-provider:v0.1.0
```

> [!TIP]
> The `--name` flag sets the image name and tag that the package will be addressable by inside the registry. Use a consistent naming convention — for example `my-org/my-provider:v0.1.0` — so your install commands are predictable.

If you're iterating and want to overwrite an existing version:

```bash
overlock reg load-image \
  --registry my-local-registry \
  --path ./my-provider.tar \
  --name my-provider:v0.1.0 \
  --upgrade
```

---

## Installing from the Local Registry

Install a loaded package with `--apply`, or later with `apply`, using the name it was loaded with:

```bash
overlock prv load my-provider:v0.1.0 --path ./my-provider.xpkg --apply
overlock prv apply my-provider:v0.1.0
```

Names without a registry refer to the local registry, so the Provider is created with `registry.overlock.svc.cluster.local/my-provider:v0.1.0`. In your own manifests, use that full address.

### Runtime Images on Nodes

Providers and functions run as pods whose image is the package itself. `load` and `serve` also write the package as an image archive into the images volume of the environment, `overlock-images-<environment>`. Local nodes created by `overlock env create` mount it as the K3s images directory (`/var/lib/rancher/k3s/agent/images`), and K3s imports new archives into containerd while running, under the local registry name and the node port name pods are rewritten to. The pods then start without pulling:

```text
Container image "localhost:32227/my-provider:v0.1.0" already present on machine
```

The archive keeps the image digest of the registry, and loading a new version replaces it. Environments without the volume, remote nodes and other engines pull from the local registry as before. Any K3s based environment can opt in by mounting a volume with that name as the images directory, as the [Compose file](../environment/compose.md) does; it must exist when K3s starts.
---

## Connecting to a Remote Registry

If you need to pull from a private registry — a team registry, a cloud provider's container registry, or your own hosted instance — register it with credentials:

```bash
overlock reg create \
  --registry-server registry.example.com \
  --username myuser \
  --password mypassword \
  --email user@example.com
```

Overlock saves these credentials and adds a rule to the Environment, so Crossplane uses them when pulling packages from that registry. Add `--rewrite <prefix>` to pull packages of another registry from this one instead, e.g. a mirror.

> [!TIP]
> For GitHub Container Registry (`ghcr.io`), use your GitHub username and a personal access token with `read:packages` scope as the password. For AWS ECR, generate temporary credentials with `aws ecr get-login-password` and use `AWS` as the username.

> [!WARNING]
> Be careful not to commit registry credentials to version control. Store them in environment variables or a secrets manager and pass them to `overlock reg create` dynamically in your scripts.

---

## Removing a Registry

To remove a registry configuration and its rules:

```bash
overlock reg delete --name my-registry
```

---

## Command Reference

### `overlock reg create`

Connects to or starts a registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--local` | `false` | Create a local registry running as a container |
| `--rewrite` | — | Fully qualified image prefix pulled from this registry instead, e.g. `xpkg.crossplane.io/crossplane-contrib/`; also applies to dependencies. Repeatable |
| `--registry-server` | — | Hostname of the remote registry |
| `--username` | — | Registry username |
| `--password` | — | Registry password |
| `--email` | — | Email address associated with the registry account |
| `--context` / `-c` | — | Kubernetes context to use |

### `overlock reg list`

Lists all configured registries. No flags.

### `overlock reg delete`

Removes a registry configuration.

| Flag | Default | Description |
|------|---------|-------------|
| `--name` | *(required)* | Name of the registry to remove |

### `overlock reg load-image`

Loads an OCI image into a registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--registry` | *(required)* | Name of the registry to load the image into |
| `--path` | *(required)* | Path to the OCI archive file |
| `--name` / `-i` | *(required)* | Image name and tag (e.g. `my-provider:v0.1.0`) |
| `--upgrade` | `false` | Overwrite if the image already exists |
| `--helm` | `false` | Treat the image as a Helm chart |

---

## Related Guides

- [Configurations](configurations.md) — use the local registry with `overlock cfg serve` and `overlock cfg load`
- [Providers](providers.md) — develop and test providers against a local registry
- [Functions](functions.md) — develop and test functions against a local registry
- [Getting Started](getting-started.md) — end-to-end walkthrough that includes local registry setup
