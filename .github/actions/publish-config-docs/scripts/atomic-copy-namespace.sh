#!/usr/bin/env bash
# Atomically swaps the generated namespace from the downloaded artifact into the
# docs checkout. A failed cp never leaves the destination partially populated:
# we copy into a staging directory first and only `mv` it into place once the
# copy succeeded.
#
# Env:
#   ARTIFACT_DIR        absolute path to the downloaded artifact root
#   ARTIFACT_NAMESPACE  directory inside the artifact containing the MDX partials
#                       (defaults to the basename of DOCS_NAMESPACE)
#   DOCS_CHECKOUT       absolute path to the docs repository checkout
#   DOCS_NAMESPACE      repo-relative directory under the docs checkout that the
#                       generated partials replace (e.g. docs/stack/reference/_generated/coordinator)
set -euo pipefail

if [[ -z "${ARTIFACT_DIR:-}" || -z "${DOCS_CHECKOUT:-}" || -z "${DOCS_NAMESPACE:-}" ]]; then
  echo "::error::ARTIFACT_DIR, DOCS_CHECKOUT and DOCS_NAMESPACE must all be set" >&2
  exit 1
fi

namespace_basename="$(basename "$DOCS_NAMESPACE")"
artifact_namespace="${ARTIFACT_NAMESPACE:-$namespace_basename}"

SRC="$ARTIFACT_DIR/$artifact_namespace"
DEST="$DOCS_CHECKOUT/$DOCS_NAMESPACE"
STAGING="$DOCS_CHECKOUT/.${namespace_basename}-config-docs-staging"

if [[ ! -d "$SRC" ]]; then
  echo "::error::Refusing to publish: artifact namespace $SRC does not exist" >&2
  exit 1
fi
if ! compgen -G "$SRC/*.mdx" >/dev/null; then
  echo "::error::Refusing to publish: no MDX partials under $SRC" >&2
  exit 1
fi

# Copy into a staging directory first, then atomically swap it into place so a
# failed cp never leaves the destination partially populated.
rm -rf "$STAGING"
mkdir -p "$STAGING"
cp -R "$SRC/." "$STAGING/"

rm -rf "$DEST"
mkdir -p "$(dirname "$DEST")"
mv "$STAGING" "$DEST"

echo "Swapped $SRC -> $DEST"
