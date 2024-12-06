# Overlock

Overlock is a lightweight CLI tool designed to simplify the management of Crossplane resources and environments. It supports KinD, K3s, and K3d clusters, making it ideal for local development and testing of Crossplane configurations, providers and functions.

## Features

- **Easily manage Crossplane environments**: Create and manage Crossplane environments for local development or testing purposes.
- **Supports multiple cluster types**: Works with KinD, K3s, and K3d, allowing you to choose the cluster type best suited for your development and testing needs.
- **Local and remote package registries**: Manage both local and remote Crossplane registries to handle configurations, providers and functions.
- **Load and apply Crossplane configurations**: Seamlessly load Crossplane configuration packages from local `.xpkg` files or apply them directly from remote URLs.
- **Provider management**: Easily load and apply Crossplane Providers from `.xpkg` files, supporting rapid local development.
- **Function management**: Easily load and apply Crossplane Functions from `.xpkg` files.
- **Simplified interface**: Overlock automates Crossplane installation, ensuring the setup process is hassle-free and quick.

## Installation

To install Overlock, follow these steps:

### Prerequisites

- Docker installed and running.
- KinD, k3d or k3s installed.

### Install via pre-compiled binary

1. Visit the [GitHub Releases page](https://github.com/web-seven/overlock/releases) and download the appropriate `.tar.gz` archive for your system (e.g., `overlock-0.3.0-linux-amd64.tar.gz`).

2. Extract the downloaded archive:

   ```bash
   tar -xvzf overlock-0.3.0-linux-amd64.tar.gz
   ```

3. Move the extracted binary to a directory in your PATH (e.g., `/usr/local/bin`):

   ```bash
   sudo mv overlock /usr/local/bin/
   ```

4. Verify the installation:

   ```bash
   overlock --version
   ```

## Usage

Overlock simplifies Crossplane setup and management across different cluster types. Use the following commands to work with your environment:

- Create or delete a Crossplane environment:

  ```bash
  overlock environment create|delete <environment-name>
  ```

- Create or delete a local Crossplane package registry:

  ```bash
  overlock registry create|delete --local [--default]
  ```

- Create or delete a remote private Crossplane registry:

  ```bash
  overlock registry create|delete [--default] --registry-server=<httpsurl> --username=<string> --password=<string> --email=<string>
  ```

- Load a Crossplane configuration from a local `.xpkg` file:

  ```bash
  overlock configuration load <file.xpkg>
  ```

- Apply a Crossplane configuration from a remote URL:

  ```bash
  overlock configuration apply <url>
  ```

- Load and apply a Crossplane provider from an `.xpkg` file:

  ```bash
  overlock provider load --apply --path=<file.xpkg> provider-name:version
  ```

- Load and apply a Crossplane function from an `.xpkg` file:

  ```bash
  overlock function load --apply --path=<file.xpkg> function-name:version
  ```

Overlock will automatically set up Crossplane on the specified cluster type (KinD, K3s, or K3d) based on your configuration.

## Contributing

We welcome contributions! Please see the [CONTRIBUTING.md](CONTRIBUTING.md) for more details on how to get involved.

## Community

This project is written in Golang but many of the community contributions so far have been through blogging, speaking engagements, helping to test and drive the backlog of Overlock. If you'd like to help in any way then that would be more than welcome whatever your level of experience.

### Chat

[Join Discord here](https://discord.gg/rWdY2y57)

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

