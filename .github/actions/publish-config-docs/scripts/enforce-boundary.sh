#!/usr/bin/env bash
# Refuses any change in the docs checkout that falls outside the configured
# component namespace. This is the safety rail that prevents a misconfigured
# generator from accidentally rewriting human-owned docs pages.
#
# Env:
#   DOCS_CHECKOUT   absolute path to the docs repository checkout
#   DOCS_NAMESPACE  repo-relative directory the action is allowed to mutate
set -euo pipefail

if [[ -z "${DOCS_CHECKOUT:-}" || -z "${DOCS_NAMESPACE:-}" ]]; then
  echo "::error::DOCS_CHECKOUT and DOCS_NAMESPACE must both be set" >&2
  exit 1
fi

invalid=0
while IFS= read -r -d '' entry; do
  # porcelain=v1 format: "XY <path>", with the path starting at column 4.
  changed_path="${entry:3}"
  if [[ "$changed_path" != "${DOCS_NAMESPACE}"/* ]]; then
    echo "::error::Refusing to publish changed path outside the component namespace: $changed_path"
    invalid=1
  fi
done < <(git -C "$DOCS_CHECKOUT" status --porcelain=v1 -z --untracked-files=all)

if [[ "$invalid" -ne 0 ]]; then
  exit 1
fi

git -C "$DOCS_CHECKOUT" status --short
echo "All changes confined to $DOCS_NAMESPACE."
