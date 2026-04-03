# Registries

## What is it

A registry is a storage location for Crossplane packages — providers, configurations, and functions. When you run `overlock prv install <url>` or `overlock cfg apply <url>`, Overlock pulls the package from a registry.

Overlock supports two kinds:

- **Local registry** — runs as a container on your machine, great for development and testing
- **Remote registry** — a hosted OCI-compatible registry (like Docker Hub, GitHub Container Registry, or your own)

---

## When would I use it

**Local registry:** You're building a provider or configuration and want to test it locally before publishing anywhere. You push your package to the local registry and install it into your environment — no internet required, no external accounts needed.

**Remote registry:** You're pulling packages from a private registry that requires credentials, or you want to push packages to a team-shared registry so others can pull them.

---

## How to use it

### Create a local registry

```bash
overlock reg create --local --default
```

This starts a local OCI registry as a Docker container and sets it as the default for package operations. After this, you can load packages into it and install from it.

### Create a connection to a remote registry

```bash
overlock reg create \
  --registry-server registry.example.com \
  --username myuser \
  --password mypassword \
  --email user@example.com
```

This saves the credentials so Overlock can pull packages from the registry.

### List configured registries

```bash
overlock reg list
```

### Delete a registry

```bash
overlock reg delete --name my-registry
```

To delete the default registry:

```bash
overlock reg delete --name my-registry --default
```

### Load an OCI image into a registry

Once you've built a package, push it to your local registry:

```bash
overlock reg load-image \
  --registry my-local-registry \
  --path ./my-package.tar \
  --name my-provider:v0.1.0
```

This makes the package available to install from the registry. You can then install it just like any remote package.

---

## Command Reference

### `overlock reg create`

Connects to or starts a registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--local` | `false` | Create a local registry running as a container |
| `--default` | `false` | Set this as the default registry |
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
| `--default` | `false` | Also unset this as the default registry |

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

## Related guides

- [Providers](providers.md) — install providers from a registry
- [Configurations](configurations.md) — install configurations from a registry
- [Functions](functions.md) — install functions from a registry
