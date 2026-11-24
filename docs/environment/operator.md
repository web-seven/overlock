---
slug: /environment/operator
---

# Operator

The Overlock operator runs inside a Kubernetes cluster and installs a
Crossplane environment described by an `Environment` custom resource. It shares
its installation logic with `overlock env create`, so the same specification
gives the same result whether it is applied by the CLI or by the operator.

The CLI creates clusters (engine, nodes, SSH, Docker, WireGuard); the operator
only installs what goes inside an existing cluster.

---

## Environment Specification

`Environment` is a cluster-scoped resource in the `overlock.io/v1alpha1` API
group. One `Environment` describes what the operator installs in its own
cluster.

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

### Status

The operator reports progress in `status.conditions`. The `Ready` condition is
`True` once Crossplane, packages and tools are installed for the current
`metadata.generation`, and `False` with reason `InstallFailed` and the error
message otherwise. Failed installations are retried every minute.

```bash
kubectl get environments
```

---

## Installation

Build and push the operator image, then deploy the CRD, RBAC and the operator
into the current kubeconfig context:

```bash
make docker-build-operator docker-push-operator IMG=<registry>/overlock-operator:<tag>
make deploy IMG=<registry>/overlock-operator:<tag>
kubectl apply -f config/samples/overlock_v1alpha1_environment.yaml
```

The operator runs in the `overlock-system` namespace. It installs Helm charts
that create cluster-wide resources, so its service account is bound to
`cluster-admin`.

For local development, install only the CRD and run the operator against the
current context:

```bash
make install
make run-operator
```

Remove everything with `make undeploy`.

---

## From overlock.yaml

The `environment` section of [overlock.yaml](cfg-file.md#environment-section)
uses the same schema as `spec`. Convert a configuration file into a resource
with:

```bash
overlock env manifest my-env | kubectl apply -f -
```

---

## Current Limitations

- Deleting an `Environment` does not uninstall Crossplane or tools.
- Changes to packages or Crossplane settings are not applied to an already
  installed Crossplane release yet; the installation is skipped when the
  release exists, as with `overlock env upgrade`.
