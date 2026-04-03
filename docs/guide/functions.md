# Functions

## What is it

A Crossplane function is a small piece of logic that runs when Crossplane is building or updating a composite resource. Functions handle the "how do I turn this high-level resource request into concrete infrastructure?" question.

When you define a composition, you can use functions to apply patches, transform values, run custom logic, and more. Think of functions as the step-by-step instructions Crossplane follows when creating a resource.

---

## When would I use it

You're building or using a Crossplane composition that needs logic beyond simple field mapping. For example, the `function-patch-and-transform` function is commonly used in compositions to copy and transform values between resources.

If a configuration you're installing lists functions as dependencies, Overlock installs them automatically. But you can also install functions directly.

---

## How to use it

### Install a function

```bash
overlock fnc apply xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0
```

Overlock installs the function and waits for it to be ready.

### List installed functions

```bash
overlock fnc list
```

### Remove a function

```bash
overlock fnc delete xpkg.upbound.io/crossplane-contrib/function-patch-and-transform:v0.7.0
```

### Load from a local archive

```bash
overlock fnc load --path ./my-function.tar --apply
```

Or from stdin:

```bash
cat my-function.tar | overlock fnc load --stdin --apply
```

### Serve a function for live development

When building a function, serve it from your local directory to get hot-reload behavior:

```bash
overlock fnc serve ./my-function
```

Overlock watches for changes and reloads the function in your environment automatically.

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
| `--path` | — | Path to the archive file |
| `--stdin` | `false` | Read the archive from stdin |
| `--apply` | `false` | Apply the function immediately after loading |
| `--upgrade` | `false` | Upgrade if the function is already installed |

### `overlock fnc serve <path>`

Watches a local directory and hot-reloads the function on changes.

| Argument | Default | Description |
|----------|---------|-------------|
| `path` | `./` | Path to the function source directory |

---

## Related guides

- [Configurations](configurations.md) — configurations often include or require functions
- [Providers](providers.md) — install providers alongside functions for complete compositions
