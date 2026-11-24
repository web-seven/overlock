# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Building and Running
- **Build the project**: `go build -o overlock ./cmd/cli`
- **Run the application**: `go run ./cmd/cli [command]`
- **Install locally**: `go build -o $(go env GOPATH)/bin/overlock ./cmd/cli`

### Testing and Quality
- **Run tests**: `go test ./...`
- **Run tests with coverage**: `go test -coverprofile=coverage.out ./...`
- **View coverage**: `go tool cover -html=coverage.out`
- **Format code**: `go fmt ./...`
- **Lint code**: `go vet ./...`
- **Tidy modules**: `go mod tidy`

### Development Workflow
- **Run specific tests**: `go test ./path/to/package -run TestName`
- **Build for specific OS/arch**: `GOOS=linux GOARCH=amd64 go build -o overlock ./cmd/cli`

## Project Architecture

Overlock is a CLI tool for managing Crossplane environments and resources. The codebase follows Go project conventions with a clear separation of concerns:

### Core Architecture
- **CLI Framework**: Uses Kong for command-line parsing and subcommands
- **Kubernetes Integration**: Built on controller-runtime and client-go for K8s operations
- **Multi-Engine Support**: Supports KinD, K3s, and K3d cluster types
- **Package Management**: Handles Crossplane configurations, providers, and functions
- **Plugin System**: Extensible architecture for custom functionality

### Key Components
- **Engine Management** (`internal/engine/`): Crossplane installation and lifecycle via Helm
- **Environment Management** (`pkg/environment/`): Kubernetes cluster creation/management
- **Registry Operations** (`pkg/registry/`): Package registry management (local/remote)
- **Resource Management** (`internal/resources/`): Crossplane resource operations
- **Kubernetes Utilities** (`internal/kube/`): K8s client abstractions and helpers
- **Plugin System** (`pkg/plugin/`): Dynamic plugin loading and execution

### Command Structure
The CLI is organized into main command groups:
- `environment` (alias: `env`): Create/manage K8s environments
- `configuration` (alias: `cfg`): Manage Crossplane configurations
- `provider` (alias: `prv`): Manage Crossplane providers
- `function` (alias: `fnc`): Manage Crossplane functions
- `registry` (alias: `reg`): Manage package registries
- `resource` (alias: `res`): Resource operations

### Key Dependencies
- **Kubernetes**: Uses client-go, controller-runtime, and kubectl libraries
- **Crossplane**: Integrates with Crossplane APIs and package formats
- **Container Runtime**: Docker client for container management
- **Cloud SDKs**: Cosmos SDK for blockchain integration, Solana Go for Solana support
- **Helm**: For Crossplane installation and management
- **Terminal UI**: PTerm for rich terminal output and user interaction

### Configuration and State
- Default plugin path: `~/.config/overlock/plugins`
- Uses kubeconfig for cluster authentication
- Environment variables: `OVERLOCK_ENGINE_NAMESPACE`, `OVERLOCK_ENGINE_RELEASE`, `OVERLOCK_ENGINE_VERSION`
- Managed resources labeled with `app.kubernetes.io/managed-by: overlock`

### Plugin Architecture
The project supports a plugin system that allows extending functionality through dynamically loaded plugins stored in the configured plugin path.