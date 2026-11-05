#!/usr/bin/env bash
# Stages the guest ELF bundled in a prover-ray image into a guests folder under
# the release name <guest>-v<version>-<id>.elf and prints its 0x-prefixed
# Program ID. For dev and tests only.
#
# Usage: scripts/stage-guest.sh <image> <guests dir>
set -euo pipefail

DOCKER="${DOCKER:-docker}"
IMAGE="${1:?usage: stage-guest.sh <image> <guests dir>}"
OUT="${2:?usage: stage-guest.sh <image> <guests dir>}"
ELF=/opt/linea/prover-ray/evm_execution_guest
NAME="stage-guest-$$"
# The release workflow sets the version of released ELFs. The bundled ELF is
# built from source and has no release version, so this placeholder only fills
# the release name format. Lookup uses the Program ID, not the version.
DEV_VERSION=0.0.0-dev

id="$($DOCKER run --rm --entrypoint /opt/linea/prover-ray/prover "$IMAGE" program-id --elf "$ELF")"
if ! [[ "$id" =~ ^0x[0-9a-f]{64}$ ]]; then
    echo "unexpected Program ID output: $id" >&2
    exit 1
fi

trap '$DOCKER rm -f "$NAME" >/dev/null 2>&1 || true' EXIT
mkdir -p "$OUT"
$DOCKER create --name "$NAME" "$IMAGE" >/dev/null
$DOCKER cp "$NAME:$ELF" "$OUT/l2-execution-v$DEV_VERSION-${id#0x}.elf"
echo "$id"
