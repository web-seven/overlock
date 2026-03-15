[![Discord](https://img.shields.io/badge/discord-join-7289DA.svg?logo=discord&longCache=true&style=flat)](https://discord.gg/W7AsrUb5GC)
[![Go Version](https://img.shields.io/badge/Go-1.24.0+-00ADD8?logo=go)](https://golang.org/doc/install)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![GitHub Release](https://img.shields.io/github/v/release/overlock-network/overlock)](https://github.com/overlock-network/overlock/releases)

<p align="center">
  <img width="170" src="https://raw.githubusercontent.com/overlock-network/overlock/refs/heads/main/docs/overlock_white_alpha.png"/>
</p>

# Overlock

**Simplify Crossplane development and testing with a powerful CLI toolkit.**

Overlock handles the complexity of setting up Crossplane environments, making it easy for developers to build, test, and deploy infrastructure-as-code solutions. Get a fully configured Crossplane environment running in minutes, not hours.

## Table of Contents

- [Features](#features)
- [Quick Start](#quick-start)
- [Installation](#installation)
- [Documentation](#documentation)
- [Architecture Overview](#architecture-overview)
- [Ecosystem Comparison](#ecosystem-comparison)
- [Community](#community)
- [Contributing](#contributing)
- [License](#license)

## Features

- **⚡ Quick Environment Setup** - Create fully configured Crossplane environments with a single command
- **🎯 Multi-Engine Support** - Works seamlessly with KinD, K3s, K3d, and K3s-Docker Kubernetes distributions
- **🖥️ Multi-Node & Remote Nodes** - Add remote Linux machines as worker nodes via SSH to distribute Crossplane workloads across multiple hosts
- **📦 Package Management** - Install and manage Crossplane configurations, providers, and functions
- **🔄 Live Development** - Hot-reload support for local package development
- **🏗️ Registry Integration** - Support for both local and remote package registries
- **🔌 Plugin System** - Extensible architecture for custom functionality

## Quick Start

```bash
# Create a new Crossplane environment
overlock environment create my-dev-env

# Install a cloud provider (GCP example)
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0

# Apply a configuration
overlock configuration apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31

# List your environments
overlock environment list
```

That's it! You now have a fully functional Crossplane environment ready for development.

### Multi-Node & Remote Nodes (k3s-docker)

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
- On environment deletion, remote node containers are automatically discovered and cleaned up

## Installation

### Prerequisites

- **Docker** (required for creating Kubernetes clusters)
- One of: **KinD**, **K3s**, or **K3d** (choose based on your preference)

### Install Overlock

**Latest version:**
```bash
curl -sL "https://raw.githubusercontent.com/overlock-network/overlock/refs/heads/main/scripts/install.sh" | sh
sudo mv overlock /usr/local/bin/
```

**Specific version:**
```bash
curl -sL "https://raw.githubusercontent.com/overlock-network/overlock/refs/heads/main/scripts/install.sh" | sh -s -- -v 0.11.0-beta.11
sudo mv overlock /usr/local/bin/
```

**Verify installation:**
```bash
overlock --version
```

### Building from Source

```bash
git clone https://github.com/overlock-network/overlock.git
cd overlock
go build -o overlock ./cmd/overlock
```

See the [Development Guide](docs/development.md) for detailed build instructions.

## Documentation

### User Guides

- **[Command Reference](docs/commands.md)** - Complete CLI command documentation
- **[Configuration Guide](docs/configuration.md)** - Environment variables and configuration options
- **[Usage Examples](docs/examples.md)** - Common workflows and practical examples
- **[Troubleshooting](docs/troubleshooting.md)** - Solutions to common issues

### Developer Resources

- **[Development Guide](docs/development.md)** - Building from source, testing, and contributing
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Contribution guidelines and code of conduct

### Command Overview

Overlock organizes functionality into intuitive command groups:

| Command | Alias | Description |
|---------|-------|-------------|
| `environment` | `env` | Create and manage Kubernetes environments |
| `provider` | `prv` | Install and manage cloud providers |
| `configuration` | `cfg` | Manage Crossplane configurations |
| `function` | `fnc` | Manage Crossplane functions |
| `registry` | `reg` | Configure package registries |
| `resource` | `res` | Create and manage custom resources |

Use `overlock <command> --help` for detailed information on any command.

## Architecture Overview

Overlock is built with a modular architecture designed for extensibility and maintainability:

```
┌─────────────────────────────────────────────────────┐
│                  Overlock CLI                       │
├─────────────────────────────────────────────────────┤
│  Environment Manager  │  Package Manager            │
│  - KinD               │  - Configurations           │
│  - K3s / K3d          │  - Providers                │
│  - K3s-Docker         │  - Functions                │
├─────────────────────────────────────────────────────┤
│  Engine Manager       │  Registry Manager           │
│  - Crossplane Install │  - Local Registries         │
│  - Helm Integration   │  - Remote Registries        │
├─────────────────────────────────────────────────────┤
│  Resource Manager     │  Plugin System              │
│  - Custom Resources   │  - Dynamic Loading          │
│  - YAML Processing    │  - Extensibility            │
└─────────────────────────────────────────────────────┘
                          │
                          ▼
        ┌─────────────────────────────────────┐
        │         Kubernetes Cluster          │
        │  (KinD / K3s / K3d / K3s-Docker)    │
        │         + Crossplane                │
        └─────────────────────────────────────┘
```

### Key Components

- **CLI Framework**: Kong-based command parsing with intuitive subcommands
- **Engine Management**: Helm-based Crossplane installation and lifecycle
- **Environment Management**: Multi-engine Kubernetes cluster operations
- **Package Management**: Crossplane configurations, providers, and functions
- **Plugin System**: Extensible architecture for custom functionality

For detailed architecture information, see the [Development Guide](docs/development.md).

## Ecosystem Comparison

| Feature | Overlock | kubectl + helm | Crossplane CLI | up CLI |
|---------|----------|----------------|----------------|--------|
| Environment creation | ✅ Single command | ❌ Manual setup | ❌ Manual setup | ✅ Automated |
| Multi-engine support | ✅ KinD/K3s/K3d/K3s-Docker | ✅ Any K8s | ✅ Any K8s | ⚠️ Limited |
| Hybrid environments | ✅ Local + remote nodes via SSH | ❌ Manual | ❌ No | ❌ No |
| Package management | ✅ Built-in | ❌ Manual | ✅ Limited | ✅ Built-in |
| Live reload dev | ✅ Yes | ❌ No | ❌ No | ⚠️ Partial |
| Registry support | ✅ Local + Remote | ❌ Manual | ⚠️ Remote only | ✅ Yes |
| Environment lifecycle | ✅ Full control | ❌ Manual | ❌ Manual | ⚠️ Limited |
| Plugin system | ✅ Yes | N/A | ❌ No | ❌ No |

**Why Overlock?**

Overlock bridges the gap between simple kubectl/helm workflows and full-featured cloud platforms. It provides:
- Faster setup than manual kubectl/helm configurations
- More development-focused features than standard Crossplane CLI
- Better local development experience than cloud-based solutions
- Complete control over your development environment

## Community

### Get Help & Connect

- **💬 Discord**: [Join our Discord](https://discord.gg/W7AsrUb5GC) for questions and community support
- **🐛 Issues**: [Report bugs or request features](https://github.com/overlock-network/overlock/issues)
- **📖 Discussions**: [Join discussions](https://github.com/overlock-network/overlock/discussions)

### Contributing

We welcome contributions from the community! Whether you're fixing bugs, adding features, or improving documentation, your help is appreciated.

- Read our [Contributing Guide](CONTRIBUTING.md) to get started
- Check out [Good First Issues](https://github.com/overlock-network/overlock/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
- Review the [Development Guide](docs/development.md) for technical details

### Code of Conduct

We are committed to providing a welcoming and inclusive experience. Please read our Code of Conduct in [CONTRIBUTING.md](CONTRIBUTING.md).

## Credits

Overlock is built on top of excellent open-source projects:
- [Crossplane](https://crossplane.io/) - The cloud native control plane framework
- [Kubernetes](https://kubernetes.io/) - Container orchestration platform
- [Helm](https://helm.sh/) - The Kubernetes package manager
- [KinD](https://kind.sigs.k8s.io/), [K3s](https://k3s.io/), [K3d](https://k3d.io/) - Kubernetes engines

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made with ❤️ by the Overlock community
</p>
