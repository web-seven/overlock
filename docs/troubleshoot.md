# Troubleshooting

This document provides solutions to common issues you may encounter when using Overlock.

## Table of Contents

- [Common Issues](#common-issues)
  - [Environment Creation Fails](#environment-creation-fails)
  - [Package Installation Fails](#package-installation-fails)
  - [Provider Not Working](#provider-not-working)
  - [Freezing During Environment Creation](#freezing-during-environment-creation)
- [Firewall Configuration for Remote Nodes](#firewall-configuration-for-remote-nodes)
- [Getting Help](#getting-help)
- [Debug Mode](#debug-mode)

## Common Issues

### Environment Creation Fails

**Symptoms:**
- Command fails with cluster creation errors
- Timeout during environment setup
- Docker-related errors

**Solutions:**

1. **Ensure Docker is running:**
   ```bash
   docker ps
   ```
   If this fails, start Docker daemon.

2. **Check Kubernetes engine installation:**
   - For KinD: `kind version`
   - For K3s: `k3s --version`
   - For K3d: `k3d version`

3. **Verify system resources:**
   - Check available memory: `free -h`
   - Check available disk space: `df -h`
   - Ensure at least 4GB RAM and 10GB disk space available

4. **Check for port conflicts:**
   ```bash
   # Check if required ports are in use
   sudo lsof -i :6443  # Kubernetes API
   sudo lsof -i :5000  # Local registry
   ```

5. **Clean up existing environments:**
   ```bash
   overlock environment list
   overlock environment delete <old-env-name>
   ```

### Package Installation Fails

**Symptoms:**
- Configuration, provider, or function fails to install
- Timeout errors
- Authentication errors

**Solutions:**

1. **Check internet connectivity:**
   ```bash
   curl -I https://xpkg.upbound.io
   ```

2. **Verify package URL is correct:**
   - Check for typos in package name
   - Verify version exists
   - Try accessing URL in browser

3. **Use debug mode for details:**
   ```bash
   overlock --debug provider install <url>
   ```

4. **Check authentication for private registries:**
   - Verify registry credentials
   - Ensure you're logged into the registry

5. **Verify Crossplane is ready:**
   ```bash
   kubectl get pods -n crossplane-system
   ```
   All pods should be in `Running` state.

### Provider Not Working

**Symptoms:**
- Provider installed but resources not working
- Authentication errors in provider logs
- Resources stuck in non-ready state

**Solutions:**

1. **Verify provider is installed and healthy:**
   ```bash
   overlock provider list
   kubectl get providers
   ```

2. **Check provider logs:**
   ```bash
   kubectl logs -n crossplane-system deployment/<provider-name>
   ```

3. **Verify provider configuration:**
   - For GCP: Check service account key configuration
   - For AWS: Verify AWS credentials
   - For Azure: Check Azure credentials

4. **Check Crossplane version compatibility:**
   - Some providers require specific Crossplane versions
   - Check provider documentation for compatibility matrix

5. **Verify ProviderConfig exists:**
   ```bash
   kubectl get providerconfigs
   ```

### Freezing During Environment Creation

#### Symptom

The process freezes for a few minutes during the "Joining worker nodes" step when creating multiple environments with Overlock CLI. Eventually, it fails with the following error:
```
ERROR: failed to create cluster: failed to join node with kubeadm: command "docker exec --privileged dest-worker kubeadm join --config /kind/kubeadm.conf --skip-phases=preflight --v=6" failed with error: exit status 1
```

#### Symptom
The process freezes for a few minutes during the "Joining worker nodes" step when creating multiple environments with Overlock CLI. Eventually, it fails with the following error:
`ERROR: failed to create cluster: failed to join node with kubeadm: command "docker exec --privileged dest-worker kubeadm join --config /kind/kubeadm.conf --skip-phases=preflight --v=6" failed with error: exit status 1`


#### Cause

When the Overlock CLI creates environments, it also installs resources, likely increasing the number of file system watches (inotify instances) that Kubernetes and its components need to manage. This increased usage, combined with existing watches from previous Overlock environments, could exceed the default system limits, leading to the kubelet.service on the newly created worker node failing to start due to the error: `Failed to allocate directory watch: Too many open files.`

#### Steps to Resolve

1. Run the following command to adjust the `fs.inotify.max_user_instances` setting on your host:
   ```bash
   sysctl fs.inotify.max_user_instances=512
   ```

2. Retry the environment creation command:
   ```bash
   overlock env create <name>
   ```

#### Explanation

**Why did the error occur?**

The error indicates that the kubelet.service failed to start due to the system reaching its limit for the number of file system watches (`inotify` instances) allowed per user.

**How did adjusting `fs.inotify.max_user_instances` solve the error?**

Increasing the `fs.inotify.max_user_instances` setting allows more `inotify` instances to be allocated per user, resolving the resource limitation that caused the kubelet.service to fail.

## Firewall Configuration for Remote Nodes

When using `k3s-docker` engine with remote nodes via SSH, the server host firewall must allow K3s traffic. If using `firewalld`:

### Open required ports

```bash
sudo firewall-cmd --zone=public --add-port=6443/tcp --permanent   # K3s API server
sudo firewall-cmd --zone=public --add-port=6444/tcp --permanent   # K3s supervisor
sudo firewall-cmd --zone=public --add-port=8472/udp --permanent   # Flannel VXLAN overlay
sudo firewall-cmd --zone=public --add-port=10250/tcp --permanent  # Kubelet
```

### Trust K3s interfaces

```bash
sudo firewall-cmd --zone=trusted --add-interface=cni0 --permanent
sudo firewall-cmd --zone=trusted --add-interface=flannel.1 --permanent
```

### Apply changes

```bash
sudo firewall-cmd --reload
```

## Getting Help

### Command Help

Use the `--help` flag to get detailed information about any command:

```bash
# General help
overlock --help

# Command-specific help
overlock environment --help
overlock configuration --help
overlock provider --help
```

### Debug Mode

Enable debug mode to see detailed output:

```bash
overlock --debug <command>
```

This will show:
- API calls being made
- Detailed error messages
- Internal operation logs
- Kubernetes resource operations

### Community Support

If you're still experiencing issues:

1. **Check existing issues**: Search [GitHub Issues](https://github.com/overlock-network/overlock/issues)
2. **Join Discord**: Get help from the community on [Discord](https://discord.gg/W7AsrUb5GC)
3. **Create an issue**: Report bugs or request features on [GitHub](https://github.com/overlock-network/overlock/issues/new)

### Providing Debug Information

When reporting issues, include:

1. **Overlock version:**
   ```bash
   overlock --version
   ```

2. **Debug output:**
   ```bash
   overlock --debug <failing-command> 2>&1 | tee debug.log
   ```

3. **System information:**
   - Operating system and version
   - Docker version: `docker version`
   - Kubernetes engine and version
   - Available resources (memory, disk)

4. **Kubernetes state:**
   ```bash
   kubectl get pods -A
   kubectl get providers
   kubectl get configurations
   ```

## Additional Resources

- [Configuration Guide](configuration.md) - Detailed configuration options
- [Command Reference](commands.md) - Complete command documentation
- [Examples](examples.md) - Common usage patterns and workflows
