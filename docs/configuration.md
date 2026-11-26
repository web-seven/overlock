# Configuration

This document describes how to configure Overlock CLI for your environment.

## Table of Contents

- [Global Options](#global-options)
- [Environment Variables](#environment-variables)
- [Plugin Configuration](#plugin-configuration)
- [Kubeconfig](#kubeconfig)

## Global Options

Overlock supports several global options that can be used with any command:

```bash
overlock [global-options] <command>
```

### Available Global Options

| Option | Short | Description | Default |
|--------|-------|-------------|---------|
| `--debug` | `-D` | Enable debug mode for verbose output | `false` |
| `--namespace` | `-n` | Namespace for cluster resources | `crossplane-system` |
| `--engine-release` | `-r` | Crossplane Helm release name | `crossplane` |
| `--engine-version` | `-v` | Crossplane version for new environments, at least `2.0.0` | operator default (`2.5.0`) |
| `--plugin-path` | | Path to plugin directory | `~/.config/overlock/plugins` |

### Usage Examples

**Enable debug mode:**
```bash
overlock --debug environment create my-env
```

**Use custom namespace:**
```bash
overlock --namespace my-namespace provider list
```

**Install specific Crossplane version:**
```bash
overlock --engine-version 1.18.0 environment create my-env
```

**Combine multiple options:**
```bash
overlock --debug --namespace custom-ns --engine-version 2.5.0 environment create my-env
```

## Environment Variables

You can set environment variables to avoid repeating the same options:

### `OVERLOCK_ENGINE_NAMESPACE`

Default namespace for Crossplane resources.

```bash
export OVERLOCK_ENGINE_NAMESPACE=custom-namespace
overlock environment create my-env
```

### `OVERLOCK_ENGINE_RELEASE`

Default Helm release name for Crossplane installations.

```bash
export OVERLOCK_ENGINE_RELEASE=my-crossplane
overlock environment create my-env
```

### `OVERLOCK_ENGINE_VERSION`

Default Crossplane version to install.

```bash
export OVERLOCK_ENGINE_VERSION=1.18.0
overlock environment create my-env
```

### Example Configuration

Add these to your `~/.bashrc` or `~/.zshrc`:

```bash
# Overlock Configuration
export OVERLOCK_ENGINE_NAMESPACE=crossplane-system
export OVERLOCK_ENGINE_RELEASE=crossplane
export OVERLOCK_ENGINE_VERSION=2.5.0
```

## Plugin Configuration

Overlock supports a plugin system for extending functionality.

### Plugin Directory

Default plugin path: `~/.config/overlock/plugins`

You can override this with:

```bash
overlock --plugin-path /path/to/plugins <command>
```

Or set it via environment variable:

```bash
export OVERLOCK_PLUGIN_PATH=/path/to/plugins
```

### Plugin Structure

Plugins should be placed in the configured plugin directory. Each plugin is a standalone executable that follows the Overlock plugin protocol.

## Kubeconfig

Overlock uses the standard Kubernetes configuration for cluster authentication:

- Default location: `~/.kube/config`
- Respects `KUBECONFIG` environment variable
- Uses the current context by default

### Working with Multiple Contexts

```bash
# List available contexts
kubectl config get-contexts

# Switch context
kubectl config use-context <context-name>

# Run Overlock with specific context
kubectl config use-context my-cluster
overlock environment list
```

## Resource Labels

Overlock automatically labels resources it manages with:

```yaml
metadata:
  labels:
    app.kubernetes.io/managed-by: overlock
```

This allows you to identify and filter resources managed by Overlock:

```bash
kubectl get all -l app.kubernetes.io/managed-by=overlock
```

## Registry Configuration

### Local Registry

When creating a local registry, Overlock sets up:
- A Docker registry container
- Port mapping (typically 5000:5000)
- Automatic integration with your environment

```bash
overlock registry create --local --default
```

### Remote Registry

For remote registries, you'll need:
- Registry URL
- Authentication credentials
- Email address

```bash
overlock registry create \
  --registry-server=registry.example.com \
  --username=myuser \
  --password=mypass \
  --email=user@example.com
```

### Registry Storage

Registry configurations are stored in the Kubernetes cluster as secrets and configmaps.

## Advanced Configuration

### Custom Helm Values

While not directly exposed through CLI flags, you can customize Crossplane Helm installations by modifying the Helm chart values that Overlock uses internally.

### Network Configuration

Overlock creates Kubernetes clusters with default networking. Ensure:
- Docker daemon is running
- Required ports are available
- No firewall blocking cluster communication

### Resource Requirements

Minimum requirements for running Overlock environments:
- **CPU**: 2 cores recommended
- **Memory**: 4GB RAM minimum, 8GB recommended
- **Disk**: 10GB free space per environment
- **Docker**: 20.10+ or compatible container runtime
