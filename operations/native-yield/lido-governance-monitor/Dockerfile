ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-slim AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

# Temp fix for the corepack issue described in https://github.com/pnpm/pnpm/issues/9029
RUN npm i -g corepack@latest

RUN corepack enable

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates bash curl \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

FROM base AS builder

WORKDIR /usr/src/app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./

COPY ./operations/native-yield/lido-governance-monitor/package.json ./operations/native-yield/lido-governance-monitor/package.json
COPY ./ts-libs/linea-shared-utils/package.json ./ts-libs/linea-shared-utils/package.json
COPY ./ts-libs/eslint-config/package.json ./ts-libs/eslint-config/package.json

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts

COPY ./operations/native-yield/lido-governance-monitor ./operations/native-yield/lido-governance-monitor
COPY ts-libs/linea-shared-utils ./ts-libs/linea-shared-utils
COPY ts-libs/eslint-config ./ts-libs/eslint-config

# Generate Prisma client and build
# DATABASE_URL is required for prisma generate but not used during build
RUN --mount=type=cache,id=pnpm,target=/pnpm/store DATABASE_URL=postgresql://localhost:5432/dummy pnpm --filter @lfdt-lineth/lido-governance-monitor db:generate \
    && pnpm run build \
    && pnpm deploy --legacy --filter=./operations/native-yield/lido-governance-monitor --prod ./prod/operations/native-yield/lido-governance-monitor

FROM node:${NODE_VERSION}-slim AS production

ENV NODE_ENV=production

WORKDIR /usr/src/app

RUN apt-get update \
    && apt-get install -y --no-install-recommends openssl \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder --chown=node:node /usr/src/app/prod/operations/native-yield/lido-governance-monitor ./operations/native-yield/lido-governance-monitor

# Copy prompts directory (not included in dist)
COPY --from=builder --chown=node:node /usr/src/app/operations/native-yield/lido-governance-monitor/src/prompts ./operations/native-yield/lido-governance-monitor/src/prompts

# Copy prisma migrations for runtime migrate deploy
COPY --from=builder --chown=node:node /usr/src/app/operations/native-yield/lido-governance-monitor/prisma ./operations/native-yield/lido-governance-monitor/prisma
COPY --from=builder --chown=node:node /usr/src/app/operations/native-yield/lido-governance-monitor/prisma.config.ts ./operations/native-yield/lido-governance-monitor/prisma.config.ts

RUN mkdir -p /home/node/.cache/prisma \
    && chown -R node:node /home/node/.cache

WORKDIR /usr/src/app/operations/native-yield/lido-governance-monitor

USER node

CMD [ "node", "./dist/run.js" ]
