#!/bin/sh
# Spike #523: can Crossplane fetch packages from K3s's embedded registry
# mirror (Spegel)? Starts a 2-node K3s in Docker with the embedded registry,
# imports a provider package into the server node only, and probes the
# registry API the way Crossplane's OCI client does.
#
# Requires: docker, crane, curl. Cleans up on exit.
set -eu

K3S=rancher/k3s:v1.36.2-k3s1
PKG=xpkg.crossplane.io/crossplane-contrib/provider-nop:v0.5.0
NAME=registry.overlock.svc.cluster.local/acme/nop:v0.5.0
NS=registry.overlock.svc.cluster.local
WORK=$(mktemp -d)
trap 'docker rm -f -v spike-server spike-agent >/dev/null 2>&1; docker network rm spike523 >/dev/null 2>&1; rm -rf "$WORK"' EXIT

cat > "$WORK/registries.yaml" <<YAML
mirrors:
  "$NS": {}
  "xpkg.crossplane.io": {}
YAML

docker network create spike523 >/dev/null
docker run -d --privileged --name spike-server --hostname spike-server --network spike523 \
  -e K3S_TOKEN=spike -v "$WORK/registries.yaml:/etc/rancher/k3s/registries.yaml:ro" \
  $K3S server --embedded-registry --disable=traefik >/dev/null
until docker exec spike-server kubectl get nodes 2>/dev/null | grep -q " Ready"; do sleep 3; done
docker run -d --privileged --name spike-agent --hostname spike-agent --network spike523 \
  -e K3S_TOKEN=spike -e K3S_URL=https://spike-server:6443 \
  -v "$WORK/registries.yaml:/etc/rancher/k3s/registries.yaml:ro" $K3S agent >/dev/null
until [ "$(docker exec spike-server kubectl get nodes --no-headers 2>/dev/null | grep -c ' Ready')" = 2 ]; do sleep 3; done

# Import the package into the server node only, as `load` would.
crane pull --format=oci --platform linux/amd64 "$PKG" "$WORK/oci"
tar -C "$WORK/oci" -cf "$WORK/pkg.tar" .
docker cp "$WORK/pkg.tar" spike-server:/tmp/pkg.tar
docker exec spike-server ctr -n k8s.io images import --index-name "$NAME" /tmp/pkg.tar >/dev/null

docker exec spike-server cat /var/lib/rancher/k3s/server/tls/server-ca.crt > "$WORK/ca.crt"
SERVER=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' spike-server)
AGENT=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' spike-agent)
ACCEPT="Accept: application/vnd.oci.image.index.v1+json,application/vnd.oci.image.manifest.v1+json,application/vnd.docker.distribution.manifest.v2+json"
code() { curl -s -o /dev/null -w "%{http_code}" --cacert "$WORK/ca.crt" -H "$ACCEPT" "$@"; }

echo "manifest by tag, with ns, no client certificate:  $(code "https://$SERVER:6443/v2/acme/nop/manifests/v0.5.0?ns=$NS")"
echo "manifest by tag, without ns (standard OCI path):  $(code "https://$SERVER:6443/v2/acme/nop/manifests/v0.5.0")"
echo "manifest, original registry in path, without ns:  $(code "https://$SERVER:6443/v2/$NS/acme/nop/manifests/v0.5.0")"
echo "tag list, with ns:                                $(code "https://$SERVER:6443/v2/acme/nop/tags/list?ns=$NS")"
echo "manifest by tag from the agent node, with ns:     $(code "https://$AGENT:6443/v2/acme/nop/manifests/v0.5.0?ns=$NS")"
DIGEST=$(curl -s -I --cacert "$WORK/ca.crt" -H "$ACCEPT" "https://$SERVER:6443/v2/acme/nop/manifests/v0.5.0?ns=$NS" | awk -F': ' 'tolower($1)=="docker-content-digest"{print $2}' | tr -d '\r')
echo "manifest by digest from the agent node, with ns:  $(code "https://$AGENT:6443/v2/acme/nop/manifests/$DIGEST?ns=$NS")"
SSL_CERT_FILE="$WORK/ca.crt" crane manifest "$SERVER:6443/acme/nop:v0.5.0" >/dev/null 2>&1 \
  && echo "crane (Crossplane's OCI library):                works" \
  || echo "crane (Crossplane's OCI library):                fails (no ns)"
