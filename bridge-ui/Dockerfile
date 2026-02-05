ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-alpine AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS builder

RUN apk add --no-cache python3 make g++

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json prettier.config.mjs .prettierignore ./
COPY ./bridge-ui/package.json ./bridge-ui/package.json
COPY ./sdk/sdk-core/package.json ./sdk/sdk-core/package.json
COPY ./sdk/sdk-viem/package.json ./sdk/sdk-viem/package.json
COPY ./ts-libs/eslint-config/package.json ./ts-libs/eslint-config/package.json

RUN --mount=type=cache,id=pnpm,target=/pnpm/store \
    pnpm install --frozen-lockfile --prefer-offline

COPY ./sdk/sdk-core ./sdk/sdk-core
COPY ./sdk/sdk-viem ./sdk/sdk-viem
COPY ./ts-libs/eslint-config ./ts-libs/eslint-config
COPY ./bridge-ui ./bridge-ui

ARG NEXT_PUBLIC_WALLET_CONNECT_ID
ARG NEXT_PUBLIC_INFURA_ID
ARG NEXT_PUBLIC_ALCHEMY_API_KEY
ARG NEXT_PUBLIC_QUICKNODE_ID
ARG NEXT_PUBLIC_WEB3_AUTH_CLIENT_ID
ARG NEXT_PUBLIC_CONNECTOR_GOOGLE_ID
ARG NEXT_PUBLIC_CONNECTOR_PASSWORDLESS_ID
ARG NEXT_PUBLIC_CONNECTOR_GROUPED_ID
ARG NEXT_PUBLIC_SOCIAL_LOGIN_ENABLED
ARG NEXT_PUBLIC_LIFI_API_KEY
ARG NEXT_PUBLIC_LIFI_INTEGRATOR_NAME
ARG NEXT_PUBLIC_ONRAMPER_API_KEY
ARG NEXT_PUBLIC_LAYERSWAP_API_KEY
ARG NEXT_PUBLIC_ENVIRONMENT
ARG ENV_FILE

ENV NEXT_PUBLIC_WALLET_CONNECT_ID=$NEXT_PUBLIC_WALLET_CONNECT_ID \
    NEXT_PUBLIC_INFURA_ID=$NEXT_PUBLIC_INFURA_ID \
    NEXT_PUBLIC_ALCHEMY_API_KEY=$NEXT_PUBLIC_ALCHEMY_API_KEY \
    NEXT_PUBLIC_QUICKNODE_ID=$NEXT_PUBLIC_QUICKNODE_ID \
    NEXT_PUBLIC_WEB3_AUTH_CLIENT_ID=$NEXT_PUBLIC_WEB3_AUTH_CLIENT_ID \
    NEXT_PUBLIC_CONNECTOR_GOOGLE_ID=$NEXT_PUBLIC_CONNECTOR_GOOGLE_ID \
    NEXT_PUBLIC_CONNECTOR_PASSWORDLESS_ID=$NEXT_PUBLIC_CONNECTOR_PASSWORDLESS_ID \
    NEXT_PUBLIC_CONNECTOR_GROUPED_ID=$NEXT_PUBLIC_CONNECTOR_GROUPED_ID \
    NEXT_PUBLIC_SOCIAL_LOGIN_ENABLED=$NEXT_PUBLIC_SOCIAL_LOGIN_ENABLED \
    NEXT_PUBLIC_LIFI_API_KEY=$NEXT_PUBLIC_LIFI_API_KEY \
    NEXT_PUBLIC_LIFI_INTEGRATOR_NAME=$NEXT_PUBLIC_LIFI_INTEGRATOR_NAME \
    NEXT_PUBLIC_ONRAMPER_API_KEY=$NEXT_PUBLIC_ONRAMPER_API_KEY \
    NEXT_PUBLIC_LAYERSWAP_API_KEY=$NEXT_PUBLIC_LAYERSWAP_API_KEY \
    NEXT_PUBLIC_ENVIRONMENT=$NEXT_PUBLIC_ENVIRONMENT

COPY $ENV_FILE ./bridge-ui/.env.production

RUN --mount=type=secret,id=revalidate_secret,env=REVALIDATE_SECRET \
    --mount=type=secret,id=contentful_space_id,env=CONTENTFUL_SPACE_ID \
    pnpm run build

FROM node:${NODE_VERSION}-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

USER node

COPY --from=builder --chown=node:node /app/bridge-ui/.next/standalone ./
COPY --from=builder --chown=node:node /app/bridge-ui/public ./bridge-ui/public
COPY --from=builder --chown=node:node /app/bridge-ui/.next/static ./bridge-ui/.next/static

CMD ["node", "./bridge-ui/server.js"]
