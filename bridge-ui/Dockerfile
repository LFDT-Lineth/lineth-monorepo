FROM node:lts-alpine AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS builder

ARG NEXT_PUBLIC_WALLET_CONNECT_ID
ARG NEXT_PUBLIC_INFURA_ID
ARG NEXT_PUBLIC_DYNAMIC_ENVIRONMENT_ID
ARG NEXT_PUBLIC_LIFI_API_KEY
ARG NEXT_PUBLIC_ONRAMPER_API_KEY
ENV NEXT_PUBLIC_WALLET_CONNECT_ID=$NEXT_PUBLIC_WALLET_CONNECT_ID
ENV NEXT_PUBLIC_INFURA_ID=$NEXT_PUBLIC_INFURA_ID
ENV NEXT_PUBLIC_DYNAMIC_ENVIRONMENT_ID=$NEXT_PUBLIC_DYNAMIC_ENVIRONMENT_ID
ENV NEXT_PUBLIC_LIFI_API_KEY=$NEXT_PUBLIC_LIFI_API_KEY
ENV NEXT_PUBLIC_ONRAMPER_API_KEY=$NEXT_PUBLIC_ONRAMPER_API_KEY
ARG ENV_FILE

WORKDIR /app

RUN mkdir -p bridge-ui

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json .eslintrc.js .prettierrc.js ./

COPY ./bridge-ui ./bridge-ui
COPY $ENV_FILE ./bridge-ui/.env.production

RUN --mount=type=cache,id=pnpm,target=/pnpm/store apk add --virtual build-dependencies --no-cache python3 make g++ \
    && pnpm install --frozen-lockfile --prefer-offline \
    && pnpm run -F bridge-ui build \
    && apk del build-dependencies

FROM node:lts-alpine AS runner

ARG X_TAG

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

USER node

COPY --from=builder --chown=node:node /app/bridge-ui/.next/standalone ./
COPY --from=builder --chown=node:node /app/bridge-ui/public ./bridge-ui/public
COPY --from=builder --chown=node:node /app/bridge-ui/.next/static ./bridge-ui/.next/static

CMD ["node", "./bridge-ui/server.js"]