#!/usr/bin/env bash
# Docker smoke test for the prover-ray image (reusable by CI).
#
# Checks four things:
#   1. dev-mock turns a dropped request file into a valid response file (the
#      supervisor runs each request in a child over stdin/stdout).
#   2. the guests folder holds one l2-execution ELF named by its Program ID.
#   3. dev-zkvm proves a real request: the guest output follows the spec.
#   4. the same dev-zkvm request over stdin/stdout leaves only the response on
#      stdout.
#
# Usage: scripts/docker-smoke.sh [image] [guest artifacts dir]
# Without a guest artifacts dir, the guest is built from riscv-guests.
set -euo pipefail

DOCKER="${DOCKER:-docker}"
IMAGE="${1:-consensys/lineth-prover-ray:dev}"
GUEST_SRC="${2:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
FIXTURE="$SCRIPT_DIR/../backend/jobadapter/testdata/request_single_block.json"
CONTAINER="prover-ray-smoke-$$"
WORK="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/prover-ray-smoke.XXXXXX")"

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

echo "==> guests folder: l2-execution ELF staged under its Program ID"
if ! program_id="$(DOCKER="$DOCKER" "$SCRIPT_DIR/stage-guest.sh" "$WORK/guests" ${GUEST_SRC:+"$GUEST_SRC"})"; then
    echo "FAIL: could not stage the guest"
    exit 1
fi
staged=("$WORK/guests/"*)
if [ "${#staged[@]}" -ne 1 ] || [[ "${staged[0]##*/}" != l2-execution-v*-"${program_id#0x}".elf ]]; then
    echo "FAIL: want one l2-execution ELF named by $program_id, got: ${staged[*]##*/}"
    exit 1
fi
echo "    ok: l2-execution staged as ${staged[0]##*/}"

echo "==> dev-zkvm: the guest proves a real request"
# One dev-zkvm prove of the guest's own stateless_input.ssz, with
# guestProgramId set to the staged guest.
ZKVM_FIXTURE="$SCRIPT_DIR/../backend/jobadapter/testdata/request_guest_fixture.json"
ZKVM_REQUEST="$WORK/zkvm/1-1-getZkL2ExecutionProofV1.json"
mkdir -p "$WORK/zkvm"
python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); d["guestProgramId"]=sys.argv[2]; json.dump(d, open(sys.argv[3], "w"))' \
    "$ZKVM_FIXTURE" "$program_id" "$ZKVM_REQUEST"
cat > "$WORK/zkvm/config.toml" <<'TOML'
version = "smoke"
log_level = 4
guests_dir = "/opt/linea/prover-ray/guests"
[execution]
prover_mode = "dev-zkvm"
requests_root_dir = "/data"
TOML
if ! $DOCKER run --rm \
        "${run_opts[@]}" \
        --entrypoint /opt/linea/prover-ray/prover \
        -v "$WORK/zkvm:/data" \
        -v "$WORK/guests:/opt/linea/prover-ray/guests:ro" \
        "$IMAGE" prove --config /data/config.toml \
        --in /data/1-1-getZkL2ExecutionProofV1.json --out /data/response.json; then
    echo "FAIL: dev-zkvm prove failed"
    cat "$WORK/zkvm/response.json" 2>/dev/null || true
    exit 1
fi
if ! grep -q '"proverVersion": "smoke-dev-zkvm"' "$WORK/zkvm/response.json"; then
    echo "FAIL: unexpected dev-zkvm response body"
    cat "$WORK/zkvm/response.json"
    exit 1
fi
echo "    ok: dev-zkvm response written"

echo "==> dev-zkvm over stdin/stdout: the way the supervisor runs every child"
# Same request, but piped in and out as the shared subprocess spawn does. Fails
# if anything besides the response reaches stdout (for example ZkC DEBUG output).
if ! $DOCKER run --rm -i \
        "${run_opts[@]}" \
        --entrypoint /opt/linea/prover-ray/prover \
        -v "$WORK/zkvm:/data" \
        -v "$WORK/guests:/opt/linea/prover-ray/guests:ro" \
        "$IMAGE" prove --config /data/config.toml --type l2-execution --id smoke \
        --in - --out - < "$ZKVM_REQUEST" > "$WORK/zkvm/stdout.json"; then
    echo "FAIL: dev-zkvm prove over stdin/stdout failed"
    cat "$WORK/zkvm/stdout.json" 2>/dev/null || true
    exit 1
fi
if ! python3 -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["proverVersion"]=="smoke-dev-zkvm", d["proverVersion"]' \
        "$WORK/zkvm/stdout.json"; then
    echo "FAIL: stdout is not exactly the dev-zkvm response JSON"
    head -c 2000 "$WORK/zkvm/stdout.json"
    exit 1
fi
echo "    ok: stdout carries only the response JSON"

echo "SMOKE PASSED"
