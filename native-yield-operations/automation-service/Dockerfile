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

COPY ./native-yield-operations/automation-service/package.json ./native-yield-operations/automation-service/package.json
COPY ./ts-libs/linea-shared-utils/package.json ./ts-libs/linea-shared-utils/package.json
COPY ./ts-libs/eslint-config/package.json ./ts-libs/eslint-config/package.json

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts

COPY ./native-yield-operations/automation-service ./native-yield-operations/automation-service
COPY ts-libs/linea-shared-utils ./ts-libs/linea-shared-utils
COPY ts-libs/eslint-config ./ts-libs/eslint-config

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm run build \
    && pnpm deploy --legacy --filter=./native-yield-operations/automation-service --prod ./prod/native-yield-operations/automation-service

FROM node:${NODE_VERSION}-slim AS production

ENV NODE_ENV=production

WORKDIR /usr/src/app

# Create ipfs-cache directory with proper permissions before switching to node user
# The @lidofinance/lsv-cli library requires this directory for caching IPFS data
RUN mkdir -p /usr/src/app/ipfs-cache && \
    chown -R node:node /usr/src/app/ipfs-cache && \
    chmod 755 /usr/src/app/ipfs-cache

USER node

COPY --from=builder /usr/src/app/prod/native-yield-operations/automation-service ./native-yield-operations/automation-service

CMD [ "node", "./native-yield-operations/automation-service/dist/run.js" ]
