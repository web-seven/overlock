---
id: command-reference
title: Command Reference
sidebar_label: Command Reference
sidebar_position: 2
pagination_next: null
pagination_prev: null
---

# Command Reference

Welcome to the official documentation for the KNDP Command-Line Interface (CLI) tool!

This guide will provide you with all the information you need to get started with KNDP CLI, including:

- Basic Usage: Learn the fundamental commands and options available in KNDP CLI.
- Installation: Install KNDP CLI on your system quickly and easily.
- Uninstallation: Remove KNDP CLI from your system if you no longer need it.


Let's dive in! 🏊‍♀️

## Available commands

```bash
Usage: kndp <command>

Kubernetes Native Development Platform CLI.

For more details open https://kndp.io

Flags:
  -h, --help       Show context-sensitive help.
  -D, --debug      Enable debug mode
      --version    Print version information and quit

Commands:
  help                       Show help.

  environment (env)          KNDP Environment commands
    create                   Create an Environment
      [<name>]               Name of environment.
    delete                   Delete an Environment
      <name>                 Name of environment.
    copy                     Copy an Environment to another destination context
      <source>               Name source of environment.
      <destination>          Name destination of environment.
    list                     List of Environments

  configuration (cfg)        KNDP Configuration commands
    apply                    Apply Crossplane Configuration.
      <link>                 Link URL to Crossplane configuration to be applied to Environment.
    list                     Apply Crossplane Configuration.
    delete                   Delete Crossplane Configuration.
      <configuration-url>    Specifies the URL of configuration to be deleted from Environment.

  resource (res)             KNDP Resource commands
    create                   Create an XR
      <type>                 XRD type name.
    list                     List of XRs
    apply                    Apply an XR

  registry (reg)             Packages registy commands
    create                   Create registry
    list                     List registries
    delete                   Delete registry

  install-completions        Install shell completions

  provider                   KNDP Provider commands
    install                  Install Crossplane Provider.
      <provider-url>         Provider URL to Crossplane provider to be installed to Environment.
    list                     Install Crossplane Provider.

Run "kndp <command> --help" for more information on a command.


```

We encourage you to explore and experiment with KNDP CLI to discover all its capabilities. 🧑‍💻