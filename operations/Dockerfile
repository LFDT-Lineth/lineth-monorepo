ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-slim AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

RUN corepack enable

FROM base AS builder

WORKDIR /usr/src/app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./

COPY ts-libs/eslint-config/ ./ts-libs/eslint-config/
COPY operations/ ./operations/

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts
RUN pnpm run -F operations build
RUN pnpm deploy --legacy --filter=./operations --prod ./prod

FROM node:${NODE_VERSION}-slim AS release

ENV NODE_ENV=production

WORKDIR /usr/src/app

USER node

COPY --from=builder /usr/src/app/prod ./

ENTRYPOINT ["./bin/run.js"]
