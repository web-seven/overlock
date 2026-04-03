# Plugins

## What is it

Plugins extend Overlock with custom commands that aren't built into the core CLI. A plugin is a standalone executable file placed in the plugins directory. When you run `overlock my-plugin`, Overlock looks for an executable named `my-plugin` in the plugin path and runs it.

This lets teams or individuals add commands specific to their workflow without modifying the core tool.

---

## When would I use it

Your team has a common workflow — say, bootstrapping a new environment with a specific set of providers and configurations — that would be tedious to run manually every time. You write a small script or program, drop it in the plugins directory, and it becomes an `overlock` command that everyone on the team can use.

---

## How to use it

### Where to put plugins

The default plugin directory is:

```
~/.config/overlock/plugins
```

Any executable file in that directory is available as an `overlock` command.

### Use a custom plugin directory

If you want to keep plugins elsewhere, use the `--plugin-path` global flag:

```bash
overlock --plugin-path /path/to/my/plugins my-plugin
```

### Writing a plugin

A plugin can be any executable — a shell script, a Go binary, a Python script, etc. Overlock passes command-line arguments through to the plugin unchanged.

**Example: a simple shell plugin**

Save this as `~/.config/overlock/plugins/bootstrap`:

```bash
#!/bin/bash
set -e

ENV_NAME=${1:-my-env}

echo "Creating environment $ENV_NAME..."
overlock env create "$ENV_NAME" --engine k3s-docker

echo "Installing standard providers..."
overlock prv install xpkg.upbound.io/crossplane-contrib/provider-gcp:v0.22.0

echo "Done."
```

Make it executable:

```bash
chmod +x ~/.config/overlock/plugins/bootstrap
```

Now run it:

```bash
overlock bootstrap my-staging-env
```

### Sharing plugins with a team

You can version-control your plugins directory and have team members point their `--plugin-path` at a shared location, or check individual plugin files into a project repository.

---

## Command Reference

Plugins don't have a specific sub-command under `overlock` — they are invoked by name:

```bash
overlock <plugin-name> [args...]
```

### Global flag for plugins

| Flag | Default | Description |
|------|---------|-------------|
| `--plugin-path` | `~/.config/overlock/plugins` | Directory to look for plugin executables |

---

## Related guides

- [Environments](environments.md) — the most common thing plugins automate
- [Getting Started](getting-started.md) — understand the core workflow before automating it
