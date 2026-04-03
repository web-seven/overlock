# Resources

In Crossplane, a [composite resource](https://docs.crossplane.io/latest/concepts/composite-resources/) is an instance of a custom type that was defined by a configuration you've installed. Resources are the actual things you create and manage: a database, a Kubernetes cluster, a DNS record, an application deployment — whatever the configuration defines. They're the end product of the entire composition pipeline.

Once you've installed a configuration and its provider dependencies, creating a resource is the step where all that setup pays off.

> [!NOTE]
> If you haven't installed a configuration yet, start with the [Configurations guide](configurations.md). Resources can only be created once the XRDs that define their types are registered in your cluster.

---

## Step 1 — Discover What's Available

After installing a configuration, new resource types are registered in your cluster. The first thing to do is discover what you can now create:

```bash
overlock res list
```

This lists the Composite Resource Definitions (XRDs) and claims that are available. You'll see types like `AppClaim`, `Database`, `KubernetesCluster` — whatever the installed configurations define.

> [!TIP]
> You can also use `kubectl get xrds` for a more detailed view, or `kubectl get crds | grep crossplane` to see all the CRDs that have been registered. The `overlock res list` output is filtered to show only the composite types relevant to you as a user — it omits the internal plumbing.

---

## Step 2 — Create a Resource

### From a YAML file

The most common way to create a resource is to write a YAML manifest and apply it. If you know the API group and kind of the resource you want (from `overlock res list`), write a claim:

```yaml
apiVersion: example.org/v1alpha1
kind: Database
metadata:
  name: my-database
  namespace: default
spec:
  parameters:
    size: small
    region: eu-west-1
  compositeDeletePolicy: Foreground
  writeConnectionSecretToRef:
    name: my-database-connection
```

Apply it:

```bash
overlock res apply -f my-database.yaml
```

Crossplane picks up the claim, creates the underlying composite resource, and starts the composition pipeline. Depending on the provider, actual infrastructure provisioning may take anywhere from a few seconds to several minutes.

> [!NOTE]
> The `compositeDeletePolicy: Foreground` field tells Crossplane to wait for all composed resources to be deleted before deleting the composite. This is the safest option for development — it prevents orphaned cloud resources if you delete a claim before the provider has finished cleaning up.

### Interactively

If you're not sure what fields a resource type requires, use the interactive create command:

```bash
overlock res create --type Database
```

Overlock walks you through the required fields and creates the resource for you.

---

## Step 3 — Observe Resource Status

After creating a resource, you want to know whether it's healthy and what it's doing. Crossplane tracks the status of both the composite resource and each managed resource it composed.

Check the top-level claim status:

```bash
kubectl get database my-database -o wide
```

Look at the `READY` and `SYNCED` columns. `True/True` means everything is healthy. If you see `False` in either column, dig deeper:

```bash
kubectl describe database my-database
```

The `Events` section shows what Crossplane has done and any errors it encountered.

> [!TIP]
> The `SYNCED` column reflects whether Crossplane was able to reconcile the resource on its last attempt. `READY` reflects whether the underlying infrastructure is actually up and functioning. A resource can be `SYNCED: True` but `READY: False` while infrastructure is still provisioning.

To see the managed resources that the composition created under the hood:

```bash
kubectl get managed
```

This shows all provider-managed resources across all composite resources. Each row has its own `READY` and `SYNCED` status — useful for pinpointing which specific resource in a complex composition is having trouble.

> [!NOTE]
> Connection details (passwords, endpoints, certificates) are written to the Kubernetes Secret named in `writeConnectionSecretToRef`. Use `kubectl get secret my-database-connection -o yaml` to retrieve them once the resource is ready.

---

## Step 4 — Update a Resource

To change a resource's parameters, edit your YAML file and re-apply it:

```bash
overlock res apply -f my-database.yaml
```

Crossplane will reconcile the difference and update the underlying infrastructure accordingly. Not all fields support updates — if a managed resource's field is immutable (a common constraint in cloud provider APIs), the provider will report an error.

---

## Step 5 — Delete a Resource

When you're done with a resource, delete the claim:

```bash
kubectl delete database my-database
```

Crossplane will cascade-delete the composite resource and all managed resources it composed. Depending on `compositeDeletePolicy`, this may be foreground (waits for cleanup) or background (immediate deletion from Kubernetes, cleanup happens asynchronously).

> [!WARNING]
> Deleting a claim deletes real infrastructure. In a production environment, make sure you have the right `deletionPolicy` set on your managed resources. During development, `compositeDeletePolicy: Foreground` is recommended to avoid orphaned resources in your cloud account.

---

## Command Reference

### `overlock res list`

Lists composite resource types available in the active environment. No flags.

### `overlock res create`

Creates a new resource interactively.

| Flag | Default | Description |
|------|---------|-------------|
| `--type` | *(required)* | The resource type to create (e.g. `Database`, `AppClaim`) |

### `overlock res apply`

Applies a resource manifest from a file.

| Flag | Default | Description |
|------|---------|-------------|
| `--file` / `-f` | *(required)* | Path to the YAML manifest file |

---

## Related Guides

- [Configurations](configurations.md) — install configurations that define the resource types you're creating
- [Providers](providers.md) — providers supply the managed resources that compose into your composite resources
- [Functions](functions.md) — functions run the composition logic that turns your claim into managed resources
- [Getting Started](getting-started.md) — end-to-end walkthrough
