#!/usr/bin/env bash
# Stages a guest ELF into a guests folder and prints its Program ID. Builds it
# from riscv-guests unless a source folder (e.g. downloaded release assets) is
# given. PROGRAM_ID picks one when the source has several.
#
# Usage: scripts/stage-guest.sh <guests dir> [source dir]
# Env:   GUEST (default l2-execution), PROGRAM_ID, DOCKER
set -euo pipefail

OUT="${1:?usage: stage-guest.sh <guests dir> [source dir]}"
SRC="${2:-}"
GUEST="${GUEST:-l2-execution}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

fail() {
    echo "stage-guest.sh: $*" >&2
    exit 1
}

if [ -z "$SRC" ]; then
    SRC="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/stage-guest.XXXXXX")"
    trap 'rm -rf "$SRC"' EXIT
    make -C "$SCRIPT_DIR/../../riscv-guests" docker-artifacts GUEST="$GUEST" OUT="$SRC" \
        DOCKER="${DOCKER:-docker}" >&2
fi

shopt -s nullglob
if [ -n "${PROGRAM_ID:-}" ]; then
    [[ "$PROGRAM_ID" =~ ^0x[0-9a-f]{64}$ ]] || fail "PROGRAM_ID must be 0x + 64 lowercase hex"
    elfs=("$SRC/$GUEST"-v*-"${PROGRAM_ID#0x}".elf)
else
    elfs=("$SRC/$GUEST"-v*.elf)
fi
case "${#elfs[@]}" in
    0) fail "no $GUEST ELF${PROGRAM_ID:+ with Program ID $PROGRAM_ID} in $SRC" ;;
    1) ;;
    *) fail "${#elfs[@]} $GUEST ELFs in $SRC; set PROGRAM_ID to pick one" ;;
esac
elf="${elfs[0]}"
base="${elf##*/}"
base="${base%.elf}"
id="0x${base##*-}"
[[ "$id" =~ ^0x[0-9a-f]{64}$ ]] || fail "unexpected ELF name ${elf##*/}"

mkdir -p "$OUT"
cp "$elf" "$OUT/"
echo "$id"
