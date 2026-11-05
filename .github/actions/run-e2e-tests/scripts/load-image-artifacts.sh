#!/usr/bin/env bash
# Usage: load-image-artifacts.sh <artifact-dir>
# Loads CI image artifacts and prints the selected image tags as VAR=value lines for $GITHUB_ENV.
# Tag precedence: loaded artifact with E2E_IMAGE_TAG, explicit env var, E2E_FALLBACK_IMAGE_TAG,
# otherwise the Compose default.

set -euo pipefail

artifact_dir="${1:?usage: $0 <artifact-dir>}"
image_tag="${E2E_IMAGE_TAG:-}"
fallback_tag="${E2E_FALLBACK_IMAGE_TAG:-}"
besu_package_provided="false"
[ -n "${LINEA_BESU_PACKAGE_TAG:-}" ] && besu_package_provided="true"

loaded_images="$(mktemp)"
trap 'rm -f "$loaded_images"' EXIT

shopt -s nullglob
for artifact in "$artifact_dir"/*.tar.gz; do
  if ! repo_tags="$(tar -xOzf "$artifact" manifest.json 2>/dev/null | jq -er '.[].RepoTags[]?')"; then
    echo "Skipping non-Docker artifact ${artifact}." >&2
    continue
  fi
  echo "Loading Docker image artifact ${artifact}..." >&2
  printf '%s\n' "$repo_tags" >> "$loaded_images"
  gunzip -c "$artifact" | docker load >&2
done

# Keep BESU_PACKAGE_IMAGE_NAME stable even when an override compose file is used.
echo "BESU_PACKAGE_IMAGE_NAME=consensys/linea-besu-package"

while read -r env_var image; do
  if [ -n "$image_tag" ] && grep -Fqx "${image}:${image_tag}" "$loaded_images"; then
    selected_tag="$image_tag"
    [ "$env_var" = "LINEA_BESU_PACKAGE_TAG" ] && besu_package_provided="true"
  elif [ -n "${!env_var:-}" ]; then
    selected_tag="${!env_var}"
  elif [ -n "$fallback_tag" ]; then
    selected_tag="$fallback_tag"
  else
    continue
  fi
  echo "${env_var}=${selected_tag}"
done <<'EOF'
LINEA_BESU_PACKAGE_TAG consensys/linea-besu-package
LINEA_COORDINATOR_TAG consensys/linea-coordinator
LINEA_POSTMAN_TAG consensys/linea-postman
LINEA_PROVER_TAG consensys/linea-prover
LINEA_TRANSACTION_EXCLUSION_API_TAG consensys/linea-transaction-exclusion-api
MARU_TAG consensys/maru
EOF

echo "E2E_BESU_PACKAGE_PROVIDED=${besu_package_provided}"
