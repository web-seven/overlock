# Providers

## What is it

A Crossplane provider is a plugin that teaches your cluster how to talk to an external system — a cloud platform, a database service, a DNS provider, or anything else that has an API. Once a provider is installed, you can use Kubernetes-style resources to manage infrastructure on that system.

For example, installing the AWS provider lets you create S3 buckets, EC2 instances, or RDS databases using YAML files, just like creating any other Kubernetes resource.

---

## When would I use it

You've set up a Crossplane environment and now want it to actually manage something. You install the provider for the platform you're targeting (AWS, GCP, Azure, GitHub, etc.), configure its credentials, and then you can start creating resources.

---

## How to use it

### Install a provider

```bash
overlock prv install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0
```

Overlock installs the provider into your active environment.

### List installed providers

```bash
overlock prv list
```

### Remove a provider

```bash
overlock prv delete xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0
```

Pass the same package URL you used when installing.

### Load from a local archive

If you have a provider built and exported as a local archive:

```bash
overlock prv load --path ./my-provider.tar --apply
```

### Serve a provider for live development

If you're building a provider and want fast feedback while making changes:

```bash
overlock prv serve ./my-provider
```

Overlock watches for changes in the local directory and hot-reloads the provider in your environment. You can also specify a custom path to the provider's main package:

```bash
overlock prv serve ./my-provider --main-path cmd/my-provider
```

---

## Command Reference

### `overlock prv install <url>`

Installs a provider from a remote registry.

| Argument | Description |
|----------|-------------|
| `url` | The full package URL including version tag |

### `overlock prv list`

Lists all providers currently installed in the active environment. No flags.

### `overlock prv delete <url>`

Removes an installed provider.

| Argument | Description |
|----------|-------------|
| `url` | The package URL used when the provider was installed |

### `overlock prv load`

Loads a provider from a local archive file.

| Flag | Default | Description |
|------|---------|-------------|
| `--path` | — | Path to the archive file |
| `--apply` | `false` | Apply the provider immediately after loading |
| `--upgrade` | `false` | Upgrade if the provider is already installed |

### `overlock prv serve <path>`

Watches a local directory and hot-reloads the provider on changes.

| Argument | Default | Description |
|----------|---------|-------------|
| `path` | `./` | Path to the provider source directory |

| Flag | Default | Description |
|------|---------|-------------|
| `--main-path` | `cmd/provider` | Relative path to the provider's main package |

---

## Related guides

- [Configurations](configurations.md) — configurations often require specific providers to be installed
- [Registries](registries.md) — set up a local registry to push and pull providers during development
- [Getting Started](getting-started.md) — end-to-end walkthrough
