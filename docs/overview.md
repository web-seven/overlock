# Overview

This document provides a detailed overview of Overlock's key features.

## Table of Contents

- [Quick Environment Setup](#quick-environment-setup)
- [Multi-Engine Support](#multi-engine-support)
- [Multi-Node & Remote Nodes (k3s-docker)](#multi-node--remote-nodes-k3s-docker)
- [CPU Limits](#cpu-limits)
- [Package Management](#package-management)
- [Live Development](#live-development)
- [Registry Integration](#registry-integration)
- [Plugin System](#plugin-system)

## Quick Environment Setup

Create fully configured Crossplane environments with a single command. Overlock handles cluster provisioning, Crossplane installation, and initial configuration automatically.

```bash
# Create a new environment with default settings
overlock environment create my-dev-env

# Create with a specific Crossplane version
overlock --engine-version 1.18.0 environment create my-dev-env
```

See the [Command Reference](commands.md#environment-management) for all environment options.

## Multi-Engine Support

Overlock works seamlessly with multiple Kubernetes distributions, so you can choose the engine that best fits your workflow:

| Engine | Description | Best for |
|--------|-------------|----------|
| **KinD** | Kubernetes in Docker | Quick local testing |
| **K3s** | Lightweight Kubernetes | Low-resource environments |
| **K3d** | K3s in Docker | Fast multi-cluster setups |
| **K3s-Docker** | K3s with Docker containers as nodes | Distributed and multi-node environments |

```bash
overlock env create my-env --engine kind
overlock env create my-env --engine k3s
overlock env create my-env --engine k3d
overlock env create my-env --engine k3s-docker
```

## Multi-Node & Remote Nodes (k3s-docker)

The `k3s-docker` engine supports multi-node clusters with dedicated node scoping and remote node management via SSH.

```bash
# Create environment with k3s-docker engine (includes workloads + engine nodes)
overlock env create my-env --engine k3s-docker

# Add a remote machine as an engine node (Crossplane, providers, functions, Kyverno, CertManager)
overlock env node create my-remote-node --env my-env --host 192.168.1.100 --scopes engine

# Remove a remote node
overlock env node delete my-remote-node --env my-env --host 192.168.1.100

# Delete environment (automatically cleans up all local and remote nodes)
overlock env delete my-env --engine k3s-docker
```

**How it works:**
- The k3s-docker engine creates an agentless K3s server with two default agent nodes: **workloads** (for user workloads and system services) and **engine** (dedicated to Crossplane, providers, functions, Kyverno, CertManager)
- Remote nodes join the cluster via SSH — any Linux host with Docker installed can be added as a worker
- Node scoping uses Kubernetes labels and taints to isolate engine components from user workloads
- Inter-node traffic is encrypted via **WireGuard** out of the box
- On environment deletion, remote node containers are automatically discovered and cleaned up

## CPU Limits

Complex Crossplane control planes with multiple providers, functions, and configurations can consume significant CPU on a development machine. The `--cpu` flag lets you cap CPU usage per container node, keeping your machine responsive while running heavy reconciliation loops.

```bash
# Limit each container to 2 CPU cores
overlock env create my-env --engine k3s-docker --cpu 2

# Fractional and percentage values are also supported
overlock env create my-env --engine k3s-docker --cpu 0.5
overlock env create my-env --engine k3s-docker --cpu 50%

# Apply CPU limit to a specific remote node
overlock env node create my-node --env my-env --host 192.168.1.100 --scopes engine --cpu 4
```

## Package Management

Install and manage Crossplane configurations, providers, and functions from remote registries or local packages.

```bash
# Install a provider
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0

# Apply a configuration
overlock configuration apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31

# Apply a function
overlock function apply xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0

# List installed packages
overlock provider list
overlock configuration list
overlock function list
```

See the [Command Reference](commands.md#provider-management) for all package operations.

## Live Development

Hot-reload support for local package development. Serve a configuration, provider, or function from your local filesystem and Overlock watches for changes, automatically rebuilding and reloading the package in your environment.

```bash
# Serve a configuration for live development
overlock configuration serve ./my-config-package

# Serve a provider with live reload
overlock provider serve ./my-provider ./cmd/provider

# Serve a function
overlock function serve ./my-function
```

This enables a fast feedback loop — edit your Crossplane package code locally, and see changes reflected in the cluster within seconds.

## Registry Integration

Support for both local and remote package registries, enabling flexible package distribution workflows.

```bash
# Create a local registry for development
overlock registry create --local --default

# Connect a remote registry
overlock registry create --registry-server=registry.example.com \
  --username=myuser --password=mypass --email=user@example.com

# List configured registries
overlock registry list
```

Local registries are useful for development and CI/CD pipelines where you want to test packages before publishing to a remote registry.

See the [Registry Authentication](registry/auth.md) guide for private registry setup.

## Plugin System

Extensible architecture for custom functionality. Plugins are standalone executables placed in the plugin directory that extend Overlock with new commands and capabilities.

```bash
# Use custom plugin path
overlock --plugin-path /path/to/plugins <command>
```

Default plugin path: `~/.config/overlock/plugins`

See the [Configuration Guide](configuration.md#plugin-configuration) for plugin setup details.
