#!/bin/sh
# Optional Go test executor for hosts with nonstandard root ownership.
# All fixtures are synthetic. The test container has no network or host secrets.
set -eu
binary=$1
shift
image=ubuntu@sha256:fe81560990fae86dc4743202b2cb4d3df98d0bcf949173cecef15ce1127ef852

if [ "${WECOM_CHAT_TEST_ROOT:-0}" = 1 ]; then
    # Only a temporary Go test executable is mounted for the privileged setup.
    # The fixture immediately starts its worker with a different, unprivileged UID.
    chmod 755 "$binary"
    exec env -u DOCKER_HOST -u DOCKER_CONTEXT -u DOCKER_TLS \
        -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH \
        docker --host=unix:///var/run/docker.sock run --rm --network=none \
        --cap-drop=ALL --cap-add=CHOWN --cap-add=SETUID --cap-add=SETGID \
        --security-opt=no-new-privileges --user=0:0 --read-only \
        --tmpfs /tmp:rw,exec,mode=1777 \
        --mount "type=bind,src=$binary,dst=/probe,readonly" \
        --workdir /tmp "$image" /probe "$@"
fi

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
go_root=$(GOFLAGS= go env GOROOT)
mod_cache=$(GOFLAGS= go env GOMODCACHE)
case "$PWD" in "$repo_dir"*) work_dir="/src${PWD#"$repo_dir"}";; *) exit 2;; esac
exec env -u DOCKER_HOST -u DOCKER_CONTEXT -u DOCKER_TLS \
    -u DOCKER_TLS_VERIFY -u DOCKER_CERT_PATH \
    docker --host=unix:///var/run/docker.sock run --rm --network=none \
    --cap-drop=ALL --security-opt=no-new-privileges \
    --user="$(id -u):$(id -g)" --read-only --tmpfs /tmp:rw,exec,mode=1777 \
    --mount "type=bind,src=$binary,dst=/probe,readonly" \
    --mount "type=bind,src=$repo_dir,dst=/src,readonly" \
    --mount "type=bind,src=$go_root,dst=/goroot,readonly" \
    --mount "type=bind,src=$mod_cache,dst=/gomod,readonly" \
    -e PATH=/goroot/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin \
    -e GOROOT=/goroot -e GOPATH=/tmp/gopath -e GOMODCACHE=/gomod \
    -e GOCACHE=/tmp/gocache -e GOWORK=off -e CGO_ENABLED=0 -e GOPROXY=off \
    --workdir "$work_dir" "$image" /probe "$@"
