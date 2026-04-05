#!/bin/bash
# Tear down k3s test cluster (containers only; leaves wg-bridge intact).
# Usage: ./teardown-cluster.sh [<user>@<remote-host>] [--key <ssh-key>]

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

docker rm -f k3s-server k3s-agent-local 2>/dev/null && echo "Local k3s containers removed" || true

if [[ -n "$SSH_TARGET" ]]; then
    SSH_OPTS="-o StrictHostKeyChecking=no -o BatchMode=yes"
    [[ -n "$SSH_KEY" ]] && SSH_OPTS="$SSH_OPTS -i $SSH_KEY"
    ssh $SSH_OPTS "$SSH_TARGET" "docker rm -f k3s-agent-remote 2>/dev/null && echo 'Remote k3s container removed'" || true
fi

echo "Cluster teardown done."
