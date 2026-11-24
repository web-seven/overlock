---
slug: /environment/spec
---

# Environment Specification

`Environment` is the canonical Overlock API for what is installed inside a
Kubernetes cluster: Crossplane, its packages and supporting tools. It is a
Kubernetes custom resource, so the same specification can be stored in Git,
applied with `kubectl`, and reconciled by an operator.

How the cluster itself is created (engine, ports, mounts, nodes, SSH, Docker,
WireGuard) is CLI-specific and is **not** part of this API. Those settings stay
at the top level of [overlock.yaml](cfg-file.md).

---

## Resource

`Environment` is a cluster-scoped resource in the `overlock.io/v1alpha1` API
group. One `Environment` describes the installation of one cluster.

```yaml
apiVersion: overlock.io/v1alpha1
kind: Environment
metadata:
  name: default
spec:
  crossplane:
    maxReconcileRate: 5
    args: []
  providers:
    - xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0
  configurations: []
  functions: []
  tools:
    kyverno:
      enabled: true
    certManager:
      enabled: true
  adminServiceAccount:
    name: overlock-admin
```

## Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `crossplane.maxReconcileRate` | int | Crossplane default | Maximum reconciliations per second for Crossplane |
| `crossplane.args` | list of strings | — | Additional arguments for the Crossplane core container |
| `providers` | list of strings | — | Crossplane provider packages to install |
| `configurations` | list of strings | — | Crossplane configuration packages to install |
| `functions` | list of strings | — | Crossplane function packages to install |
| `tools.kyverno.enabled` | bool | `true` | Install the Kyverno policy engine |
| `tools.certManager.enabled` | bool | `true` | Install cert-manager |
| `adminServiceAccount` | object | — | When set, create a service account with `cluster-admin` privileges |
| `adminServiceAccount.name` | string | `overlock-admin` | Name of the admin service account |

## Status

| Field | Description |
|-------|-------------|
| `observedGeneration` | Generation of the resource last processed |
| `conditions` | Standard Kubernetes conditions. `Ready` is `True` when everything in `spec` is installed |

---

## Mapping from overlock.yaml

| overlock.yaml (legacy top level) | Environment `spec` |
|----------------------------------|--------------------|
| `providers` | `providers` |
| `configurations` | `configurations` |
| `functions` | `functions` |
| max reconcile rate | `crossplane.maxReconcileRate` |
| create admin service account + name | `adminServiceAccount.name` |
| — | `crossplane.args`, `tools` |
| `engine`, `engine_config`, ports, `context`, `mount`, `cpu`, `nodes` | not mapped (CLI-specific) |

The `environment` section of overlock.yaml uses the `spec` schema directly and
overrides the legacy keys. See [Config File](cfg-file.md#environment-section).

## Converting overlock.yaml

```bash
overlock env manifest my-env > environment.yaml
```

prints the `Environment` resource built from the configuration file(s) in the
current directory, or from `--config`.

## Installing the CRD

The CustomResourceDefinition is generated from `api/v1alpha1` and stored in
`config/crd`:

```bash
make manifests            # regenerate CRD and deepcopy code
kubectl apply -k config/crd
```
