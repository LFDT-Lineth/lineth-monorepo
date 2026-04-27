#!/bin/bash

set -e

echo "BESU_DIR=$BESU_DIR"
echo "BESU_COMMIT=$BESU_COMMIT"
echo "VERSION_LABEL=$VERSION_LABEL"
echo "SKIP_CHECKOUT_AFTER_CLONE=$SKIP_CHECKOUT_AFTER_CLONE"
SHORT_COMMIT=${BESU_COMMIT:0:7}
echo "SHORT_COMMIT=$SHORT_COMMIT"

if [ ! -d "$BESU_DIR/.git" ]; then
  echo "Cloning https://github.com/besu-eth/besu into $BESU_DIR"
  mkdir -p "$(dirname "$BESU_DIR")"
  # Partial clone: skip trees + blobs on the initial fetch so we only download the
  # commit graph + tag refs. That's all `git describe --tags --abbrev=0 <commit>`
  # below needs, keeping the version-only resolve fast for published commits. If a
  # later step (build-dist-and-publish.sh) actually needs source files, 
  # a subsequent git checkout will fetch the needed blobs on demand.
  git clone --filter=tree:0 --no-checkout https://github.com/besu-eth/besu.git "$BESU_DIR"
  if [ "$SKIP_CHECKOUT_AFTER_CLONE" != "true" ]; then
    (cd "$BESU_DIR" && git checkout "$BESU_COMMIT")
  fi
else
  (cd "$BESU_DIR" && git reset --hard && git fetch origin && git checkout "$BESU_COMMIT")
fi

BASE_TAG=$(cd "$BESU_DIR" && git describe --tags --abbrev=0 "$BESU_COMMIT" 2>/dev/null || true)

if [ -n "$BASE_TAG" ]; then
  BESU_VERSION="${BASE_TAG}${VERSION_LABEL}-${SHORT_COMMIT}"
else
  BESU_VERSION="0.0.0${VERSION_LABEL}-${SHORT_COMMIT}"
fi

echo "Resolved besuVersion: $BESU_VERSION"
echo "$BESU_VERSION"
