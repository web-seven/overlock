#!/bin/bash
# Start a k3s cluster across wg-bridge: server + local agent + remote agent.
# Run setup.sh first to establish the WireGuard bridge.
#
# Usage:
#   ./test-cluster.sh <user>@<remote-host> [--key <ssh-key>]

set -euo pipefail

SSH_TARGET="${1:?Usage: $0 <user>@<remote-host> [--key <ssh-key>]}"
shift
SSH_KEY=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --key) SSH_KEY="$2"; shift 2 ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

SSH_OPTS="-o StrictHostKeyChecking=no -o BatchMode=yes"
[[ -n "$SSH_KEY" ]] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"

if [[ -z "$SSH_KEY" ]]; then
    for candidate in "$HOME/.ssh/id_ed25519" "$HOME/.ssh/id_rsa"; do
        [[ -f "$candidate" ]] && SSH_KEY="$candidate" && SSH_OPTS="$SSH_OPTS -i $SSH_KEY" && break
    done
fi

ssh_run() { ssh $SSH_OPTS "$SSH_TARGET" "$@"; }

# ── Config ───────────────────────────────────────────────────────────────────
DOCKER_NET=wg-bridge
SERVER_IP=10.201.0.2
LOCAL_AGENT_IP=10.201.0.3
REMOTE_AGENT_IP=10.202.0.2
K3S_IMAGE=rancher/k3s:latest

# ── Cleanup any previous run ─────────────────────────────────────────────────
docker rm -f k3s-server k3s-agent-local 2>/dev/null || true
ssh_run "docker rm -f k3s-agent-remote 2>/dev/null" || true

# ── k3s server ───────────────────────────────────────────────────────────────
echo "==> Starting k3s server ($SERVER_IP)..."
docker run -d --name k3s-server --hostname k3s-server \
    --privileged \
    --tmpfs /run --tmpfs /var/run \
    --network $DOCKER_NET --ip $SERVER_IP \
    $K3S_IMAGE server \
    --tls-san $SERVER_IP \
    --node-ip $SERVER_IP \
    --flannel-iface eth0

echo "==> Waiting for k3s server to be ready..."
for i in $(seq 1 60); do
    docker exec k3s-server kubectl get nodes &>/dev/null && break
    [ "$i" -eq 60 ] && echo "Timeout waiting for k3s server." && exit 1
    sleep 3
done
echo "    Server ready."

TOKEN=$(docker exec k3s-server cat /var/lib/rancher/k3s/server/node-token)

# ── Local agent ──────────────────────────────────────────────────────────────
echo "==> Starting local k3s agent ($LOCAL_AGENT_IP)..."
docker run -d --name k3s-agent-local --hostname k3s-agent-local \
    --privileged \
    --tmpfs /run --tmpfs /var/run \
    --network $DOCKER_NET --ip $LOCAL_AGENT_IP \
    $K3S_IMAGE agent \
    --server https://$SERVER_IP:6443 \
    --token "$TOKEN" \
    --node-ip $LOCAL_AGENT_IP \
    --flannel-iface eth0

# ── Remote agent ─────────────────────────────────────────────────────────────
echo "==> Starting remote k3s agent ($REMOTE_AGENT_IP)..."
ssh_run "docker run -d --name k3s-agent-remote --hostname k3s-agent-remote \
    --privileged \
    --tmpfs /run --tmpfs /var/run \
    --network $DOCKER_NET --ip $REMOTE_AGENT_IP \
    $K3S_IMAGE agent \
    --server https://$SERVER_IP:6443 \
    --token '$TOKEN' \
    --node-ip $REMOTE_AGENT_IP \
    --flannel-iface eth0"

# ── Wait for all 3 nodes Ready ────────────────────────────────────────────────
echo "==> Waiting for all nodes to be Ready..."
for i in $(seq 1 90); do
    READY=$(docker exec k3s-server kubectl get nodes --no-headers 2>/dev/null | grep -c " Ready" || true)
    [ "$READY" -eq 3 ] && break
    [ "$i" -eq 90 ] && echo "Timeout. Current state:" && docker exec k3s-server kubectl get nodes && exit 1
    sleep 3
done

echo ""
docker exec k3s-server kubectl get nodes -o wide

# ── Pod-to-pod connectivity test ─────────────────────────────────────────────
echo ""
echo "==> Deploying test pods..."

# One pod pinned to each node
docker exec -i k3s-server kubectl apply -f - <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: test-local
spec:
  nodeName: k3s-server
  containers:
  - name: alpine
    image: alpine
    command: ["sh", "-c", "echo 'hello from local node'; sleep 300"]
---
apiVersion: v1
kind: Pod
metadata:
  name: test-remote
spec:
  nodeName: k3s-agent-remote
  containers:
  - name: alpine
    image: alpine
    command: ["sh", "-c", "echo 'hello from remote node'; sleep 300"]
EOF

echo "==> Waiting for test pods to be Running..."
for i in $(seq 1 60); do
    RUNNING=$(docker exec k3s-server kubectl get pods --no-headers 2>/dev/null | grep -c " Running" || true)
    [ "$RUNNING" -eq 2 ] && break
    [ "$i" -eq 60 ] && echo "Timeout. Pod state:" && docker exec k3s-server kubectl get pods && exit 1
    sleep 3
done

LOCAL_POD_IP=$(docker exec k3s-server kubectl get pod test-local -o jsonpath='{.status.podIP}')
REMOTE_POD_IP=$(docker exec k3s-server kubectl get pod test-remote -o jsonpath='{.status.podIP}')
echo "    test-local  pod IP: $LOCAL_POD_IP  (node: k3s-server)"
echo "    test-remote pod IP: $REMOTE_POD_IP (node: k3s-agent-remote)"

echo ""
echo "==> Testing pod connectivity..."

if docker exec k3s-server kubectl exec test-local -- ping -c 3 -W 3 "$REMOTE_POD_IP" &>/dev/null; then
    echo "    OK  local  -> remote ($LOCAL_POD_IP -> $REMOTE_POD_IP)"
else
    echo "    FAIL local  -> remote ($LOCAL_POD_IP -> $REMOTE_POD_IP)"
fi

if docker exec k3s-server kubectl exec test-remote -- ping -c 3 -W 3 "$LOCAL_POD_IP" &>/dev/null; then
    echo "    OK  remote -> local  ($REMOTE_POD_IP -> $LOCAL_POD_IP)"
else
    echo "    FAIL remote -> local  ($REMOTE_POD_IP -> $LOCAL_POD_IP)"
fi

# ── Log streaming test ───────────────────────────────────────────────────────
echo ""
echo "==> Testing kubectl logs (API server -> kubelet on each node)..."

LOCAL_LOG=$(docker exec k3s-server kubectl logs test-local 2>/dev/null)
if echo "$LOCAL_LOG" | grep -q "hello from local node"; then
    echo "    OK  logs from local  node: $LOCAL_LOG"
else
    echo "    FAIL logs from local  node (got: '$LOCAL_LOG')"
fi

REMOTE_LOG=$(docker exec k3s-server kubectl logs test-remote 2>/dev/null)
if echo "$REMOTE_LOG" | grep -q "hello from remote node"; then
    echo "    OK  logs from remote node: $REMOTE_LOG"
else
    echo "    FAIL logs from remote node (got: '$REMOTE_LOG')"
fi

docker exec k3s-server kubectl delete pod test-local test-remote --wait=false 2>/dev/null || true

echo ""
echo "Cluster is up. Use:"
echo "  docker exec k3s-server kubectl get pods -A"
echo ""
echo "Teardown: ./teardown-cluster.sh $SSH_TARGET${SSH_KEY:+ --key $SSH_KEY}"
