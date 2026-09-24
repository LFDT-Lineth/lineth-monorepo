#!/usr/bin/env bash
# Docker smoke test for the prover-ray image (reusable by CI).
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
cat > "$WORK/config.toml" <<'TOML'
version = "smoke"
log_level = 4
[execution]
prover_mode = "dev-mock"
requests_root_dir = "/data"
TOML

echo "==> dev-mock: turning a request into a response"
# Make the bind mount readable/writable by the container as the caller.
# Rootless podman already maps container-root -> the host user, so no flags are
# needed (and --user/keep-id fight that). Rootful docker needs --user so the
# files it writes are owned by the caller, not root.
run_opts=(--user "$(id -u):$(id -g)")
case "$DOCKER" in
    *podman*) run_opts=() ;;
esac
# The image's default CMD runs the adapter, expanding CONFIG_FILE and WORKER_ID.
$DOCKER run -d --name "$CONTAINER" \
    "${run_opts[@]}" \
    -e CONFIG_FILE=/data/config.toml \
    -e WORKER_ID=smoke \
    -v "$WORK:/data" \
    "$IMAGE" >/dev/null

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

echo "==> dev-zkvm: native runner executes (exercises glibc/mcl/secp256k1/crypto)"
# Run the bundled native runner directly on a fixture and check it emits a
# 34-byte 0x0003 commitment, proving every shared library the runner links
# actually resolves in the image.
FIXTURE_DIR="$SCRIPT_DIR/../../riscv-guests/l2-execution/test/testdata"
if ! $DOCKER run --rm \
        --entrypoint /opt/linea/prover-ray/l2-execution-runner \
        -v "$FIXTURE_DIR:/in:ro" \
        "$IMAGE" /in/stateless_input.ssz --ssz > "$WORK/commitment.bin" 2>/dev/null; then
    echo "FAIL: native runner failed to run (missing library or exec error)"
    exit 1
fi
sz=$(wc -c < "$WORK/commitment.bin")
prefix=$(head -c 2 "$WORK/commitment.bin" | od -An -tx1 | tr -d ' \n')
if [ "$sz" -ne 34 ] || [ "$prefix" != "0003" ]; then
    echo "FAIL: native runner output is not a 34-byte 0x0003 commitment (size=$sz prefix=$prefix)"
    exit 1
fi
echo "    ok: native runner produced a valid 0x0003 commitment"

echo "SMOKE PASSED"
