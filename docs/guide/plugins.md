# Plugins

Plugins let you extend Overlock with custom commands tailored to your team's workflow. A plugin is any executable file — a shell script, a Go binary, a Python script — placed in the Overlock plugins directory. When you run `overlock my-plugin`, Overlock finds the executable named `my-plugin` in the plugin path and runs it, passing any additional arguments through unchanged.

This is a lightweight but powerful extensibility model: no SDKs, no manifest files, no registration steps. If it's executable and it lives in the right directory, it's an `overlock` command.

---

## Why Plugins Matter for Team Workflows

As your Crossplane development practice matures, you'll find that certain sequences of Overlock commands get repeated constantly. Every new team member needs to bootstrap a development environment the same way. Every release goes through the same build-load-install cycle. Every project needs the same set of providers and configurations.

Plugins are how you codify those workflows once and share them with everyone. Instead of a wiki page listing 12 commands to copy-paste, you have `overlock bootstrap` that does it all. Instead of a CI script that no one understands, you have `overlock release` that anyone can read and modify.

> [!TIP]
> Think of plugins as project-specific or team-specific Overlock commands. The core CLI gives you the primitives; plugins let you compose those primitives into higher-level workflows that make sense for your specific context.

---

## Where Plugins Live

The default plugin directory is:

```
~/.config/overlock/plugins
```

Any executable file in that directory is immediately available as an `overlock` command — no restart required.

To use a different directory (useful for per-project plugins checked into a repository):

```bash
overlock --plugin-path ./scripts/plugins my-plugin
```

You can also set this globally by always using the `--plugin-path` flag in your shell alias or wrapper script.

---

## Writing Your First Plugin

A plugin can be any executable. Here's a realistic example: a `bootstrap` plugin that sets up a complete development environment for a team working on a cloud database abstraction.

Save this as `~/.config/overlock/plugins/bootstrap`:

```bash
#!/bin/bash
set -euo pipefail

ENV_NAME=${1:-dev-env}
REGISTRY_URL="xpkg.upbound.io"

echo "==> Creating environment: $ENV_NAME"
overlock env create "$ENV_NAME" --engine k3s-docker

echo "==> Setting up local package registry"
overlock reg create --local --default

echo "==> Installing required providers"
overlock prv install "$REGISTRY_URL/crossplane-contrib/provider-helm:v0.19.0"
overlock prv install "$REGISTRY_URL/upbound/provider-aws-s3:v1.1.0"

echo "==> Installing composition functions"
overlock fnc apply "$REGISTRY_URL/crossplane-contrib/function-patch-and-transform:v0.7.0"

echo "==> Environment $ENV_NAME is ready."
echo "    Run 'overlock cfg serve ./my-config' to start developing."
```

Make it executable:

```bash
chmod +x ~/.config/overlock/plugins/bootstrap
```

Now any team member can get a fully configured environment with:

```bash
overlock bootstrap my-dev-env
```

> [!NOTE]
> Inside a plugin, you call `overlock` commands just like you would from the terminal. The plugin runs in a normal shell environment with your full `PATH` available. You can call `kubectl`, `helm`, `crossplane`, or any other tool — not just Overlock commands.

---

## A More Advanced Example: Release Pipeline

Here's a plugin that automates the full package release cycle — building, loading into a local registry, and verifying the install:

Save as `~/.config/overlock/plugins/release`:

```bash
#!/bin/bash
set -euo pipefail

PACKAGE_DIR=${1:-.}
PACKAGE_NAME=${2:-my-config}
VERSION=${3:-v0.1.0}

echo "==> Building package from $PACKAGE_DIR"
crossplane xpkg build \
  --package-root "$PACKAGE_DIR" \
  --output "${PACKAGE_NAME}-${VERSION}.xpkg"

echo "==> Loading into local registry"
overlock reg load-image \
  --registry local \
  --path "${PACKAGE_NAME}-${VERSION}.xpkg" \
  --name "${PACKAGE_NAME}:${VERSION}" \
  --upgrade

echo "==> Installing in active environment"
overlock cfg apply "localhost:5000/${PACKAGE_NAME}:${VERSION}"

echo "==> Release $VERSION complete."
```

Use it like this:

```bash
overlock release ./my-config-package my-config v0.2.0
```

> [!TIP]
> Version your plugins in the same repository as the packages they build. This keeps the tooling and the package in sync, and makes it easy for contributors to understand and improve both.

---

## Sharing Plugins with Your Team

There are two common approaches:

**Per-project plugins in the repository** — Keep a `plugins/` or `scripts/` directory in your project and have each developer point their plugin path at it:

```bash
overlock --plugin-path ./plugins release
```

Or add a shell alias in your project's `.envrc` (if using direnv):

```bash
alias overlock="overlock --plugin-path $(pwd)/plugins"
```

**Shared team plugin library** — Maintain a separate repository of shared plugins that team members clone to a standard location (`~/.config/overlock/plugins`). Combined with a git pull in a team onboarding script, this ensures everyone has the latest versions automatically.

> [!WARNING]
> Plugins are executables that run with your full user permissions. Only use plugins from sources you trust. Review plugin scripts before running them, just as you would with any shell script.

---

## Command Reference

Plugins are invoked directly by name — there's no `overlock plugin` subcommand:

```bash
overlock <plugin-name> [args...]
```

### Global flag for plugins

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin-path` | `~/.config/overlock/plugins` | Directory to search for plugin executables |

---

## Related Guides

- [Environments](environments.md) — the most common target of bootstrap and setup plugins
- [Registries](registries.md) — the release cycle that release plugins typically automate
- [Getting Started](getting-started.md) — understand the core workflow before automating it
