# Development Guide

This guide covers how to build, test, and contribute to Overlock.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Building from Source](#building-from-source)
- [Development Commands](#development-commands)
- [Project Architecture](#project-architecture)
- [Testing](#testing)
- [Contributing](#contributing)

## Prerequisites

To develop Overlock, you'll need:

- **Go 1.24.0 or later**: [Install Go](https://golang.org/doc/install)
- **Docker**: For building and testing
- **Git**: For version control
- **Make** (optional): For using Makefile commands
- **Kubernetes CLI tools**: kubectl, and optionally kind/k3s/k3d

## Building from Source

### Clone the Repository

```bash
git clone https://github.com/web-seven/overlock.git
cd overlock
```

### Build the Binary

```bash
# Build for your current platform
go build -o overlock ./cmd/overlock

# The binary will be created in the current directory
./overlock --version
```

### Install Locally

```bash
# Install to $GOPATH/bin (ensure it's in your PATH)
go install ./cmd/overlock

# Verify installation
overlock --version
```

### Build for Specific Platforms

```bash
# Build for Linux (AMD64)
GOOS=linux GOARCH=amd64 go build -o overlock-linux-amd64 ./cmd/overlock

# Build for macOS (ARM64)
GOOS=darwin GOARCH=arm64 go build -o overlock-darwin-arm64 ./cmd/overlock

# Build for Windows
GOOS=windows GOARCH=amd64 go build -o overlock-windows-amd64.exe ./cmd/overlock
```

## Development Commands

### Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...

# View coverage report
go tool cover -html=coverage.out

# Run tests for a specific package
go test ./pkg/environment

# Run specific test
go test ./pkg/environment -run TestCreate

# Run tests with verbose output
go test -v ./...

# Run tests with race detection
go test -race ./...
```

### Code Quality

```bash
# Format code
go fmt ./...

# Lint code
go vet ./...

# Tidy dependencies
go mod tidy

# Verify dependencies
go mod verify

# Check for vulnerabilities
go run golang.org/x/vuln/cmd/govulncheck ./...
```

### Running the Application

```bash
# Run without building
go run ./cmd/overlock [command]

# Example: Create environment
go run ./cmd/overlock environment create test-env

# Run with debug output
go run ./cmd/overlock --debug environment list
```

## Project Architecture

### Directory Structure

```
overlock/
├── cmd/
│   └── overlock/          # Main entry point
├── internal/              # Internal packages
│   ├── engine/           # Crossplane installation/management
│   ├── kube/             # Kubernetes utilities
│   └── resources/        # Resource management
├── pkg/                   # Public packages
│   ├── environment/      # Environment management
│   ├── plugin/           # Plugin system
│   └── registry/         # Registry operations
├── scripts/              # Build and installation scripts
└── docs/                 # Documentation
```

### Core Components

#### CLI Framework
- Uses [Kong](https://github.com/alecthomas/kong) for command-line parsing
- Command structure defined in `cmd/overlock/`
- Supports subcommands and flags

#### Engine Management
Location: `internal/engine/`

Handles Crossplane installation and lifecycle:
- Helm-based installation
- Version management
- Namespace configuration
- Health checking

#### Environment Management
Location: `pkg/environment/`

Manages Kubernetes clusters:
- Supports KinD, K3s, K3d
- Cluster creation and deletion
- Lifecycle management (start/stop)
- Environment listing

#### Registry Operations
Location: `pkg/registry/`

Package registry management:
- Local registry setup
- Remote registry authentication
- Package publishing support

#### Resource Management
Location: `internal/resources/`

Crossplane resource operations:
- Configuration application
- Provider installation
- Function management
- Custom resource handling

#### Plugin System
Location: `pkg/plugin/`

Extensibility through plugins:
- Dynamic plugin loading
- Plugin path configuration
- Plugin execution interface

### Key Dependencies

The project uses several important libraries:

- **Kubernetes**: client-go, controller-runtime, kubectl
- **Crossplane**: Crossplane APIs and runtime
- **Container**: Docker client for container management
- **Helm**: Helm SDK for Crossplane installation
- **CLI**: Kong for argument parsing, PTerm for terminal UI
- **Cloud**: Cosmos SDK, Solana Go for blockchain integration

### Configuration

Environment variables:
- `OVERLOCK_ENGINE_NAMESPACE`: Default namespace
- `OVERLOCK_ENGINE_RELEASE`: Helm release name
- `OVERLOCK_ENGINE_VERSION`: Crossplane version

Default plugin path: `~/.config/overlock/plugins`

Resource labels: `app.kubernetes.io/managed-by: overlock`

## Testing

### Unit Tests

```bash
# Run all unit tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/environment -v
```

### Integration Tests

```bash
# Integration tests require a Kubernetes cluster
# Create a test environment first
go run ./cmd/overlock environment create test-integration

# Run integration tests
go test ./test/integration -tags=integration

# Cleanup
go run ./cmd/overlock environment delete test-integration
```

### Manual Testing

```bash
# Build and test locally
go build -o overlock ./cmd/overlock

# Create test environment
./overlock environment create manual-test

# Test provider installation
./overlock provider install xpkg.upbound.io/crossplane-contrib/provider-kubernetes:v0.14.0

# Verify
./overlock provider list

# Cleanup
./overlock environment delete manual-test
```

### Testing Best Practices

1. **Write tests for new features**: All new code should include tests
2. **Test edge cases**: Consider error conditions and boundary cases
3. **Use table-driven tests**: For testing multiple scenarios
4. **Mock external dependencies**: Use interfaces and mocks for external services
5. **Keep tests fast**: Avoid slow integration tests in unit test suite

## Contributing

### Development Workflow

1. **Fork the repository** on GitHub

2. **Clone your fork:**
   ```bash
   git clone https://github.com/YOUR_USERNAME/overlock.git
   cd overlock
   ```

3. **Create a feature branch:**
   ```bash
   git checkout -b feature/my-new-feature
   ```

4. **Make your changes:**
   - Write code
   - Add tests
   - Update documentation

5. **Test your changes:**
   ```bash
   go test ./...
   go fmt ./...
   go vet ./...
   ```

6. **Commit your changes:**
   ```bash
   git add .
   git commit -m "Add my new feature"
   ```

7. **Push to your fork:**
   ```bash
   git push origin feature/my-new-feature
   ```

8. **Create a Pull Request** on GitHub

### Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Run `go vet` to catch common issues
- Keep functions focused and small
- Write clear comments for exported functions
- Use meaningful variable names

### Commit Messages

Follow conventional commit format:

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `refactor`: Code refactoring
- `test`: Test changes
- `chore`: Build/tooling changes

Example:
```
feat(environment): Add support for k3d cluster creation

Adds k3d as a supported Kubernetes engine option alongside
KinD and K3s. Includes configuration options for k3d-specific
features.

Closes #123
```

### Pull Request Guidelines

- Keep PRs focused on a single feature or fix
- Update documentation for new features
- Add tests for bug fixes
- Ensure all tests pass
- Follow the code style
- Provide a clear description of changes

### Getting Help

- Join our [Discord](https://discord.gg/W7AsrUb5GC)
- Check [existing issues](https://github.com/web-seven/overlock/issues)
- Review [CONTRIBUTING.md](../CONTRIBUTING.md) for detailed guidelines

## Development Tools

### Recommended Editor Setup

#### VS Code
Install these extensions:
- Go (golang.go)
- YAML (redhat.vscode-yaml)
- Kubernetes (ms-kubernetes-tools.vscode-kubernetes-tools)

#### GoLand
Built-in Go support with excellent refactoring tools.

### Debugging

#### Using Delve
```bash
# Install delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug the application
dlv debug ./cmd/overlock -- environment create test

# Set breakpoints in code
# Use debugger commands (break, continue, next, etc.)
```

#### Using Print Debugging
```bash
# Run with debug flag
go run ./cmd/overlock --debug environment create test
```

### Performance Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof ./...
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof ./...
go tool pprof mem.prof

# Benchmark tests
go test -bench=. -benchmem ./...
```

## Release Process

Releases are typically handled by maintainers:

1. Update version in code
2. Update CHANGELOG.md
3. Create and push git tag
4. GitHub Actions builds and publishes release
5. Update installation script

## Additional Resources

- [Command Reference](commands.md)
- [Configuration Guide](configuration.md)
- [Examples](examples.md)
- [Troubleshooting](troubleshooting.md)
- [Crossplane Documentation](https://docs.crossplane.io/)
- [Go Documentation](https://golang.org/doc/)
