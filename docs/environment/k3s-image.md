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
    Kyverno, including the images of their install hooks; the list is at
    `/opt/overlock/images.txt`
  - `k3s-system-images.tar.zst`: the K3s system images (CoreDNS,
    metrics-server, local-path provisioner, ...) listed by the K3s release,
    without Traefik; the list is at `/opt/overlock/k3s-images.txt`
- K3s server defaults that disable Traefik and the network policy controller,
  which Overlock environments do not use (installed into `/etc/rancher/k3s/config.yaml.d/00-overlock.yaml` for
  servers only, since agents reject server options)
- Environment profiles at `/opt/overlock/profiles/`

A default environment therefore starts without downloading any image. Images
of upgrade, removal and test hooks (for example `kyverno-cli` for Kyverno CRD
migration on upgrades) are not bundled and are pulled when they run.

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

---

## Automatic Bootstrap

Started as `server`, the container bootstraps a complete Overlock environment
by itself:

1. K3s starts and imports the preloaded images.
2. The entrypoint writes the operator manifests and an `Environment` into the
   K3s auto-deploy directory (`<data-dir>/server/manifests`), which K3s applies.
3. The operator installs Crossplane and the tools described by the
   `Environment`.

```bash
docker run -d --privileged --name overlock \
  -v overlock-data:/var/lib/rancher/k3s \
  ghcr.io/web-seven/overlock-k3s:latest server
```

The files are written on every start: a newer image upgrades the operator, and
the mounted file or profile stays the source of the `Environment` (changes
made with `kubectl` are replaced on the next start).

### Environment Specification

The `Environment` comes from, in order:

1. A mounted `Environment` resource at `/etc/overlock/environment.yaml`
   (path: `OVERLOCK_ENVIRONMENT_FILE`). Generate it from `overlock.yaml` with
   `overlock env manifest`:

   ```bash
   overlock env manifest dev > environment.yaml
   docker run -d --privileged \
     -v $PWD/environment.yaml:/etc/overlock/environment.yaml:ro \
     ghcr.io/web-seven/overlock-k3s:latest server
   ```

2. A bundled profile selected with `OVERLOCK_PROFILE`, named
   `OVERLOCK_ENVIRONMENT_NAME`:

   | Profile | Components |
   |---------|------------|
   | `default` | Crossplane, cert-manager, Kyverno, image store caching `xpkg.crossplane.io` and `xpkg.upbound.io` |
   | `minimal` | Crossplane |

| Variable | Default | Description |
|----------|---------|-------------|
| `OVERLOCK_BOOTSTRAP` | `true` | `false` runs plain K3s without the operator |
| `OVERLOCK_PROFILE` | `default` | Profile used when no environment file is mounted |
| `OVERLOCK_ENVIRONMENT_NAME` | `default` | Name of the `Environment` created from a profile |
| `OVERLOCK_ENVIRONMENT_FILE` | `/etc/overlock/environment.yaml` | Mounted `Environment` resource |

An unknown profile or an environment file that is not an `Environment`
resource stops the container with an error.

### Health and Readiness

The image has a Docker health check (`/bin/overlock-healthcheck`). The
container is `healthy` when:

- server with bootstrap: the `Environment` is `Ready`
- server with `OVERLOCK_BOOTSTRAP=false`: the Kubernetes API is ready
- agent: the kubelet is healthy

Installing the components on first start takes a few minutes, covered by the
start period. Until the environment is ready, its progress is written to the
container log:

```text
overlock: environment 'default' from profile 'default'
overlock: waiting for K3s
overlock: environment 'default': waiting for the operator
overlock: environment 'default': Progressing: waiting for crossplane
overlock: environment 'default' is ready
```

Wait for it with `docker inspect -f '{{.State.Health.Status}}' overlock` or
`docker exec overlock overlock-healthcheck --status`.

### Agents

The same image joins a server as an agent; bootstrap only runs on servers:

```bash
docker run -d --privileged \
  -e K3S_URL=https://<server>:6443 -e K3S_TOKEN=<token> \
  ghcr.io/web-seven/overlock-k3s:latest agent
```

---

## Building

```bash
# Local platform, bundling a given operator image
make docker-build-k3s K3S_IMG=overlock-k3s:dev K3S_OPERATOR_IMG=ghcr.io/web-seven/overlock-operator:<version>

# Another K3s version
make docker-build-k3s K3S_VERSION=v1.35.4-k3s1

# Keep other K3s add-on images (space separated names to exclude)
docker build --build-arg K3S_EXCLUDE_IMAGES="klipper-helm" k3s

# Multi-arch, pushed
make docker-push-k3s K3S_IMG=<registry>/overlock-k3s:<tag>
```

`make k3s-image-context` only generates `k3s/images.txt` and
`k3s/operator.yaml` with `hack/k3s-image`. Release builds run in the release
workflow after the operator image of the same tag is published.
