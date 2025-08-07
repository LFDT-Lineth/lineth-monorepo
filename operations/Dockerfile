FROM node:lts-slim AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

RUN corepack enable

FROM base AS builder

WORKDIR /usr/src/app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./

COPY operations/ ./operations/

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts
RUN pnpm run -F operations build
RUN pnpm deploy --filter=./operations --prod ./prod

FROM node:lts-slim AS release

ENV NODE_ENV=production

WORKDIR /usr/src/app

USER node

COPY --from=builder /usr/src/app/prod ./

ENTRYPOINT ["./bin/run.js"]