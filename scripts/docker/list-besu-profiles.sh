#!/usr/bin/env bash
#
# Lists Besu profile names (one per *.toml file) under a profiles directory, as a JSON array
# suitable for a GitHub Actions matrix. Shared by:
#   - lineth-monorepo/.github/workflows/reusable-linea-besu-package-run-test.yml
#   - lineth-enterprise/.github/workflows/e-besu-package-run-test.yml (via the mirrored
#     scripts/docker/ copy — see .github/actions/e-checkout-and-setup-lineth-monorepo)
#
# Usage: list-besu-profiles.sh <profiles-dir>
# When $GITHUB_OUTPUT is set, writes `files=<json>`; otherwise prints the JSON to stdout.

set -euo pipefail

profiles_dir="${1:?Usage: list-besu-profiles.sh <profiles-dir>}"

files_json="$(
  find "${profiles_dir}" -maxdepth 1 -type f -name '*.toml' -exec basename {} .toml \; |
    sort |
    jq -R -s -c 'split("\n") | map(select(length > 0))'
)"

echo "Profiles: ${files_json}" >&2

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  echo "files=${files_json}" >> "${GITHUB_OUTPUT}"
else
  echo "${files_json}"
fi
