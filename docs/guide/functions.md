# Functions

A [Crossplane composition function](https://docs.crossplane.io/latest/concepts/composition-functions/) is a piece of logic that runs as part of a composition pipeline. When Crossplane needs to reconcile a composite resource, it calls each function in the pipeline in order. Each function receives the current desired state, applies some logic, and passes an updated desired state to the next function.

Functions are what make compositions powerful beyond simple field copying. They let you template resources, apply conditional logic, loop over lists, call external APIs, and much more.

> [!NOTE]
> Functions run as containers inside your cluster. Crossplane calls them via gRPC when reconciling composite resources. You don't need to understand the gRPC protocol to use or develop functions — Overlock and the Crossplane SDKs handle that. See the [Crossplane composition functions documentation](https://docs.crossplane.io/latest/concepts/composition-functions/) for the conceptual overview.

---

## How Functions Fit Into the Development Flow

When you install a [configuration](configurations.md) that uses functions, Crossplane installs the required functions automatically as package dependencies. Most of the time you don't need to install functions directly.

The cases where you'd manage functions explicitly are:

- **Pinning a specific version** of a community function for reproducibility
- **Installing a function before applying a configuration** that depends on it, in environments with restricted network access
- **Developing your own function** and wanting to test it with live reload in a local environment

---

## Common Community Functions

The Crossplane community maintains several widely-used functions that you'll encounter in most real configurations:

| Function | Purpose |
|----------|---------|
| `function-patch-and-transform` | Field patching, transforms, and conditional logic — the most widely used function |
| `function-go-templating` | Full Go template rendering for composed resources |
| `function-kcl` | Use the KCL language for composition logic |
| `function-cue` | Use CUE for composition logic |
| `function-auto-ready` | Automatically mark composite resources as ready when all composed resources are ready |

Browse the full list on the [Upbound marketplace](https://marketplace.upbound.io/functions).

---

## Installing a Function

Install a function by its package URL:

```bash
overlock fnc apply xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0
```

Overlock installs the function and waits until it's healthy before returning.

> [!TIP]
> If you're setting up a fresh environment to develop or test a composition, install the required functions before applying the configuration. This avoids the configuration entering a degraded state while waiting for its dependencies.

Check that the function is running:

```bash
overlock fnc list
```

---

## Using Functions in a Composition

Once a function is installed, you reference it by name in your composition's pipeline:

```yaml
spec:
  mode: Pipeline
  pipeline:
    - step: patch-and-transform
      functionRef:
        name: function-patch-and-transform
      input:
        apiVersion: pt.fn.crossplane.io/v1beta1
        kind: Resources
        resources:
          - name: my-bucket
            base:
              apiVersion: s3.aws.upbound.io/v1beta1
              kind: Bucket
            patches:
              - type: FromCompositeFieldPath
                fromFieldPath: spec.parameters.region
                toFieldPath: spec.forProvider.region
```

> [!NOTE]
> The `functionRef.name` must match the metadata name of the installed Function object in your cluster — not the full package URL. Run `kubectl get functions` to see the exact names.

---

## Removing a Function

When you no longer need a function:

```bash
overlock fnc delete xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0
```

> [!WARNING]
> Removing a function that is referenced in an active composition will break reconciliation for any composite resources that use that composition. Make sure no active compositions reference the function before deleting it.

---

## Loading a Function from a Local Archive

If you've built a function and exported it as an OCI archive:

```bash
overlock fnc load --path ./my-function.tar --apply
```

In CI pipelines where the archive is piped rather than written to disk:

```bash
cat my-function.tar | overlock fnc load --stdin --apply
```

---

## Developing a Function with Live Reload

When you're building a custom function — for example, using the [Go composition functions SDK](https://docs.crossplane.io/latest/guides/write-a-composition-function-in-go/) or the [Python SDK](https://docs.crossplane.io/latest/guides/write-a-composition-function-in-python/) — Overlock's `serve` command gives you a hot-reload loop.

### Step 1 — Set up a local registry

```bash
overlock reg create --local --default
```

### Step 2 — Start the live-reload server

```bash
overlock fnc serve ./my-function
```

Overlock watches your source directory for changes, rebuilds the function container, pushes it to the local registry, and reloads the Function object in your cluster.

> [!TIP]
> Keep `serve` running in a dedicated terminal. In a second terminal, create or update composite resources to trigger reconciliation and verify your function's logic. Use `kubectl describe` on the composite resource to see events and any errors from the function pipeline.

### Step 3 — Observe function execution

When a composite resource reconciles, Crossplane calls your function and records the result. Watch for reconciliation activity:

```bash
kubectl get composite -o wide
kubectl describe composite my-resource
```

Look at the `Events` section of the describe output — it shows when the pipeline ran, which steps succeeded, and any errors your function returned.

> [!NOTE]
> Function development involves writing and testing the function's gRPC handler logic. The Crossplane SDK provides a test harness that lets you unit-test your function without deploying it. Use `overlock fnc serve` for integration testing once the unit tests pass.

---

## Command Reference

### `overlock fnc apply <url>`

Installs a function from a remote registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--wait` / `-w` | `true` | Wait for the function to become ready |
| `--timeout` / `-t` | — | How long to wait before giving up |

### `overlock fnc list`

Lists all functions currently installed in the active environment. No flags.

### `overlock fnc delete <url>`

Removes an installed function.

| Argument | Description |
|----------|-------------|
| `url` | The package URL used when the function was installed |

### `overlock fnc load`

Loads a function from a local archive file or stdin.

| Flag | Default | Description |
|------|---------|-------------|
| `--path` | — | Path to the OCI archive file |
| `--stdin` | `false` | Read the archive from stdin |
| `--apply` | `false` | Apply the function immediately after loading |
| `--upgrade` | `false` | Upgrade if the function is already installed |

### `overlock fnc serve <path>`

Watches a local directory and hot-reloads the function on changes.

| Argument | Default | Description |
|----------|---------|-------------|
| `path` | `./` | Path to the function source directory |

---

## Related Guides

- [Configurations](configurations.md) — configurations commonly include or depend on functions
- [Providers](providers.md) — install providers that supply the managed resource types your functions compose
- [Resources](resources.md) — create composite resources to exercise your function pipeline
- [Registries](registries.md) — set up a local registry for function development
