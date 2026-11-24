---
slug: /environment/operator
---

# Operator

The Overlock operator runs inside a Kubernetes cluster and reconciles
[Environment](spec.md) resources: it installs, upgrades and removes Crossplane,
its packages and supporting tools so the cluster matches the specification.

---

## Installation

`overlock env create` and `overlock env upgrade` install the operator
automatically, using the image matching the CLI version
(`ghcr.io/web-seven/overlock-operator:<version>`, `latest` for development
builds; override with `--operator-image`). The manifests are embedded in the
CLI and applied with server-side apply, so running the commands again upgrades
the operator in place.

To install it into an existing cluster manually, build and push the operator image, then deploy the CRD, RBAC and the operator
into the current kubeconfig context:

```bash
make docker-build-operator docker-push-operator IMG=<registry>/overlock-operator:<tag>
make deploy IMG=<registry>/overlock-operator:<tag>
kubectl apply -f config/samples/overlock_v1alpha1_environment.yaml
kubectl get environments
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

---

## Reconciliation

Components are reconciled in dependency order:

1. cert-manager
2. Crossplane
3. Kyverno
4. Package runtime scheduling (when `scheduling` is set) and Crossplane packages
   (providers, configurations, functions), once Crossplane is ready
5. Admin service account

With `scheduling` set, its node selector and tolerations are added to the chart
values of each component and to the `default` `DeploymentRuntimeConfig`, so
provider and function pods run on the same nodes.

Reconciliation is idempotent. A Helm release is upgraded only when its chart
version or values change, or when its last operation did not succeed. Packages
are created as Crossplane `Provider`, `Configuration` and `Function` resources;
changing a package reference updates the resource, and removing it from the
specification deletes the resource.

Readiness is checked per component: Helm components are ready when all their
deployments are available, external components when their CRDs are
established, and packages when they are `Installed` and `Healthy`.

The `Ready` condition summarizes all components:

| Reason | Meaning | Retry |
|--------|---------|-------|
| `Available` | All components are ready | every 10 minutes, to detect drift |
| `Progressing` | Components are installed but not ready yet | every 15 seconds |
| `DependencyMissing` | A component with `External` policy is not installed | every minute |
| `Failed` | Installing, upgrading or removing a component failed | every minute |
| `Deleting` | The Environment is being deleted with `Delete` policy | every 15 seconds |

The state of each component is listed in `status.components`.

---

## Ownership

The operator only modifies or removes components it owns. Ownership is recorded
with the `overlock.io/environment: <environment name>` label on Helm releases
and Kubernetes resources. Components without this label, or with the label of
another Environment, are never modified or removed.

## Management Policies

`crossplane.policy` and `tools.<tool>.policy` define how existing installations
are treated:

| Policy | Component missing | Release with expected name, not owned | Installed without that release (CRDs found) |
|--------|-------------------|---------------------------------------|---------------------------------------------|
| `Auto` (default) | installed and owned | used as-is (`External`) | used as-is (`External`) |
| `Adopt` | installed and owned | taken over: labeled, upgraded to desired version and values | used as-is (`External`) |
| `External` | reported `Missing` | used as-is (`External`) | used as-is (`External`) |

The expected release names are `overlock-crossplane` (namespace `overlock`),
`cert-manager` (namespace `cert-manager`) and `kyverno` (namespace `kyverno`),
the same as the ones created by the Overlock CLI. To bring an environment
created by the CLI under operator management, use `Adopt`.

With `crossplane.policy: Adopt`, existing packages with the expected names are
adopted as well; otherwise they are left untouched and reported as `External`.

Ownership is never taken from another Environment, even with `Adopt`.

## Deletion

An Environment carries the `overlock.io/environment` finalizer, so its
`deletionPolicy` is applied before it is removed:

- `Orphan` (default): all components stay in the cluster.
- `Delete`: owned packages are removed first; once they are gone, the owned
  admin service account and owned Helm releases are removed in reverse
  installation order. Components not owned by the Environment are kept.

Disabling a tool, removing a package or the admin service account from the
specification removes it immediately, if it is owned.

## Uninstalling the Operator

```bash
make undeploy
```

deletes all Environments first and waits for them, so their deletion policy is
applied while the operator is still running, then removes the operator, its
RBAC and the CRD. With the default `Orphan` policy, Crossplane, packages and
tools keep running. Nothing without the ownership label is ever removed.

If the operator is removed before its Environments, delete the remaining
Environments' finalizers manually:

```bash
kubectl patch environment <name> --type=merge -p '{"metadata":{"finalizers":null}}'
```
