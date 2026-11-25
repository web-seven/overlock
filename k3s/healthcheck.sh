#!/bin/sh
# Health check of the Overlock K3s image.
#
# Exits 0 when the container is ready:
# - server with an Overlock Environment: the Environment is Ready
# - server without bootstrap: the Kubernetes API is ready
# - agent: the kubelet is healthy
#
# With --status, prints a one-line description of the current state.
kubeconfig=/etc/rancher/k3s/k3s.yaml
state_file=/run/overlock/environment

status() {
    [ "$1" = "--status" ] && echo "$2"
}

if [ ! -f "$kubeconfig" ]; then
    if wget -q -T 3 -O /dev/null http://127.0.0.1:10248/healthz 2>/dev/null; then
        status "$1" "kubelet ready"
        exit 0
    fi
    status "$1" "waiting for K3s"
    exit 1
fi

export KUBECONFIG="$kubeconfig"
if ! kubectl get --raw /readyz >/dev/null 2>&1; then
    status "$1" "waiting for the Kubernetes API"
    exit 1
fi

if [ ! -f "$state_file" ]; then
    status "$1" "Kubernetes API ready"
    exit 0
fi

name="$(cat "$state_file")"
ready="$(kubectl get environments.overlock.app "$name" -o 'jsonpath={.status.conditions[?(@.type=="Ready")].status}|{.status.conditions[?(@.type=="Ready")].reason}|{.status.conditions[?(@.type=="Ready")].message}' 2>/dev/null)"
case "$ready" in
    ""|"||")
        status "$1" "environment '$name': waiting for the operator"
        exit 1 ;;
    True\|*)
        status "$1" "environment '$name' is ready"
        exit 0 ;;
    *)
        reason="$(echo "$ready" | cut -d'|' -f2)"
        message="$(echo "$ready" | cut -d'|' -f3-)"
        status "$1" "environment '$name': $reason: $message"
        exit 1 ;;
esac
