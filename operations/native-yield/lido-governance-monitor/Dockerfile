ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-slim AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

# Temp fix for the corepack issue described in https://github.com/pnpm/pnpm/issues/9029
# Version pinned (KICS: NPM Install Command Without Pinned Version). Bump deliberately.
RUN npm i -g corepack@0.35.0

RUN corepack enable

# kics-scan ignore-line
# Not version-pinned: these packages track the node:slim base image, and pinning
# would break the build on every base refresh for negligible security gain here.
# KICS: Apt Get Install Pin Version Not Defined (ca-certificates, bash, curl).
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates bash curl \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

FROM base AS builder

WORKDIR /usr/src/app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./

COPY patches/ ./patches/
COPY ./operations/native-yield/lido-governance-monitor/package.json ./operations/native-yield/lido-governance-monitor/package.json
COPY ./ts-libs/linea-shared-utils/package.json ./ts-libs/linea-shared-utils/package.json
COPY ./ts-libs/eslint-config/package.json ./ts-libs/eslint-config/package.json

# kics-scan ignore-line
# Versions are pinned by pnpm-lock.yaml (--frozen-lockfile); this is a workspace
# install from the lockfile, not a single-package add. KICS: NPM Install Without Pinned Version.
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts

COPY ./operations/native-yield/lido-governance-monitor ./operations/native-yield/lido-governance-monitor
COPY ts-libs/linea-shared-utils ./ts-libs/linea-shared-utils
COPY ts-libs/eslint-config ./ts-libs/eslint-config

# Generate Prisma client and build
# DATABASE_URL is required for prisma generate but not used during build
RUN --mount=type=cache,id=pnpm,target=/pnpm/store DATABASE_URL=postgresql://localhost:5432/dummy pnpm --filter @lfdt-lineth/lido-governance-monitor db:generate \
    && pnpm run build \
    && pnpm --config.allow-unused-patches=true --config.block-exotic-subdeps=false deploy --legacy --filter=./operations/native-yield/lido-governance-monitor --prod ./prod/operations/native-yield/lido-governance-monitor

# kics-scan ignore-line
# HEALTHCHECK does not apply: this image runs a one-shot batch job (run.ts exits
# after a single execution), not a long-running server. KICS: Healthcheck Instruction Missing.
FROM node:${NODE_VERSION}-slim AS production

ENV NODE_ENV=production

WORKDIR /usr/src/app

# kics-scan ignore-line
# Not version-pinned: openssl tracks the node:slim base image; pinning would break
# the build on base refresh for negligible gain. KICS: Apt Get Install Pin Version Not Defined.
RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

# Runtime files are intentionally root-owned (no --chown=node:node): the node user
# only needs read/execute, and root ownership prevents tampering with code and
# migrations. The writable Prisma cache is chown'd separately below.
# KICS: Chown Flag Exists.
COPY --from=builder /usr/src/app/prod/operations/native-yield/lido-governance-monitor ./operations/native-yield/lido-governance-monitor

# Copy prompts directory (not included in dist)
COPY --from=builder /usr/src/app/operations/native-yield/lido-governance-monitor/src/prompts ./operations/native-yield/lido-governance-monitor/src/prompts

# Copy prisma migrations for runtime migrate deploy
COPY --from=builder /usr/src/app/operations/native-yield/lido-governance-monitor/prisma ./operations/native-yield/lido-governance-monitor/prisma
COPY --from=builder /usr/src/app/operations/native-yield/lido-governance-monitor/prisma.config.ts ./operations/native-yield/lido-governance-monitor/prisma.config.ts

RUN mkdir -p /home/node/.cache/prisma \
    && chown -R node:node /home/node/.cache

WORKDIR /usr/src/app/operations/native-yield/lido-governance-monitor

USER node

CMD [ "node", "./dist/run.js" ]
