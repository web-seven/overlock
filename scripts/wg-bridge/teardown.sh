#!/bin/bash
# Tear down WireGuard bridge on local and optionally remote.
# Usage: ./teardown.sh [<user>@<remote-host>] [--key <ssh-key>]

set -euo pipefail

SSH_TARGET="${1:-}"
shift 2>/dev/null || true
SSH_KEY=""
while [[ $# -gt 0 ]]; do
    case "$1" in
        --key) SSH_KEY="$2"; shift 2 ;;
        *) shift ;;
    esac
done

if [[ -z "$SSH_KEY" ]]; then
    for candidate in "$HOME/.ssh/id_ed25519" "$HOME/.ssh/id_rsa"; do
        [[ -f "$candidate" ]] && SSH_KEY="$candidate" && break
    done
fi

NETIMG="docker:cli"

teardown_local() {
    docker run --rm -i --privileged --network host \
        -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock \
        $NETIMG sh <<LOCAL
apk add -q iproute2 iptables nftables 2>/dev/null
nft delete rule ip raw PREROUTING iifname "wg0" ip daddr 10.201.0.0/24 return 2>/dev/null || true
nft delete rule ip nat nat_POST_public_allow oifname "wg0" return 2>/dev/null || true
ip link del wg0 2>/dev/null && echo "wg0 removed" || true
ip route del 10.202.0.0/24 2>/dev/null || true
docker ps -q --filter network=wg-bridge | xargs -r docker rm -f
docker network rm wg-bridge 2>/dev/null && echo "Docker network wg-bridge removed" || true
rm -f /tmp/wg-local.key /tmp/wg-remote.key /tmp/wg-local.pub /tmp/wg-remote.pub
LOCAL
    echo "Local teardown done"
}

teardown_local

if [[ -n "$SSH_TARGET" ]]; then
    echo "==> Tearing down remote ($SSH_TARGET)..."
    SSH_OPTS="-o StrictHostKeyChecking=no -o BatchMode=yes"
    [[ -n "$SSH_KEY" ]] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"
    ssh $SSH_OPTS "$SSH_TARGET" docker run --rm -i --privileged --network host \
        -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock \
        $NETIMG sh <<REMOTE
apk add -q iproute2 iptables nftables 2>/dev/null
nft delete rule ip raw PREROUTING iifname "wg0" ip daddr 10.202.0.0/24 return 2>/dev/null || true
ip link del wg0 2>/dev/null && echo "wg0 removed" || true
ip route del 10.201.0.0/24 2>/dev/null || true
docker ps -q --filter network=wg-bridge | xargs -r docker rm -f
docker network rm wg-bridge 2>/dev/null && echo "Docker network wg-bridge removed" || true
rm -f /tmp/wg-remote.key /tmp/wg-remote.pub
echo "Remote teardown done"
REMOTE
fi
