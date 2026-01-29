ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-slim AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

# Temp fix for the corepack issue described in https://github.com/pnpm/pnpm/issues/9029
RUN npm i -g corepack@latest

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

COPY ./postman/package.json ./postman/package.json
COPY ./sdk/sdk-ethers/package.json ./sdk/sdk-ethers/package.json
COPY ./ts-libs/linea-native-libs/package.json ./ts-libs/linea-native-libs/package.json
COPY ./ts-libs/linea-shared-utils/package.json ./ts-libs/linea-shared-utils/package.json
COPY ./ts-libs/eslint-config/package.json ./ts-libs/eslint-config/package.json

# Use cache mount for faster rebuilds and prefer offline to use cache
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts

# Copy source code for building
COPY ./postman ./postman
COPY ./sdk/sdk-ethers ./sdk/sdk-ethers
COPY ./ts-libs/linea-native-libs ./ts-libs/linea-native-libs
COPY ./ts-libs/linea-shared-utils ./ts-libs/linea-shared-utils
COPY ./ts-libs/eslint-config ./ts-libs/eslint-config

# Build and create production deployment
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm run build \
    && pnpm deploy --legacy --filter=./postman --prod ./prod/postman

FROM node:${NODE_VERSION}-slim AS production

ENV NODE_ENV=production

WORKDIR /usr/src/app

# Copy only production dependencies and built files
# Note: Using slim (Debian/glibc) instead of alpine (musl) for compatibility
# with the prebuilt native blob_compressor Go library that's dynamically linked against glibc
COPY --from=builder --chown=node:node /usr/src/app/prod/postman ./postman

# Run as non-root user
USER node

# Use exec form for proper signal handling
CMD ["node", "./postman/dist/scripts/runPostman.js"]
