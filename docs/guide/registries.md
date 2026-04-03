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

Before you can develop packages locally, you need a local registry. Create one and set it as the default:

```bash
overlock reg create --local --default
```

This starts a local OCI registry as a Docker container and registers it as the default destination for package load and serve operations.

> [!NOTE]
> You only need to do this once. The registry container persists across environment restarts — it's not tied to any specific environment, so it works with all your environments.

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

After loading a package, install it from the local registry the same way you'd install from any remote registry — just use the local registry's address as the package URL:

```bash
overlock prv install localhost:5000/my-provider:v0.1.0
```

> [!NOTE]
> The exact hostname and port depend on how your local registry was created and how Overlock configured it. Run `overlock reg list` to see the registry address.

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

Overlock saves these credentials and uses them automatically when pulling packages from that registry.

> [!TIP]
> For GitHub Container Registry (`ghcr.io`), use your GitHub username and a personal access token with `read:packages` scope as the password. For AWS ECR, generate temporary credentials with `aws ecr get-login-password` and use `AWS` as the username.

> [!WARNING]
> Be careful not to commit registry credentials to version control. Store them in environment variables or a secrets manager and pass them to `overlock reg create` dynamically in your scripts.

---

## Removing a Registry

To remove a registry configuration:

```bash
overlock reg delete --name my-registry
```

If it was set as the default registry, pass `--default` to unset it at the same time:

```bash
overlock reg delete --name my-registry --default
```

---

## Command Reference

### `overlock reg create`

Connects to or starts a registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--local` | `false` | Create a local registry running as a container |
| `--default` | `false` | Set this registry as the default for package operations |
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
| `--default` | `false` | Also unset this registry as the default |

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
