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
4. Once Crossplane is ready: package runtime scheduling (when `scheduling` is
   set), registry rules (`registries`, as Crossplane `ImageConfig`s), then
   Crossplane packages (providers, configurations, functions). Packages wait
   while a registry rule fails, so they and their dependencies always resolve
   with the rules
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

## Image Store

With `imageStore.enabled`, the operator serves the images of the nodes'
containerd to Crossplane as a registry, so packages are installed without a
separate registry:

- A DaemonSet runs the store on every node (from the operator image), behind
  the `registry` Service in the Crossplane namespace, so it is reached as
  `registry.overlock.svc.cluster.local`. Clients use the store of their own
  node.
- cert-manager issues its TLS certificate (`overlock-image-store`), and its CA
  is added to Crossplane's trusted CA bundle.
- The store serves manifests, blobs and tag lists (pull only, read-only for
  clients). A repository starting with a registry host, e.g.
  `xpkg.crossplane.io/crossplane-contrib/function-auto-ready`, refers to that
  registry: missing images are pulled through the CRI (so node registry
  mirrors apply), pinned against image garbage collection and also named with
  the store name pods use. Other repositories refer to images loaded by the
  CLI and are never pulled.
- `imageStore.cache` prefixes become registry rules rewriting them into the
  store, so packages and dependencies of those registries are cached on first
  use and served from containerd afterwards, also offline. Tag lists combine
  cached and upstream tags.
- Packages are installed once the store is ready. An existing `registry`
  Service not owned by the environment (a Docker registry) is not replaced;
  the store reports `Failed` until it is removed.

The store accesses containerd only through its image, content and CRI image
services: it reads images and content, pulls images, and labels and names
pulled images. It never writes content, runs containers or deletes images.

### Why the store reads containerd directly

K3s's embedded registry mirror (Spegel) also serves containerd content, but
Crossplane cannot use it:

- Spegel answers only requests carrying the original registry as the
  non-standard `?ns=<registry>` query parameter, which containerd adds for
  mirrors. Crossplane's OCI client sends standard requests, and an
  `ImageConfig` can only rewrite the image reference, not add parameters, so
  every request returns 404.
- Spegel has no tag listing, which Crossplane needs to resolve dependency
  version ranges, and only serves images some node already has.
- On K3s v1.36 it serves images without requiring a client certificate.

The store answers the standard API itself, including tag listing and pulls of
missing images. Spegel remains useful to distribute images between nodes,
which goes through containerd's mirror support; images imported into
containerd (rather than pulled) are only shared once their content carries the
`containerd.io/distribution.source.<registry>=<repository>` labels.

## Ownership

The operator only modifies or removes components it owns. Ownership is recorded
with the `overlock.app/environment: <environment name>` label on Helm releases
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

An Environment carries the `overlock.app/environment` finalizer, so its
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
