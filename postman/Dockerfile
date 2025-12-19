ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-slim AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

# Temp fix for the corepack issue described in https://github.com/pnpm/pnpm/issues/9029
RUN npm i -g corepack@latest

RUN corepack enable

RUN apt-get update \
    && apt-get install -y --no-install-recommends python3 ca-certificates bash curl make g++ \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

FROM base AS builder

WORKDIR /usr/src/app

ARG NATIVE_LIBS_RELEASE_TAG
ENV NATIVE_LIBS_RELEASE_TAG=${NATIVE_LIBS_RELEASE_TAG}

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./

COPY ./postman/package.json ./postman/package.json
COPY ./sdk/sdk-ethers/package.json ./sdk/sdk-ethers/package.json
COPY ./ts-libs/linea-native-libs/package.json ./ts-libs/linea-native-libs/package.json
COPY ./ts-libs/linea-shared-utils/package.json ./ts-libs/linea-shared-utils/package.json

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts

COPY ./postman ./postman
COPY ./sdk/sdk-ethers ./sdk/sdk-ethers
COPY ts-libs/linea-native-libs ./ts-libs/linea-native-libs
COPY ts-libs/linea-shared-utils ./ts-libs/linea-shared-utils

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm run build \
    && pnpm deploy --legacy --filter=./postman --prod ./prod/postman

FROM node:${NODE_VERSION}-slim AS production

ENV NODE_ENV=production

WORKDIR /usr/src/app

USER node

COPY --from=builder /usr/src/app/prod/postman ./postman

CMD [ "node", "./postman/dist/scripts/runPostman.js" ]
