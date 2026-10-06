#!/usr/bin/env bash
# Writes a provenance partial next to the generated MDX partial so doc.linea
# readers can trace the partial back to the exact monorepo commit (and release
# tag, when applicable).
#
# Env:
#   SOURCE_SHA                     full commit SHA
#   SOURCE_REF_NAME                github.ref_name
#   SOURCE_REF_TYPE                github.ref_type ("tag" or "branch")
#   PROVENANCE_RELEASE_TAG_PREFIX  e.g. "releases/coordinator/"; empty disables the suffix
#   MDX_PARTIAL_PATH               repo-relative path to the MDX partial; provenance is
#                                  written to its parent directory as provenance.mdx
set -euo pipefail

if [[ -z "${SOURCE_SHA:-}" ]]; then
  echo "Refusing to publish: SOURCE_SHA is empty" >&2
  exit 1
fi

if [[ -z "${MDX_PARTIAL_PATH:-}" ]]; then
  echo "::error::MDX_PARTIAL_PATH is empty; cannot derive provenance output path" >&2
  exit 1
fi

SHORT="${SOURCE_SHA:0:7}"
OUT="$(dirname "$MDX_PARTIAL_PATH")/provenance.mdx"
mkdir -p "$(dirname "$OUT")"

LINE="Generated from [\`LFDT-Lineth/lineth-monorepo@${SHORT}\`](https://github.com/LFDT-Lineth/lineth-monorepo/commit/${SOURCE_SHA})."
if [[ "${SOURCE_REF_TYPE:-}" == "tag" && -n "${PROVENANCE_RELEASE_TAG_PREFIX:-}" && "${SOURCE_REF_NAME:-}" == "${PROVENANCE_RELEASE_TAG_PREFIX}"* ]]; then
  LINE="${LINE} (release \`${SOURCE_REF_NAME}\`)."
fi

# Capitalize the first letter of the component name for the footer sentence.
component="${COMPONENT_NAME:-component}"
component_display="$(printf '%s' "${component:0:1}" | tr '[:lower:]' '[:upper:]')${component:1}"

{
  echo "${LINE}"
  echo ""
  echo "Keys may differ in other ${component_display} releases."
  echo ""
} > "$OUT"

echo "Wrote $OUT"
