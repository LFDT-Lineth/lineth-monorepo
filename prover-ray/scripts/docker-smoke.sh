#!/usr/bin/env bash
# Docker smoke test for the prover-ray image (reusable by CI, plan Stage 6).
#
# Checks two things:
#   1. dev-mock turns a dropped request file into a valid response file.
#   2. the dev-zkvm artifacts (native runner + guest ELF) are present and the
#      runner's shared libraries all resolve inside the image.
#
# Usage: scripts/docker-smoke.sh [image]
set -euo pipefail

DOCKER="${DOCKER:-docker}"
IMAGE="${1:-consensys/linea-prover-ray:dev}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FIXTURE="$SCRIPT_DIR/../backend/jobadapter/testdata/request_single_block.json"
CONTAINER="prover-ray-smoke-$$"
WORK="$(mktemp -d)"

cleanup() {
    $DOCKER rm -f "$CONTAINER" >/dev/null 2>&1 || true
    rm -rf "$WORK"
}
trap cleanup EXIT

mkdir -p "$WORK/requests"
cp "$FIXTURE" "$WORK/requests/req.json"

echo "==> dev-mock: turning a request into a response"
# Make the bind mount writable by the container. Rootless podman remaps the host
# UID, so keep-id passes it through; docker just runs as the given user.
run_opts=(--user "$(id -u):$(id -g)")
case "$DOCKER" in
    *podman*) run_opts=(--userns=keep-id) ;;
esac
$DOCKER run -d --name "$CONTAINER" \
    "${run_opts[@]}" \
    -v "$WORK:/data" \
    "$IMAGE" \
    --requests-dir /data --mode dev-mock --prover-version smoke >/dev/null

for _ in $(seq 1 30); do
    [ -f "$WORK/responses/req.json" ] && break
    sleep 1
done

if [ ! -f "$WORK/responses/req.json" ]; then
    echo "FAIL: no response file produced"
    $DOCKER logs "$CONTAINER" || true
    exit 1
fi
if ! grep -q '"proverVersion": "smoke-dev-mock"' "$WORK/responses/req.json"; then
    echo "FAIL: unexpected response body"
    cat "$WORK/responses/req.json"
    exit 1
fi
echo "    ok: response written with proverVersion smoke-dev-mock"

echo "==> dev-zkvm: artifacts present and linkable"
$DOCKER run --rm --entrypoint sh "$IMAGE" -c '
    set -e
    test -x /opt/linea/prover-ray/l2-execution-runner
    test -f /opt/linea/prover-ray/evm_execution_guest
    if ldd /opt/linea/prover-ray/l2-execution-runner | grep -q "not found"; then
        echo "unresolved shared libraries:"
        ldd /opt/linea/prover-ray/l2-execution-runner | grep "not found"
        exit 1
    fi
'
echo "    ok: native runner + guest ELF present, all libraries resolve"

echo "SMOKE PASSED"
