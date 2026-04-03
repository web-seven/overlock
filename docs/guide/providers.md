# Providers

A [Crossplane provider](https://docs.crossplane.io/latest/concepts/providers/) is a package that teaches your cluster how to talk to an external system — a cloud platform, a database service, a DNS provider, or anything else that has an API. Once a provider is installed and configured, you can manage infrastructure on that system using Kubernetes-style YAML manifests.

For example, installing the AWS provider lets you describe an S3 bucket or an RDS database in a YAML file, and Crossplane will create and maintain that infrastructure for you. Installing the GitHub provider lets you manage repositories and teams the same way.

Providers are usually installed as dependencies of a [configuration](configurations.md) — but you can also install them directly when you need fine-grained control over versions, or when you're developing your own compositions.

> [!NOTE]
> This guide covers how Overlock manages the provider package lifecycle. After installing a provider you'll also need to create a `ProviderConfig` to supply credentials. That part is provider-specific — the [Crossplane provider documentation](https://docs.crossplane.io/latest/concepts/providers/#provider-configuration) explains the pattern, and each provider's own documentation covers the exact fields.

---

## Installing a Provider

Find the provider you need — the [Upbound marketplace](https://marketplace.upbound.io/providers) is a good starting point — and install it by package URL:

```bash
overlock prv install xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0
```

Overlock installs the provider and reports when it's healthy. Once it's ready, the provider's managed resource types are registered in your cluster.

> [!TIP]
> Providers take slightly longer to become healthy than configurations, because they start a controller process inside the cluster. If you're scripting environment setup, add `--wait` to ensure the provider is fully ready before proceeding to the next step.

After installation, check that the provider is running:

```bash
overlock prv list
```

You should see it listed with a `Healthy: True` status.

---

## Configuring Provider Credentials

Installing a provider is only half the picture. The provider needs credentials to actually communicate with the external system — an AWS access key, a GCP service account, a GitHub token, and so on.

You supply these by creating a `ProviderConfig` resource in your cluster. The exact structure is different for every provider, but the pattern is always:

1. Create a Kubernetes `Secret` containing the credentials
2. Create a `ProviderConfig` that references the secret

For example, for the Helm provider:

```yaml
apiVersion: helm.crossplane.io/v1beta1
kind: ProviderConfig
metadata:
  name: default
spec:
  credentials:
    source: InjectedIdentity
```

> [!NOTE]
> Refer to your provider's own documentation for the exact `ProviderConfig` schema. The [Crossplane providers page](https://docs.crossplane.io/latest/concepts/providers/#provider-configuration) explains the general pattern. Overlock doesn't manage `ProviderConfig` resources — you apply them with `kubectl apply` directly.

Once the `ProviderConfig` is in place and the provider's managed resources types are registered, you're ready to start [creating resources](resources.md).

---

## Removing a Provider

When you no longer need a provider:

```bash
overlock prv delete xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0
```

Pass the same package URL you used when installing.

> [!WARNING]
> Deleting a provider removes its controller and unregisters its managed resource types. Any managed resources of those types that exist in the cluster will no longer be reconciled. Delete or migrate those resources before removing the provider.

---

## Loading a Provider from a Local Archive

If you've built a provider locally and exported it as an OCI archive:

```bash
overlock prv load --path ./my-provider.tar --apply
```

To upgrade an already-installed provider:

```bash
overlock prv load --path ./my-provider.tar --apply --upgrade
```

---

## Developing a Provider with Live Reload

If you're building a provider from scratch — for example, using the [Crossplane provider template](https://github.com/crossplane/upjet) or writing a custom controller — Overlock's `serve` command gives you a hot-reload workflow that eliminates the manual build-push-install cycle.

### Step 1 — Set up a local registry

The `serve` command pushes rebuilt packages to a local registry. If you haven't set one up:

```bash
overlock reg create --local --default
```

### Step 2 — Start the live-reload server

Point `serve` at your provider's source directory:

```bash
overlock prv serve ./my-provider
```

Overlock watches for file changes, rebuilds the provider binary and package, pushes it to the local registry, and hot-reloads it in your cluster.

If your provider's main package isn't in the default `cmd/provider` location, specify the path explicitly:

```bash
overlock prv serve ./my-provider --main-path cmd/my-provider
```

> [!TIP]
> Provider builds involve compiling Go code, so reload cycles are slower than configuration reloads — typically 10–30 seconds depending on your machine. The workflow is still much faster than the manual alternative. Run your provider's tests separately with `go test ./...` for the tightest feedback loop during logic development.

### Step 3 — Test with managed resources

Once the provider is running, create a test managed resource to verify it's reconciling correctly:

```bash
kubectl apply -f my-test-resource.yaml
kubectl get managed -o wide
```

Watch the provider's controller logs for reconciliation output:

```bash
kubectl logs -n crossplane-system -l pkg.crossplane.io/revision -f
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
| `--path` | — | Path to the OCI archive file |
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

## Related Guides

- [Configurations](configurations.md) — configurations commonly depend on specific providers
- [Functions](functions.md) — install composition functions alongside providers for complete pipelines
- [Registries](registries.md) — set up a local registry for provider development
- [Resources](resources.md) — create managed resources once your provider is installed and configured
- [Getting Started](getting-started.md) — end-to-end walkthrough
