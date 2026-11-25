#!/bin/sh
# Entrypoint of the Overlock K3s image.
#
# - Installs the Overlock K3s server defaults for "server".
# - Links the bundled image tarballs into the K3s airgap images directory, so
#   they are also available when the data directory is a mounted volume.
# - For "server", bootstraps the Overlock operator and an Environment through
#   the K3s auto-deploy manifests directory; the operator installs Crossplane
#   and the tools. The Environment comes from a mounted file
#   (OVERLOCK_ENVIRONMENT_FILE) or a bundled profile (OVERLOCK_PROFILE).
# - Reports environment progress in the container log until it is ready.
# - Optionally writes kubeconfigs for the host (OVERLOCK_KUBECONFIG_OUTPUT) and
#   for other containers (OVERLOCK_KUBECONFIG_SHARED_OUTPUT).
set -e

OVERLOCK_BOOTSTRAP="${OVERLOCK_BOOTSTRAP:-true}"
OVERLOCK_PROFILE="${OVERLOCK_PROFILE:-default}"
OVERLOCK_ENVIRONMENT_NAME="${OVERLOCK_ENVIRONMENT_NAME:-default}"
OVERLOCK_ENVIRONMENT_FILE="${OVERLOCK_ENVIRONMENT_FILE:-/etc/overlock/environment.yaml}"
OVERLOCK_KUBECONFIG_OUTPUT="${OVERLOCK_KUBECONFIG_OUTPUT:-}"
OVERLOCK_KUBECONFIG_SHARED_OUTPUT="${OVERLOCK_KUBECONFIG_SHARED_OUTPUT:-}"
OVERLOCK_KUBECONFIG_SHARED_SERVER="${OVERLOCK_KUBECONFIG_SHARED_SERVER:-}"
state_dir=/run/overlock

log() {
    echo "overlock: $*" >&2
}

data_dir="${K3S_DATA_DIR:-/var/lib/rancher/k3s}"
command=""
prev=""
for arg in "$@"; do
    case "$arg" in
        --data-dir=*|-d=*) data_dir="${arg#*=}" ;;
        -*) ;;
        *) case "$prev" in
               --data-dir|-d) data_dir="$arg" ;;
               *) [ -z "$command" ] && command="$arg" ;;
           esac ;;
    esac
    prev="$arg"
done

images_dir="$data_dir/agent/images"
mkdir -p "$images_dir"
for f in /opt/overlock/images/*; do
    [ -e "$f" ] || continue
    ln -sf "$f" "$images_dir/$(basename "$f")"
done

# environment_name prints metadata.name of an Environment manifest.
environment_name() {
    awk '
        /^metadata:/ { in_meta = 1; next }
        in_meta && /^[^ ]/ { in_meta = 0 }
        in_meta && /^  name:/ { sub(/^  name:[ ]*/, ""); gsub(/["'\'']/, ""); print; exit }
    ' "$1"
}

bootstrap() {
    manifests_dir="$data_dir/server/manifests"
    mkdir -p "$manifests_dir" "$state_dir"
    rm -f "$state_dir/environment"

    if [ "$OVERLOCK_BOOTSTRAP" != "true" ]; then
        log "bootstrap disabled (OVERLOCK_BOOTSTRAP=$OVERLOCK_BOOTSTRAP)"
        rm -f "$manifests_dir/overlock-operator.yaml" "$manifests_dir/overlock-environment.yaml"
        return
    fi

    if [ -f "$OVERLOCK_ENVIRONMENT_FILE" ]; then
        if ! grep -q '^kind: Environment' "$OVERLOCK_ENVIRONMENT_FILE"; then
            log "$OVERLOCK_ENVIRONMENT_FILE is not an Environment resource (generate one with: overlock env manifest)"
            exit 1
        fi
        name="$(environment_name "$OVERLOCK_ENVIRONMENT_FILE")"
        if [ -z "$name" ]; then
            log "$OVERLOCK_ENVIRONMENT_FILE has no metadata.name"
            exit 1
        fi
        cp "$OVERLOCK_ENVIRONMENT_FILE" "$manifests_dir/overlock-environment.yaml.tmp"
        log "environment '$name' from $OVERLOCK_ENVIRONMENT_FILE"
    else
        profile="/opt/overlock/profiles/$OVERLOCK_PROFILE.yaml"
        if [ ! -f "$profile" ]; then
            available=""
            for p in /opt/overlock/profiles/*.yaml; do
                p="${p##*/}"
                available="$available ${p%.yaml}"
            done
            log "unknown profile '$OVERLOCK_PROFILE', available:$available"
            exit 1
        fi
        name="$OVERLOCK_ENVIRONMENT_NAME"
        sed "s/OVERLOCK_ENVIRONMENT_NAME/$name/" "$profile" > "$manifests_dir/overlock-environment.yaml.tmp"
        log "environment '$name' from profile '$OVERLOCK_PROFILE'"
    fi

    # Written on every start, so a new image upgrades the operator and the
    # mounted file or profile stays the source of the Environment.
    cp /opt/overlock/manifests/operator.yaml "$manifests_dir/overlock-operator.yaml.tmp"
    mv "$manifests_dir/overlock-operator.yaml.tmp" "$manifests_dir/overlock-operator.yaml"
    mv "$manifests_dir/overlock-environment.yaml.tmp" "$manifests_dir/overlock-environment.yaml"
    echo "$name" > "$state_dir/environment"
}

# report logs environment progress until it is ready.
report() {
    last=""
    while :; do
        status="$(/bin/overlock-healthcheck --status 2>/dev/null || true)"
        if [ -n "$status" ] && [ "$status" != "$last" ]; then
            log "$status"
            last="$status"
        fi
        if /bin/overlock-healthcheck >/dev/null 2>&1; then
            return
        fi
        sleep 5
    done
}

# write_kubeconfig writes the server kubeconfig to $1 with the K3s "default"
# names replaced by the context name and, when $2 is set, the server address
# replaced by $2. The file is owned like its directory.
write_kubeconfig() {
    out="$1"
    server="$2"
    mkdir -p "${out%/*}"
    sed -E "s/^( *(- )?(name|cluster|user|current-context)): default$/\1: ${context}/" "$src" > "$out.tmp"
    if [ -n "$server" ]; then
        sed -i -E "s#^( *server: ).*#\1${server}#" "$out.tmp"
    fi
    chmod 644 "$out.tmp"
    chown "$(stat -c %u:%g "${out%/*}")" "$out.tmp" || true
    mv "$out.tmp" "$out"
    log "kubeconfig written to $out (context '$context'${server:+, server $server})"
}

# export_kubeconfig writes kubeconfigs once the API is ready:
# - OVERLOCK_KUBECONFIG_OUTPUT: for the host (server https://127.0.0.1:<port>)
# - OVERLOCK_KUBECONFIG_SHARED_OUTPUT: for other containers, with the server
#   address OVERLOCK_KUBECONFIG_SHARED_SERVER (e.g. https://k3s:6443)
export_kubeconfig() {
    src=/etc/rancher/k3s/k3s.yaml
    context="${OVERLOCK_KUBECONFIG_CONTEXT:-overlock-${OVERLOCK_ENVIRONMENT_NAME}}"
    until [ -s "$src" ] && KUBECONFIG="$src" kubectl get --raw /readyz >/dev/null 2>&1; do
        sleep 2
    done
    if [ -n "$OVERLOCK_KUBECONFIG_OUTPUT" ]; then
        write_kubeconfig "$OVERLOCK_KUBECONFIG_OUTPUT" ""
    fi
    if [ -n "$OVERLOCK_KUBECONFIG_SHARED_OUTPUT" ]; then
        write_kubeconfig "$OVERLOCK_KUBECONFIG_SHARED_OUTPUT" "$OVERLOCK_KUBECONFIG_SHARED_SERVER"
    fi
}

# Server defaults (e.g. disabled Traefik) apply to servers only: agents read
# the same config directory and reject server options.
server_config=/etc/rancher/k3s/config.yaml.d/00-overlock.yaml
if [ "$command" = "server" ]; then
    mkdir -p "${server_config%/*}"
    cp /opt/overlock/server-config.yaml "$server_config"
else
    rm -f "$server_config"
fi

if [ "$command" = "server" ]; then
    bootstrap
    if [ -f "$state_dir/environment" ]; then
        report &
    fi
    if [ -n "$OVERLOCK_KUBECONFIG_OUTPUT$OVERLOCK_KUBECONFIG_SHARED_OUTPUT" ]; then
        export_kubeconfig &
    fi
fi

exec /bin/k3s "$@"
