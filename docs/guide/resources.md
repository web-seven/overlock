# Resources

## What is it

In Crossplane, a resource is an instance of a composite resource type — a custom kind that was defined by a configuration you've installed. Resources are the actual things you create and manage: a database, a Kubernetes cluster, a DNS record, whatever the configuration defines.

When you install a configuration, it registers new resource types in your cluster. You then create instances of those types as resources.

---

## When would I use it

You've installed a configuration that adds a `Database` or `AppClaim` type to your cluster, and now you want to actually create one. You write a YAML file describing what you want and apply it, or you let Overlock walk you through the creation interactively.

---

## How to use it

### List available resource types

After installing a configuration, you can see what resource types are now available:

```bash
overlock res list
```

This shows the composite resource types (XRs and claims) registered in your cluster.

### Create a resource interactively

```bash
overlock res create --type MyDatabaseClaim
```

Overlock prompts you for the required fields and creates the resource.

### Apply a resource from a file

If you already have a YAML manifest:

```bash
overlock res apply -f my-database.yaml
```

This applies the resource to your active cluster, the same way `kubectl apply` would, but with awareness of Crossplane resource types.

**Example YAML:**

```yaml
apiVersion: example.com/v1alpha1
kind: AppClaim
metadata:
  name: my-app
  namespace: default
spec:
  parameters:
    size: small
  compositeDeletePolicy: Foreground
  writeConnectionSecretToRef:
    name: my-app-connection
```

---

## Command Reference

### `overlock res list`

Lists composite resource types available in the active environment. No flags.

### `overlock res create`

Creates a new resource interactively.

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | *(required)* | The resource type to create (e.g. `AppClaim`) |

### `overlock res apply`

Applies a resource manifest from a file.

| Flag | Default | Description |
|------|---------|-------------|
| `--file` / `-f` | *(required)* | Path to the YAML manifest file |

---

## Related guides

- [Configurations](configurations.md) — install configurations that define resource types
- [Providers](providers.md) — providers handle the actual infrastructure provisioning
- [Getting Started](getting-started.md) — end-to-end walkthrough
