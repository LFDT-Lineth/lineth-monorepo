# kics-scan disable=965a08d7-ef86-4f14-8792-4a3b2098937e
# Apt Get Install Pin Version Not Defined: apt packages are intentionally unpinned.
# They track the digest-pinned node:slim base image; pinning would break the build
# on base refresh for negligible security gain on top of --no-install-recommends.
ARG NODE_VERSION=24.22.1
# Digest of node:${NODE_VERSION}-slim, kept in sync with .nvmrc. If you override
# NODE_VERSION at build time, override NODE_IMAGE_DIGEST to match.
ARG NODE_IMAGE_DIGEST=sha256:be8204d4bd327363357f532187b752ff62759c31bedf98e9e18fc08c9cbd605c

FROM node:${NODE_VERSION}-slim@${NODE_IMAGE_DIGEST} AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

# Temp fix for the corepack issue described in https://github.com/pnpm/pnpm/issues/9029
# Use `npm install` (not the `i` alias): KICS only whitelists the literal word `install`.
# Version pinned to 0.35.0; bump deliberately.
RUN npm install -g corepack@0.35.0

RUN corepack enable

FROM base AS builder

# Install build dependencies only in builder stage (using Debian packages for glibc compatibility)
RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 ca-certificates bash curl make g++ \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /usr/src/app

ARG NATIVE_LIBS_RELEASE_TAG
ENV NATIVE_LIBS_RELEASE_TAG=${NATIVE_LIBS_RELEASE_TAG}

# Copy workspace and package files for dependency installation
# This layer is cached unless these files change
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY patches/ ./patches/
COPY ./postman/package.json ./postman/package.json
COPY ./ts-libs/sdk/sdk-core/package.json ./ts-libs/sdk/sdk-core/package.json
COPY ./ts-libs/sdk/sdk-viem/package.json ./ts-libs/sdk/sdk-viem/package.json
COPY ./ts-libs/linea-native-libs/package.json ./ts-libs/linea-native-libs/package.json
COPY ./ts-libs/linea-shared-utils/package.json ./ts-libs/linea-shared-utils/package.json
COPY ./ts-libs/eslint-config/package.json ./ts-libs/eslint-config/package.json

# Use cache mount for faster rebuilds and prefer offline to use cache
# kics-scan ignore-line
# Versions are pinned by pnpm-lock.yaml (--frozen-lockfile); this is a workspace
# install from the lockfile, not a single-package add. KICS: NPM Install Without Pinned Version.
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts

# Copy source code for building
COPY ./postman ./postman
COPY ./ts-libs/sdk/sdk-core ./ts-libs/sdk/sdk-core
COPY ./ts-libs/sdk/sdk-viem ./ts-libs/sdk/sdk-viem
COPY ./ts-libs/linea-native-libs ./ts-libs/linea-native-libs
COPY ./ts-libs/linea-shared-utils ./ts-libs/linea-shared-utils
COPY ./ts-libs/eslint-config ./ts-libs/eslint-config

# Build and create production deployment
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm run build \
    && pnpm --config.allow-unused-patches=true --config.block-exotic-subdeps=false deploy --legacy --filter=./postman --prod ./prod/postman

FROM node:${NODE_VERSION}-slim@${NODE_IMAGE_DIGEST} AS production

ENV NODE_ENV=production

WORKDIR /usr/src/app

# Copy only production dependencies and built files
# Note: Using slim (Debian/glibc) instead of alpine (musl) for compatibility
# with the prebuilt native blob_compressor Go library that's dynamically linked against glibc
COPY --from=builder --chown=node:node /usr/src/app/prod/postman ./postman
COPY LICENSE-APACHE LICENSE-MIT /licenses/

# Run as non-root user
USER node

# Use exec form for proper signal handling
CMD ["node", "./postman/dist/src/main.js"]

# Build-time metadata as defined at http://label-schema.org
# Values are injected by scripts/docker/build-image.sh.
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="postman" \
	org.label-schema.description="Lineth Postman, the bridge message executor" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/LFDT-Lineth/lineth-monorepo" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0" \
	org.opencontainers.image.licenses="MIT OR Apache-2.0"
