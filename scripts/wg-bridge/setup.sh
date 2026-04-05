#!/bin/bash
# WireGuard bridge — no sudo required; privileged ops run inside Docker with NET_ADMIN.
# Local machine can be behind NAT; only the remote needs UDP 51820 open.
#
# Usage:
#   ./setup.sh <user>@<remote-host> [--key <ssh-key>]
#
# Requirements: docker (user must be in docker group)

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

# ── Network config ───────────────────────────────────────────────────────────
WG_IFACE=wg0
WG_PORT=51820
LOCAL_WG_ADDR=10.200.0.1
REMOTE_WG_ADDR=10.200.0.2
LOCAL_DOCKER_SUBNET=10.201.0.0/24
LOCAL_DOCKER_GW=10.201.0.1
REMOTE_DOCKER_SUBNET=10.202.0.0/24
REMOTE_DOCKER_GW=10.202.0.1
DOCKER_NET=wg-bridge
LOCAL_KEY=/tmp/wg-local.key
REMOTE_KEY=/tmp/wg-remote.key
REMOTE_HOST="${SSH_TARGET#*@}"

# docker:cli = alpine + docker binary; we add wireguard/net tools via apk
NETIMG="docker:cli"
NETPKGS="wireguard-tools iproute2 iptables nftables"

# ── Generate local WireGuard keys ────────────────────────────────────────────
echo "==> Generating local WireGuard keys..."
LOCAL_PUBKEY=$(docker run --rm --privileged --network host -v /tmp:/tmp \
    $NETIMG sh -c "apk add -q wireguard-tools 2>/dev/null && umask 077 && wg genkey > $LOCAL_KEY && wg pubkey < $LOCAL_KEY")
echo "    Local public key: $LOCAL_PUBKEY"

# ── Set up WireGuard on remote ───────────────────────────────────────────────
echo "==> Setting up WireGuard on remote ($SSH_TARGET)..."

ssh_run docker run --rm -i --privileged --network host \
    -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock \
    $NETIMG sh <<REMOTE
apk add -q $NETPKGS 2>/dev/null

umask 077
wg genkey > $REMOTE_KEY
wg pubkey < $REMOTE_KEY > /tmp/wg-remote.pub
chmod 644 /tmp/wg-remote.pub

ip link del $WG_IFACE 2>/dev/null || true
ip link add $WG_IFACE type wireguard
ip addr add $REMOTE_WG_ADDR/24 dev $WG_IFACE
wg set $WG_IFACE \
    private-key $REMOTE_KEY \
    listen-port $WG_PORT \
    peer $LOCAL_PUBKEY \
        allowed-ips $LOCAL_WG_ADDR/32,$LOCAL_DOCKER_SUBNET
ip link set $WG_IFACE up
echo 1 > /proc/sys/net/ipv4/ip_forward

# Docker network with masquerade disabled — preserves real container IPs through WireGuard
docker network rm $DOCKER_NET 2>/dev/null || true
docker network create --driver bridge \
    --subnet $REMOTE_DOCKER_SUBNET \
    --gateway $REMOTE_DOCKER_GW \
    --opt com.docker.network.bridge.enable_ip_masquerade=false \
    $DOCKER_NET

ip route replace $LOCAL_DOCKER_SUBNET via $LOCAL_WG_ADDR

BR=\$(docker network inspect $DOCKER_NET --format "{{.Id}}" | head -c 12)
iptables -I FORWARD -i br-\$BR -o $WG_IFACE -j ACCEPT 2>/dev/null || true
iptables -I FORWARD -i $WG_IFACE -o br-\$BR -j ACCEPT 2>/dev/null || true
# Bypass Docker's raw PREROUTING isolation rule for WireGuard traffic
nft insert rule ip raw PREROUTING iifname $WG_IFACE ip daddr $REMOTE_DOCKER_SUBNET return 2>/dev/null || true
REMOTE

REMOTE_PUBKEY=$(ssh_run "cat /tmp/wg-remote.pub")
echo "    Remote public key: $REMOTE_PUBKEY"

# ── Set up WireGuard locally ─────────────────────────────────────────────────
echo "==> Setting up WireGuard locally..."

docker run --rm -i --privileged --network host \
    -v /tmp:/tmp -v /var/run/docker.sock:/var/run/docker.sock \
    $NETIMG sh <<LOCAL
apk add -q $NETPKGS 2>/dev/null

ip link del $WG_IFACE 2>/dev/null || true
ip link add $WG_IFACE type wireguard
ip addr add $LOCAL_WG_ADDR/24 dev $WG_IFACE
wg set $WG_IFACE \
    private-key $LOCAL_KEY \
    peer $REMOTE_PUBKEY \
        endpoint $REMOTE_HOST:$WG_PORT \
        allowed-ips $REMOTE_WG_ADDR/32,$REMOTE_DOCKER_SUBNET \
        persistent-keepalive 25
ip link set $WG_IFACE up
echo 1 > /proc/sys/net/ipv4/ip_forward

# Docker network with masquerade disabled
docker network rm $DOCKER_NET 2>/dev/null || true
docker network create --driver bridge \
    --subnet $LOCAL_DOCKER_SUBNET \
    --gateway $LOCAL_DOCKER_GW \
    --opt com.docker.network.bridge.enable_ip_masquerade=false \
    $DOCKER_NET

ip route replace $REMOTE_DOCKER_SUBNET via $REMOTE_WG_ADDR

BR=\$(docker network inspect $DOCKER_NET --format "{{.Id}}" | head -c 12)
iptables -I FORWARD -i br-\$BR -o $WG_IFACE -j ACCEPT 2>/dev/null || true
iptables -I FORWARD -i $WG_IFACE -o br-\$BR -j ACCEPT 2>/dev/null || true
# Exempt wg0 from firewalld's blanket masquerade (if firewalld is running)
nft insert rule ip nat nat_POST_public_allow oifname $WG_IFACE return 2>/dev/null || true
# Bypass Docker's raw PREROUTING isolation rule for WireGuard traffic
nft insert rule ip raw PREROUTING iifname $WG_IFACE ip daddr $LOCAL_DOCKER_SUBNET return 2>/dev/null || true
LOCAL

# ── Verify tunnel ────────────────────────────────────────────────────────────
echo "==> Verifying WireGuard tunnel (host-to-host)..."
sleep 2
if docker run --rm --network host $NETIMG ping -c 2 -W 3 "$REMOTE_WG_ADDR" &>/dev/null; then
    echo "    Tunnel OK — $LOCAL_WG_ADDR <-> $REMOTE_WG_ADDR"
else
    echo "    Warning: tunnel ping failed. Check firewall allows UDP $WG_PORT."
fi

# ── Verify container-to-container ────────────────────────────────────────────
echo "==> Verifying container-to-container connectivity..."
LOCAL_TEST_IP=10.201.0.10
REMOTE_TEST_IP=10.202.0.10

ssh_run "docker rm -f wg-bridge-test 2>/dev/null; docker run -d --name wg-bridge-test --network $DOCKER_NET --ip $REMOTE_TEST_IP alpine sleep 30"
docker rm -f wg-bridge-test 2>/dev/null || true
docker run -d --name wg-bridge-test --network "$DOCKER_NET" --ip "$LOCAL_TEST_IP" alpine sleep 30

if docker exec wg-bridge-test ping -c 3 -W 3 "$REMOTE_TEST_IP" &>/dev/null; then
    echo "    Container OK — $LOCAL_TEST_IP -> $REMOTE_TEST_IP"
else
    echo "    Warning: container ping $LOCAL_TEST_IP -> $REMOTE_TEST_IP failed."
fi

if ssh_run "docker exec wg-bridge-test ping -c 3 -W 3 $LOCAL_TEST_IP" &>/dev/null; then
    echo "    Container OK — $REMOTE_TEST_IP -> $LOCAL_TEST_IP"
else
    echo "    Warning: container ping $REMOTE_TEST_IP -> $LOCAL_TEST_IP failed."
fi

docker rm -f wg-bridge-test 2>/dev/null || true
ssh_run "docker rm -f wg-bridge-test 2>/dev/null" || true

echo ""
echo "Done. Network layout:"
echo "  Local  containers: $LOCAL_DOCKER_SUBNET  (Docker network: $DOCKER_NET)"
echo "  Remote containers: $REMOTE_DOCKER_SUBNET (Docker network: $DOCKER_NET)"
echo ""
echo "Teardown: ./teardown.sh $SSH_TARGET${SSH_KEY:+ --key $SSH_KEY}"
