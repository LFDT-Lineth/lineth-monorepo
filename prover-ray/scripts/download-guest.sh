#!/usr/bin/env bash
# Downloads a guest ELF from its GitHub release into a folder, for stage-guest.sh.
# Takes the most recently published release, excluding drafts and pre-releases,
# unless GUEST_VERSION is set. Needs gh, authenticated through GH_TOKEN in CI.
#
# Usage: scripts/download-guest.sh <out dir>
# Env:   GUEST (default l2-execution), GUEST_VERSION (e.g. 0.1.0),
#        GH_REPO (default LFDT-Lineth/lineth-monorepo)
set -euo pipefail

OUT="${1:?usage: download-guest.sh <out dir>}"
GUEST="${GUEST:-l2-execution}"
export GH_REPO="${GH_REPO:-LFDT-Lineth/lineth-monorepo}"
prefix="releases/riscv-$GUEST/v"

fail() {
    echo "download-guest.sh: $*" >&2
    exit 1
}

if [ -n "${GUEST_VERSION:-}" ]; then
    tag="$prefix${GUEST_VERSION#v}"
else
    # The monorepo releases every component, so filter by tag rather than trust "latest".
    tag="$(gh release list --exclude-drafts --exclude-pre-releases --limit 1000 --json tagName,publishedAt \
        --jq "map(select(.tagName | startswith(\"$prefix\"))) | sort_by(.publishedAt) | last | .tagName // empty")"
    [ -n "$tag" ] || fail "no published $GUEST release in $GH_REPO"
fi

mkdir -p "$OUT"
gh release download "$tag" --pattern "$GUEST-v*.elf" --dir "$OUT" --clobber
echo "Downloaded $GUEST guest from $GH_REPO release $tag" >&2
