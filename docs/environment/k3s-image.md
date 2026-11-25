---
slug: /environment/k3s-image
---

# Overlock K3s Image

`ghcr.io/web-seven/overlock-k3s` is a [K3s](https://k3s.io) image with
everything a default Overlock environment needs, so an environment can start
without pulling each component image separately.

It contains:

- K3s (`rancher/k3s`) of the version pinned by Overlock
- The Overlock operator bootstrap manifests (CRD, RBAC, deployment) at
  `/opt/overlock/manifests/operator.yaml`
- Preloaded image tarballs at `/opt/overlock/images/`:
  - `overlock-images.tar.zst`: the operator, Crossplane, cert-manager and
    Kyverno, including the images of their install, upgrade and removal hooks;
    the list is at `/opt/overlock/images.txt`
  - `k3s-airgap-images.tar.zst`: the K3s system images (CoreDNS,
    metrics-server, local-path provisioner, Traefik, ...) from the K3s release,
    verified against its published checksums

A default environment therefore starts without downloading any image.

The image list is computed from the Helm charts the operator installs, with the
same versions and values, so the bundle always matches the operator of the same
release.

---

## Tags

| Tag | Content |
|-----|---------|
| `<overlock version>` | Bundle of that Overlock release with its default K3s version |
| `<overlock version>-k3s-<k3s version>` | Same, with the K3s version in the tag, e.g. `2.0.0-k3s-v1.36.2-k3s1` |
| `latest` | Latest stable release |

Images are published for `linux/amd64` and `linux/arm64`.

---

## Preloaded Images and Volumes

K3s imports image tarballs from `<data-dir>/agent/images` at startup. The
bundle is stored in `/opt/overlock/images` instead, because
`/var/lib/rancher/k3s` is a volume and would hide files baked into it. The image
entrypoint links the bundle into the data directory on every start, so it is
also available when the data directory is a mounted or persistent volume. The
data directory is taken from `--data-dir`/`-d` or `K3S_DATA_DIR`.

```bash
docker run -d --privileged --name k3s \
  -v k3s-data:/var/lib/rancher/k3s \
  ghcr.io/web-seven/overlock-k3s:latest server
```

The operator manifests are only bundled; they are not applied automatically.

---

## Building

```bash
# Local platform, bundling a given operator image
make docker-build-k3s K3S_IMG=overlock-k3s:dev K3S_OPERATOR_IMG=ghcr.io/web-seven/overlock-operator:<version>

# Another K3s version
make docker-build-k3s K3S_VERSION=v1.35.4-k3s1

# Multi-arch, pushed
make docker-push-k3s K3S_IMG=<registry>/overlock-k3s:<tag>
```

`make k3s-image-context` only generates `k3s/images.txt` and
`k3s/operator.yaml` with `hack/k3s-image`. Release builds run in the release
workflow after the operator image of the same tag is published.
