#!/usr/bin/env bash
#
# Build (and optionally publish) a Lineth Docker image with `docker buildx`.
#
# Single source of truth shared by CI (.github/actions/docker-build-publish)
# and local runs (`make docker-build-<image>`), so a local build exercises the
# same buildx invocation as the pipeline.
#
# Every option can be given as a flag or as an environment variable, so the
# GitHub composite action can pass its (multiline) inputs straight through.
#
# Written for bash 3.2 so it also runs on stock macOS.

set -euo pipefail

die() {
  if [[ "${GITHUB_ACTIONS:-}" == "true" ]]; then
    echo "::error::$*" >&2
  else
    echo "ERROR: $*" >&2
  fi
  exit 1
}

warn() {
  echo "WARNING: $*" >&2
}

usage() {
  cat <<'EOF'
Usage: scripts/docker/build-image.sh --image-name NAME --tags TAGS --dockerfile PATH [options]

Required:
  --image-name NAME         Image repository, e.g. consensys/linea-coordinator   [env: IMAGE_NAME]
  --tags TAGS               Comma-separated tags. The first one is the primary   [env: IMAGE_TAGS]
                            tag: it is the only tag applied to a local (--load)
                            build and the one used by --save-to.
  --dockerfile PATH         Path to the Dockerfile                               [env: DOCKERFILE_PATH]

Options:
  --context PATH            Build context (default: .)                           [env: DOCKER_CONTEXT]
  --build-arg K=V           Repeatable                                           [env: BUILD_ARGS, one per line]
  --build-context NAME=PATH Repeatable named build context                       [env: BUILD_CONTEXTS, one per line]
  --secret SPEC             Repeatable buildx secret                             [env: BUILD_SECRETS, one per line]
                            e.g. id=mysecret,src=/path or id=mysecret,env=VAR
  --platforms LIST          Comma-separated platforms
                            (default: linux/amd64, or linux/amd64,linux/arm64
                            when --push)                                         [env: PLATFORMS]
  --push                    Push all tags instead of loading the primary tag
                            into the local docker image store                    [env: PUSH_IMAGE=true]
  --save-to FILE            After a successful --load build, write the primary
                            image to FILE as a gzipped tarball                   [env: SAVE_TO]
  --registry-cache          Import/export the shared registry build cache
                            (<image>:buildcache-<arch>). Export only happens
                            with --push. Default: on in CI, off locally.         [env: REGISTRY_CACHE=true|false]
  --no-registry-cache       Disable the registry build cache.
  --builder NAME            Use (and create if missing) this buildx builder.     [env: DOCKER_BUILDER]
                            Created with the docker-container driver, which is
                            what CI uses. Leave empty to use the current builder.
  --dry-run                 Print the commands instead of running them.          [env: DRY_RUN=true]
  -h, --help                Show this help.
EOF
}

IMAGE_NAME="${IMAGE_NAME:-}"
IMAGE_TAGS="${IMAGE_TAGS:-}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-}"
DOCKER_CONTEXT="${DOCKER_CONTEXT:-.}"
PLATFORMS="${PLATFORMS:-}"
PUSH_IMAGE="${PUSH_IMAGE:-false}"
SAVE_TO="${SAVE_TO:-}"
DOCKER_BUILDER="${DOCKER_BUILDER:-}"
DRY_RUN="${DRY_RUN:-false}"
# Registry cache defaults on in CI (where it is populated and warm) and off
# locally, so a local build needs neither network nor registry credentials.
REGISTRY_CACHE="${REGISTRY_CACHE:-${GITHUB_ACTIONS:-false}}"

BUILD_ARG_ITEMS=()
BUILD_CONTEXT_ITEMS=()
SECRET_ITEMS=()

# Trim leading/trailing whitespace.
trim() {
  local s="$1"
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image-name) IMAGE_NAME="$2"; shift 2 ;;
    --tags) IMAGE_TAGS="${IMAGE_TAGS:+${IMAGE_TAGS},}$2"; shift 2 ;;
    --dockerfile) DOCKERFILE_PATH="$2"; shift 2 ;;
    --context) DOCKER_CONTEXT="$2"; shift 2 ;;
    --build-arg) BUILD_ARG_ITEMS+=("$2"); shift 2 ;;
    --build-context) BUILD_CONTEXT_ITEMS+=("$2"); shift 2 ;;
    --secret) SECRET_ITEMS+=("$2"); shift 2 ;;
    --platforms) PLATFORMS="$2"; shift 2 ;;
    --push) PUSH_IMAGE=true; shift ;;
    --save-to) SAVE_TO="$2"; shift 2 ;;
    --registry-cache) REGISTRY_CACHE=true; shift ;;
    --no-registry-cache) REGISTRY_CACHE=false; shift ;;
    --builder) DOCKER_BUILDER="$2"; shift 2 ;;
    --dry-run) DRY_RUN=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) usage >&2; die "unknown argument: $1" ;;
  esac
done

[[ -n "$IMAGE_NAME" ]] || { usage >&2; die "--image-name is required"; }
[[ -n "$DOCKERFILE_PATH" ]] || { usage >&2; die "--dockerfile is required"; }
[[ -f "$DOCKERFILE_PATH" ]] || die "Dockerfile not found: ${DOCKERFILE_PATH}"
[[ -d "$DOCKER_CONTEXT" ]] || die "build context not found: ${DOCKER_CONTEXT}"

# Append each non-empty, trimmed line of a multiline env var to an array.
collect_lines() {
  local target="$1" value="$2" line
  [[ -n "$value" ]] || return 0
  while IFS= read -r line; do
    line="$(trim "$line")"
    [[ -n "$line" ]] || continue
    eval "${target}+=(\"\$line\")"
  done <<< "$value"
}

collect_lines BUILD_ARG_ITEMS "${BUILD_ARGS:-}"
collect_lines BUILD_CONTEXT_ITEMS "${BUILD_CONTEXTS:-}"
collect_lines SECRET_ITEMS "${BUILD_SECRETS:-}"

# --- tags -------------------------------------------------------------------
# Split on commas, trim, drop empties, de-duplicate while keeping the first
# occurrence order. The first surviving entry is the primary tag.
TAGS=()
add_tag() {
  local candidate="$1" existing
  for existing in ${TAGS[@]+"${TAGS[@]}"}; do
    [[ "$existing" == "$candidate" ]] && return 0
  done
  TAGS+=("$candidate")
}

if [[ -n "$IMAGE_TAGS" ]]; then
  IFS=',' read -ra PARSED_TAGS <<< "$IMAGE_TAGS"
  for raw_tag in ${PARSED_TAGS[@]+"${PARSED_TAGS[@]}"}; do
    trimmed="$(trim "$raw_tag")"
    [[ -n "$trimmed" ]] && add_tag "$trimmed"
  done
fi

if [[ ${#TAGS[@]} -eq 0 ]]; then
  die "no image tags resolved from --tags/IMAGE_TAGS — in CI this usually means the upstream version-tag job was cancelled or failed. Failing early to avoid an invalid Docker tag."
fi

PRIMARY_IMAGE="${IMAGE_NAME}:${TAGS[0]}"

# --- platforms --------------------------------------------------------------
if [[ -z "$PLATFORMS" ]]; then
  if [[ "$PUSH_IMAGE" == "true" ]]; then
    PLATFORMS="linux/amd64,linux/arm64"
  else
    PLATFORMS="linux/amd64"
  fi
fi
IFS=',' read -ra PLATFORM_LIST <<< "$PLATFORMS"

# --- builder ----------------------------------------------------------------
# CI provisions the builder with docker/setup-buildx-action. Locally we create
# an equivalent docker-container builder on demand: the default `docker` driver
# supports neither multi-platform builds nor registry cache import/export.
if [[ -n "$DOCKER_BUILDER" ]]; then
  if ! docker buildx inspect "$DOCKER_BUILDER" >/dev/null 2>&1; then
    if [[ "$DRY_RUN" == "true" ]]; then
      echo "Would create buildx builder '${DOCKER_BUILDER}' (driver: docker-container)"
    else
      echo "Creating buildx builder '${DOCKER_BUILDER}' (driver: docker-container)"
      docker buildx create --name "$DOCKER_BUILDER" --driver docker-container >/dev/null
    fi
  fi
fi

# --- assemble the buildx command -------------------------------------------
BUILD_CMD=(docker buildx build)
[[ -n "$DOCKER_BUILDER" ]] && BUILD_CMD+=(--builder "$DOCKER_BUILDER")
BUILD_CMD+=(--file "$DOCKERFILE_PATH" --platform "$PLATFORMS")

for item in ${BUILD_ARG_ITEMS[@]+"${BUILD_ARG_ITEMS[@]}"}; do
  BUILD_CMD+=(--build-arg "$item")
done
for item in ${BUILD_CONTEXT_ITEMS[@]+"${BUILD_CONTEXT_ITEMS[@]}"}; do
  BUILD_CMD+=(--build-context "$item")
done
for item in ${SECRET_ITEMS[@]+"${SECRET_ITEMS[@]}"}; do
  BUILD_CMD+=(--secret "$item")
done

if [[ "$PUSH_IMAGE" == "true" ]]; then
  # Every tag is published.
  for tag in ${TAGS[@]+"${TAGS[@]}"}; do
    BUILD_CMD+=(--tag "${IMAGE_NAME}:${tag}")
  done
  BUILD_CMD+=(--push)
  if [[ "$REGISTRY_CACHE" == "true" ]]; then
    BUILD_CMD+=(
      --cache-from "type=registry,ref=${IMAGE_NAME}:buildcache-amd64,platform=linux/amd64"
      --cache-from "type=registry,ref=${IMAGE_NAME}:buildcache-arm64,platform=linux/arm64"
      --cache-to "type=registry,ref=${IMAGE_NAME}:buildcache-amd64,mode=max,platform=linux/amd64"
      --cache-to "type=registry,ref=${IMAGE_NAME}:buildcache-arm64,mode=max,platform=linux/arm64"
    )
  fi
else
  # Local build: only the primary tag, and only imported into the docker image
  # store when a single platform is requested (`--load` cannot handle a
  # multi-platform manifest).
  BUILD_CMD+=(--tag "$PRIMARY_IMAGE")
  if [[ ${#PLATFORM_LIST[@]} -eq 1 ]]; then
    BUILD_CMD+=(--load)
  else
    warn "building ${#PLATFORM_LIST[@]} platforms without --push: the result stays in the build cache and is not loaded into 'docker images'."
  fi
  if [[ "$REGISTRY_CACHE" == "true" ]]; then
    BUILD_CMD+=(--cache-from "type=registry,ref=${IMAGE_NAME}:buildcache-amd64")
  fi
fi

BUILD_CMD+=("$DOCKER_CONTEXT")

if [[ -n "$SAVE_TO" && "$PUSH_IMAGE" == "true" ]]; then
  warn "--save-to is ignored semantics-wise for pushed images: the image is not loaded locally, so 'docker save' will fail."
fi

print_cmd() {
  local part out=""
  for part in "$@"; do
    out+=" $(printf '%q' "$part")"
  done
  echo "+${out}"
}

echo "Image:     ${PRIMARY_IMAGE}"
echo "Tags:      ${TAGS[*]}"
echo "Platforms: ${PLATFORMS}"
echo "Mode:      $([[ "$PUSH_IMAGE" == "true" ]] && echo push || echo local)"
print_cmd "${BUILD_CMD[@]}"

if [[ "$DRY_RUN" == "true" ]]; then
  if [[ -n "$SAVE_TO" ]]; then
    print_cmd docker save "$PRIMARY_IMAGE" "|" gzip ">" "$SAVE_TO"
  fi
  exit 0
fi

"${BUILD_CMD[@]}"

if [[ -n "$SAVE_TO" ]]; then
  echo "Saving ${PRIMARY_IMAGE} to ${SAVE_TO}"
  docker save "$PRIMARY_IMAGE" | gzip > "$SAVE_TO"
fi
