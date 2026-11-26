---
slug: /environment/compose
---

# Docker Compose

An Overlock environment can run with Docker Compose alone, without the CLI.
`compose.yaml` starts the [Overlock K3s image](k3s-image.md), which bootstraps
the operator and an `Environment` by itself.

Each release attaches a `compose.yaml` pinned to its image version. The file in
the repository uses `latest`, which only exists for stable releases; set
`OVERLOCK_VERSION` for pre-releases.

---

## Quick Start

```bash
curl -LO https://github.com/web-seven/overlock/releases/download/<version>/compose.yaml
docker compose up -d
docker compose ps          # wait until the k3s service is "healthy"
```

The directory of `compose.yaml` is mounted into the container: once the API is
ready, the container writes `kubeconfig` there (see [CLI Access](#cli-access)),
and reads an optional `environment.yaml` from it. The cluster state lives in a
Docker volume.

Progress is written to the log until the environment is ready:

```bash
docker compose logs -f k3s | grep overlock:
```

---

## Configuration

Settings are read from the shell environment or an `.env` file next to
`compose.yaml`:

| Variable | Default | Description |
|----------|---------|-------------|
| `OVERLOCK_VERSION` | `latest` (pinned in release files) | Image version |
| `OVERLOCK_IMAGE` | `ghcr.io/web-seven/overlock-k3s:<version>` | Full image reference |
| `OVERLOCK_ENVIRONMENT_NAME` | `overlock` | `Environment` name and kubeconfig context |
| `OVERLOCK_PROFILE` | `default` | `default` or `minimal`, when no environment file is present |
| `OVERLOCK_BIND_ADDRESS` | `127.0.0.1` | Host address the ports are published on |
| `OVERLOCK_API_PORT` | `6443` | Kubernetes API port |
| `OVERLOCK_HTTP_PORT` | `8080` | Host port for port 80 (LoadBalancer services) |
| `OVERLOCK_HTTPS_PORT` | `8443` | Host port for port 443 (LoadBalancer services) |

Ports are published on `127.0.0.1` by default, so the API is not reachable from
other machines. Set `OVERLOCK_BIND_ADDRESS=0.0.0.0` to expose them.

### Environment Specification

By default the `Environment` comes from the profile. To use your own
`Environment` resource instead, put `environment.yaml` next to `compose.yaml`,
for example generated from `overlock.yaml`:

```bash
overlock env manifest overlock > environment.yaml
docker compose up -d --force-recreate
```

The file is applied on every start and is the source of the `Environment`:
edit it and recreate the container (`docker compose up -d --force-recreate`)
to change the environment. See [Environment Specification](spec.md).

### Packages

The default profile serves packages from the nodes' containerd with the
[image store](operator.md#image-store); there is no separate registry. Load
packages with the CLI against the Compose environment:

```bash
export KUBECONFIG=$PWD/kubeconfig
overlock cfg load my-config:v1.0.0 --path ./my-config.xpkg --apply
```

`load` writes the package into the `overlock-images-<OVERLOCK_ENVIRONMENT_NAME>`
volume, mounted at `/var/lib/rancher/k3s/agent/images/overlock` (a sub-folder of
the K3s images directory, next to the images bundled with the K3s image), where
K3s imports it, and Crossplane installs it from the store.
Packages of `xpkg.crossplane.io` and `xpkg.upbound.io`, including dependencies,
are cached in containerd on first use. With your own `environment.yaml`, enable
the store the same way and keep `metadata.name` equal to
`OVERLOCK_ENVIRONMENT_NAME`, the name the CLI uses to find the volume:

```yaml
spec:
  imageStore:
    enabled: true
    cache: ["xpkg.crossplane.io/", "xpkg.upbound.io/"]
```

### Persistent State

The cluster state is stored in the `k3s-data` volume. `docker compose down`
and `up` keep the cluster, installed packages and resources;
`docker compose down -v` deletes them.

The container has a fixed hostname, so a recreated container is the same K3s
node. Changing the hostname would register a second node and leave the
workloads of the old one stuck.

---

## CLI Access

Once the API is ready, the container writes `kubeconfig` next to
`compose.yaml`, with the context named after `OVERLOCK_ENVIRONMENT_NAME`. It is
owned by the owner of that directory and readable only by them (mode `600`), as
it holds the cluster administrator credentials:

```bash
export KUBECONFIG=$PWD/kubeconfig
kubectl get environments
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0
```

It is rewritten on every start and stays valid as long as the `k3s-data`
volume is kept. To merge it into your default kubeconfig:

```bash
KUBECONFIG=~/.kube/config:$PWD/kubeconfig kubectl config view --flatten > /tmp/config \
  && mv /tmp/config ~/.kube/config
```

The API server listens on `OVERLOCK_API_PORT` inside the container as well, so
the kubeconfig address (`https://127.0.0.1:<port>`) is correct for any port.



### Services Using the API

Other containers reach the API at `https://k3s:<OVERLOCK_API_PORT>` on the
Compose network. The container writes a kubeconfig with that address to the
`kubeconfig` volume (`/kubeconfig/config`), and the API certificate covers the
`k3s` and `overlock` names, so TLS is verified as usual.

Add a service to the same `compose.yaml`, or to a `compose.override.yaml` next
to it:

```yaml
services:
  app:
    image: your/app
    environment:
      KUBECONFIG: /kubeconfig/config
    volumes:
      - kubeconfig:/kubeconfig:ro
    depends_on:
      k3s:
        condition: service_healthy
```

`service_healthy` starts the service once the `Environment` is ready, so the
kubeconfig exists and Crossplane is available.

A service of another Compose project uses the external volume and network of
the `overlock` project:

```yaml
services:
  app:
    image: your/app
    environment:
      KUBECONFIG: /kubeconfig/config
    volumes:
      - overlock-kubeconfig:/kubeconfig:ro
    networks: [overlock]

volumes:
  overlock-kubeconfig:
    external: true
    name: overlock_kubeconfig

networks:
  overlock:
    external: true
    name: overlock_default
```

The shared kubeconfig is owned by `root` with mode `644`, so services can read
it whatever user they run as; on the host it is only reachable through Docker.
Both kubeconfigs carry the cluster administrator credentials: every service
that mounts the volume has full access to the cluster.

---

## Limitations

- **Nodes:** `overlock env node create` and `env node delete` only manage
  environments created by the CLI with the `k3s-docker` engine. They look up
  the server container (`k3s-docker-<name>`), the kube context
  (`k3s-docker-<name>`) and the WireGuard network (`overlock-<name>`) created
  by `overlock env create`, which a Compose environment does not have. Remote
  nodes over SSH and WireGuard therefore cannot be added to a Compose
  environment.
- **Other CLI environment commands:** `env start`, `env stop`, `env delete`
  and `env upgrade` do not apply; use `docker compose start`, `stop`, `down`
  and a newer image instead.
- **Ingress:** Traefik is disabled. Ports 80 and 443 are served by K3s
  LoadBalancer services, if any are created.
