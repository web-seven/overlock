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
mkdir -p environment kubeconfig
docker compose up -d
docker compose ps          # wait until the k3s service is "healthy"
```

Create `environment` and `kubeconfig` before the first start, so they belong to
your user; otherwise Docker creates them as `root`.

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

Put an `Environment` resource in `environment/environment.yaml` to replace the
profile, for example generated from `overlock.yaml`:

```bash
overlock env manifest overlock > environment/environment.yaml
docker compose up -d
```

The file is applied on every start and is the source of the `Environment`:
edit it and restart (`docker compose up -d --force-recreate`) to change the
environment. See [Environment Specification](spec.md).

### Persistent State

The cluster state is stored in the `k3s-data` volume. `docker compose down`
and `up` keep the cluster, installed packages and resources;
`docker compose down -v` deletes them.

The container has a fixed hostname, so a recreated container is the same K3s
node. Changing the hostname would register a second node and leave the
workloads of the old one stuck.

---

## CLI Access

The container writes a kubeconfig to `kubeconfig/config`, with the context
named after `OVERLOCK_ENVIRONMENT_NAME`:

```bash
export KUBECONFIG=$PWD/kubeconfig/config
kubectl get environments
overlock provider install xpkg.upbound.io/crossplane-contrib/provider-helm:v0.19.0
```

To merge it into your default kubeconfig:

```bash
KUBECONFIG=~/.kube/config:$PWD/kubeconfig/config kubectl config view --flatten > /tmp/config \
  && mv /tmp/config ~/.kube/config
```

The API server listens on `OVERLOCK_API_PORT` inside the container as well, so
the kubeconfig address (`https://127.0.0.1:<port>`) is correct for any port.

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
