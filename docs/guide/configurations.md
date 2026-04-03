# Configurations

A [Crossplane configuration](https://docs.crossplane.io/latest/concepts/packages/) is a package that bundles together Composite Resource Definitions (XRDs) and Compositions — the building blocks that define what kinds of infrastructure your cluster can create. Installing a configuration is how you add new capabilities to your environment: before it's installed, the cluster doesn't know what a `Database` or `AppClaim` is; after it's installed, it does.

Overlock gives you two modes for working with configurations: consuming published community packages, and developing your own with a fast live-reload loop. This guide covers both.

> [!NOTE]
> If you haven't created an environment yet, start there. The [Getting Started guide](getting-started.md) walks through the full flow from scratch. The commands in this guide assume you have an active environment pointed at by your current Kubernetes context.

---

## Installing a Community Configuration

The fastest way to understand what a configuration does is to install one. The Upbound marketplace and the Crossplane community publish dozens of ready-to-use configurations. Installing one is a single command:

```bash
overlock cfg apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
```

Overlock pulls the package, applies it to your cluster, and waits until Crossplane reports it healthy. You won't get your terminal back until everything is ready — or until the timeout is reached.

> [!TIP]
> Configurations can declare dependencies on providers and functions. Crossplane resolves and installs those dependencies automatically. After applying a configuration, run `overlock prv list` and `overlock fnc list` to see what was pulled in alongside it.

Once the configuration is installed, new resource types are registered in your cluster. Run the following to see what's now available:

```bash
overlock res list
```

From here you can start creating resources. See the [Resources guide](resources.md) for how to do that.

### Installing without waiting

If you're running in a CI pipeline or want to kick off multiple installs in parallel, you can skip the wait:

```bash
overlock cfg apply xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31 --wait=false
```

You can then check status manually with `overlock cfg list`.

---

## Developing Your Own Configuration

When you're building a configuration from scratch — writing XRDs, compositions, and wiring up functions — Overlock's `serve` command gives you a tight feedback loop. Instead of packaging, pushing, and reinstalling every time you change a file, `serve` watches your directory and hot-reloads changes into your cluster automatically.

### Step 1 — Set up a local registry

The `serve` command needs a local OCI registry to push packages to. If you haven't set one up yet:

```bash
overlock reg create --local --default
```

> [!NOTE]
> You only need to do this once. The registry persists across environment restarts. See the [Registries guide](registries.md) for more detail on what this sets up.

### Step 2 — Start the live-reload server

Point `serve` at your package directory:

```bash
overlock cfg serve ./my-config-package
```

Overlock builds your package, pushes it to the local registry, installs it in your cluster, and then watches for file changes. Every time you save a file, the cycle repeats. Changes are usually live in the cluster within a few seconds.

> [!TIP]
> Keep `serve` running in a dedicated terminal window while you work. Open a second terminal to interact with the cluster using `kubectl` or `overlock res` commands. This way you can see reload output and test your changes side by side.

### Step 3 — Iterate on your XRDs and Compositions

With `serve` running, editing your composition files is the core of the development workflow. Make a change to your `composition.yaml`, save it, and watch the serve output confirm the reload. Then verify the change took effect:

```bash
kubectl get compositions
kubectl describe composition my-composition
```

> [!NOTE]
> If your composition depends on functions, make sure those functions are installed in your environment before serving. If your `crossplane.yaml` declares function dependencies, Crossplane will install them automatically when the package loads — but only if the function registry is reachable. For local function development, see the [Functions guide](functions.md).

### Step 4 — Test with a real resource

Create an instance of your composite resource to verify the full pipeline works:

```bash
overlock res apply -f my-test-claim.yaml
```

Then watch the resource and any composed resources it created:

```bash
kubectl get managed
```

See the [Resources guide](resources.md) for more on observing resource status and troubleshooting.

---

## Loading from a Local Archive

If you've built a configuration package and exported it as an OCI archive (for example, using `crossplane xpkg build`), you can load it directly:

```bash
overlock cfg load --path ./my-config.tar --apply
```

This pushes the archive to your local registry and applies the configuration in one step. To upgrade an already-installed configuration:

```bash
overlock cfg load --path ./my-config.tar --apply --upgrade
```

In CI pipelines where the archive is streamed rather than written to disk, you can pipe it via stdin:

```bash
cat my-config.tar | overlock cfg load --stdin --apply
```

> [!TIP]
> The `load` command is useful in release pipelines where you want to test a specific built artifact rather than the current state of your source directory. Use `serve` during active development and `load` when validating a release candidate.

---

## Removing a Configuration

When you want to remove a configuration from your environment:

```bash
overlock cfg delete xpkg.upbound.io/devops-toolkit/dot-application:v3.0.31
```

Pass the same URL you used when installing.

> [!WARNING]
> Removing a configuration unregisters the XRDs and Compositions it defined. Any resources of those types that exist in the cluster will lose their reconciling composition. Delete those resources before removing the configuration to avoid orphaned managed resources.

---

## Command Reference

### `overlock cfg apply <url>`

Installs a configuration from a remote registry.

| Flag | Default | Description |
|------|---------|-------------|
| `--wait` / `-w` | `true` | Wait for the configuration to become ready |
| `--timeout` / `-t` | — | How long to wait before giving up |

### `overlock cfg list`

Lists all configurations currently installed in the active environment. No flags.

### `overlock cfg delete <url>`

Removes an installed configuration. Pass the same URL used when installing.

### `overlock cfg load`

Loads a configuration from a local archive file or stdin.

| Flag | Default | Description |
|------|---------|-------------|
| `--path` | — | Path to the OCI archive file |
| `--stdin` | `false` | Read the archive from stdin |
| `--apply` | `false` | Apply the configuration immediately after loading |
| `--upgrade` | `false` | Upgrade if the configuration is already installed |

### `overlock cfg serve <path>`

Watches a local directory for changes and live-reloads the configuration into the active environment.

| Argument | Default | Description |
|----------|---------|-------------|
| `path` | `./` | Path to the configuration package directory |

---

## Related Guides

- [Providers](providers.md) — install the providers that your configuration's compositions depend on
- [Functions](functions.md) — install and develop the composition functions your XRDs use
- [Resources](resources.md) — create and observe instances of the resource types your configuration defines
- [Registries](registries.md) — set up and manage the local registry used by `serve` and `load`
- [Getting Started](getting-started.md) — end-to-end walkthrough
