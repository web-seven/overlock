---
slug: /environment/cfg-file
---

# Environment Config File

Instead of passing every flag on the command line, you can supply environment creation options through a YAML config file. Overlock supports both a single explicit config file and layered automatic discovery.

---

## File Discovery (Layered Loading)

When no `--config` flag is provided, Overlock automatically looks for config files in the current directory and merges them in the following order (later files override earlier ones):

1. `overlock.yaml`
2. `.overlock.yaml`
3. `.overlock.*.yaml` — any files matching this glob, loaded in alphabetical order (e.g. `.overlock.dev.yaml`, `.overlock.ci.yaml`)

This layered approach lets you keep a base config in `overlock.yaml` and override specific values per environment or context in additional files.

## Explicit Config File

To use a specific file, pass the `--config` flag:

```bash
overlock env create my-env --config ./my-config.yaml
```

If the file is not found at the given path, Overlock exits with an error. If the file exists but cannot be parsed, Overlock logs a message and points you to this page.

---

## File Format

The config file is a YAML document. All fields are optional — only set the ones you want to override.

```yaml
engine: kind
http_port: 80
https_port: 443
context: ""
engine_config: ""
mount: []
providers: []
configurations: []
functions: []
create_admin_service_account: false
admin_service_account_name: ""
cpu: ""
max_reconcile_rate: 1
```

### Field Reference

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `engine` | string | `kind` | Kubernetes engine to use: `kind`, `k3s`, `k3d`, `k3s-docker` |
| `http_port` | int | `80` | Host port to map for HTTP traffic |
| `https_port` | int | `443` | Host port to map for HTTPS traffic |
| `context` | string | — | Kubernetes context name to create or use |
| `engine_config` | string | — | Path to an engine-specific config file (e.g. a KinD config YAML) |
| `mount` | list of strings | — | Bind mounts in `host:container` format, e.g. `/data:/storage` |
| `providers` | list of strings | — | Providers to install at creation time |
| `configurations` | list of strings | — | Configurations to install at creation time |
| `functions` | list of strings | — | Functions to install at creation time |
| `create_admin_service_account` | bool | `false` | Create a `cluster-admin` service account |
| `admin_service_account_name` | string | `overlock-admin` | Name for the admin service account |
| `cpu` | string | — | CPU limit for `k3s-docker` container nodes (e.g. `2`, `0.5`, `50%`) |
| `max_reconcile_rate` | int | `1` | Max concurrent reconciliations for Crossplane |

---

## Examples

### Minimal config

```yaml
engine: k3s-docker
max_reconcile_rate: 5
```

### Pre-installing packages

```yaml
engine: kind
configurations:
  - xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
providers:
  - xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0
```

### Layered config workflow

Keep a base file checked into your project:

```yaml
# overlock.yaml
engine: kind
max_reconcile_rate: 1
```

Add a local override that you `.gitignore`:

```yaml
# .overlock.local.yaml
max_reconcile_rate: 10
http_port: 8080
https_port: 8443
```

Overlock merges both files, with `.overlock.local.yaml` taking precedence.

---

## Related

- [Environments](environments.md) — full guide to creating and managing environments
- [Getting Started](../guide/getting-started.md) — end-to-end walkthrough
