# Configurations

## What is it

A Crossplane configuration is a package that bundles together resource definitions and compositions — the building blocks that define what kinds of infrastructure or services your cluster can create. Installing a configuration is how you add new capabilities to your environment.

Think of it like installing an app: before the app exists, the cluster doesn't know how to create that type of resource. After the configuration is installed, it does.

---

## When would I use it

You want to give your environment the ability to create cloud databases, Kubernetes clusters, or any other resource type that doesn't exist by default. You find or build a configuration package and install it with Overlock.

---

## How to use it

### Install a configuration

```bash
overlock cfg apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
```

Overlock installs the configuration and waits for it to become ready before returning. If you don't want to wait:

```bash
overlock cfg apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31 --wait=false
```

### List installed configurations

```bash
overlock cfg list
```

### Remove a configuration

```bash
overlock cfg delete xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
```

### Load from a local file

If you've built a configuration locally and exported it as an OCI archive:

```bash
overlock cfg load --path ./my-config.tar --apply
```

Or from stdin (useful in CI pipelines):

```bash
cat my-config.tar | overlock cfg load --stdin --apply
```

### Serve a configuration for live development

If you're actively developing a configuration package, you can serve it directly from your local directory. Overlock watches for file changes and automatically rebuilds and reloads the configuration in the cluster:

```bash
overlock cfg serve ./my-config-package
```

This gives you a fast feedback loop — edit your package files, and changes show up in the cluster within seconds.

---

## Command Reference

### `overlock cfg apply <url>`

Installs a configuration from a remote registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--wait` / `-w` | `true` | Wait for the configuration to become ready |
| `--timeout` / `-t` | — | How long to wait before giving up |

### `overlock cfg list`

Lists all configurations currently installed in the active environment. No flags.

### `overlock cfg delete <url>`

Removes an installed configuration. Pass the same URL used when installing.

### `overlock cfg load`

Loads a configuration from a local archive file or stdin.

| Flag | Default | Description |
|------|---------|-------------|
| `--path` | — | Path to the archive file |
| `--stdin` | `false` | Read the archive from stdin |
| `--apply` | `false` | Apply the configuration immediately after loading |
| `--upgrade` | `false` | Upgrade if the configuration is already installed |

### `overlock cfg serve <path>`

Watches a local directory for changes and live-reloads the configuration.

| Argument | Default | Description |
|----------|---------|-------------|
| `path` | `./` | Path to the configuration package directory |

---

## Related guides

- [Providers](providers.md) — install providers that configurations often depend on
- [Functions](functions.md) — install composition functions used by configurations
- [Getting Started](getting-started.md) — end-to-end walkthrough
