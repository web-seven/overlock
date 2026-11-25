#!/bin/sh
# Links the bundled Overlock image tarballs into the K3s airgap images
# directory, then starts K3s. Linking at startup keeps the bundle available when
# the data directory (/var/lib/rancher/k3s by default) is a mounted volume.
set -e

data_dir="${K3S_DATA_DIR:-/var/lib/rancher/k3s}"
prev=""
for arg in "$@"; do
    case "$arg" in
        --data-dir=*|-d=*) data_dir="${arg#*=}" ;;
    esac
    case "$prev" in
        --data-dir|-d) data_dir="$arg" ;;
    esac
    prev="$arg"
done

images_dir="$data_dir/agent/images"
mkdir -p "$images_dir"
for f in /opt/overlock/images/*; do
    [ -e "$f" ] || continue
    ln -sf "$f" "$images_dir/$(basename "$f")"
done

exec /bin/k3s "$@"
