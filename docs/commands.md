# Command Reference

This document provides detailed information about all Overlock CLI commands.

## Table of Contents

- [Environment Management](#environment-management)
- [Provider Management](#provider-management)
- [Configuration Management](#configuration-management)
- [Function Management](#function-management)
- [Registry Management](#registry-management)
- [Resource Management](#resource-management)
- [Command Aliases](#command-aliases)

## Environment Management

Create and manage Crossplane-enabled Kubernetes environments.

### `overlock environment create`

Create a new Crossplane environment.

```bash
overlock environment create <name> [options]
```

**Options:**
- `--engine`: Kubernetes engine to use (kind, k3s, k3d, k3s-docker)
- `--crossplane-version`: Specific Crossplane version to install
- `--cpu`: CPU limit for k3s-docker containers (e.g., `2`, `0.5`, `50%`)
- Additional options available via `overlock environment create --help`

**Example:**
```bash
overlock environment create my-dev-env
```

### `overlock environment list`

List all available environments.

```bash
overlock environment list
```

### `overlock environment start`

Start a stopped environment.

```bash
overlock environment start <name>
```

### `overlock environment stop`

Stop a running environment without deleting it.

```bash
overlock environment stop <name>
```

### `overlock environment upgrade`

Upgrade an environment to the latest Crossplane version.

```bash
overlock environment upgrade <name>
```

### `overlock environment delete`

Delete an environment and all its resources.

```bash
overlock environment delete <name>
```

### `overlock environment node create`

Add a remote node to a k3s-docker environment via SSH.

```bash
overlock environment node create <name> [options]
```

**Options:**
- `--env`: Target environment name
- `--host`: Remote host IP address
- `--scopes`: Node scopes (e.g., `engine`, `workloads`)
- `--user`: SSH user (default: `root`)
- `--port`: SSH port (default: `22`)
- `--key`: Path to SSH private key (default: `~/.ssh/id_rsa`)
- `--cpu`: CPU limit for the node container (e.g., `2`, `0.5`, `50%`)

**Example:**
```bash
overlock env node create my-node --env my-env --host 192.168.1.100 --scopes engine
```

### `overlock environment node delete`

Remove a remote node from a k3s-docker environment.

```bash
overlock environment node delete <name> [options]
```

**Options:**
- `--env`: Target environment name
- `--host`: Remote host IP address

**Example:**
```bash
overlock env node delete my-node --env my-env --host 192.168.1.100
```

## Provider Management

Install and manage cloud providers (GCP, AWS, Azure, etc.).

### `overlock provider install`

Install a provider from a repository.

```bash
overlock provider install <provider-url>
```

**Example:**
```bash
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0
```

### `overlock provider list`

List all installed providers.

```bash
overlock provider list
```

### `overlock provider load`

Load a provider from a local file.

```bash
overlock provider load <name>
```

### `overlock provider serve`

Serve a provider for development with live reload support.

```bash
overlock provider serve <path> <main-path>
```

**Example:**
```bash
overlock provider serve ./my-provider ./cmd/provider
```

### `overlock provider delete`

Remove an installed provider.

```bash
overlock provider delete <provider-url>
```

## Configuration Management

Manage Crossplane configurations that define infrastructure patterns.

### `overlock configuration apply`

Apply a configuration from URL.

```bash
overlock configuration apply <url>
```

**Multiple configurations:**
```bash
overlock configuration apply <url1>,<url2>,<url3>
```

**Example:**
```bash
overlock configuration apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
```

### `overlock configuration list`

List all applied configurations.

```bash
overlock configuration list
```

### `overlock configuration load`

Load a configuration from a local file.

```bash
overlock configuration load <name>
```

### `overlock configuration serve`

Serve a configuration for development with live reload support.

```bash
overlock configuration serve <path>
```

**Example:**
```bash
overlock configuration serve ./my-config-package
```

### `overlock configuration delete`

Delete a configuration.

```bash
overlock configuration delete <url>
```

## Function Management

Manage Crossplane functions for custom composition logic.

### `overlock function apply`

Apply a function from URL.

```bash
overlock function apply <url>
```

**Multiple functions:**
```bash
overlock function apply <url1>,<url2>
```

### `overlock function list`

List all applied functions.

```bash
overlock function list
```

### `overlock function load`

Load a function from a local file.

```bash
overlock function load <name>
```

### `overlock function serve`

Serve a function for development with live reload support.

```bash
overlock function serve <path>
```

### `overlock function delete`

Delete a function.

```bash
overlock function delete <url>
```

## Registry Management

Configure package registries for storing and distributing Crossplane packages.

### `overlock registry create`

Create a local or remote registry connection.

**Local registry:**
```bash
overlock registry create --local --default
```

**Remote registry:**
```bash
overlock registry create --registry-server=<url> \
                        --username=<user> \
                        --password=<pass> \
                        --email=<email>
```

### `overlock registry list`

List all configured registries.

```bash
overlock registry list
```

### `overlock registry delete`

Delete a registry configuration.

```bash
overlock registry delete
```

## Resource Management

Create and manage custom resources.

### `overlock resource create`

Create a custom resource definition.

```bash
overlock resource create <type>
```

### `overlock resource list`

List all custom resources.

```bash
overlock resource list
```

### `overlock resource apply`

Apply resources from a file.

```bash
overlock resource apply <file.yaml>
```

## Command Aliases

All commands support short aliases for faster typing:

| Full Command | Alias |
|-------------|-------|
| `environment` | `env` |
| `configuration` | `cfg` |
| `provider` | `prv` |
| `function` | `fnc` |
| `registry` | `reg` |
| `resource` | `res` |

**Example:**
```bash
# These are equivalent
overlock environment list
overlock env list

# These are equivalent
overlock configuration apply <url>
overlock cfg apply <url>
```
