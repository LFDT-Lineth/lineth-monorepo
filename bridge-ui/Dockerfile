ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-alpine AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
RUN corepack enable

FROM base AS builder

ARG NEXT_PUBLIC_WALLET_CONNECT_ID
ARG NEXT_PUBLIC_INFURA_ID
ARG NEXT_PUBLIC_ALCHEMY_API_KEY
ARG NEXT_PUBLIC_QUICKNODE_ID
ARG NEXT_PUBLIC_WEB3_AUTH_CLIENT_ID
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
    NEXT_PUBLIC_LIFI_API_KEY=$NEXT_PUBLIC_LIFI_API_KEY \
    NEXT_PUBLIC_LIFI_INTEGRATOR_NAME=$NEXT_PUBLIC_LIFI_INTEGRATOR_NAME \
    NEXT_PUBLIC_ONRAMPER_API_KEY=$NEXT_PUBLIC_ONRAMPER_API_KEY \
    NEXT_PUBLIC_LAYERSWAP_API_KEY=$NEXT_PUBLIC_LAYERSWAP_API_KEY \
    NEXT_PUBLIC_ENVIRONMENT=$NEXT_PUBLIC_ENVIRONMENT

WORKDIR /app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY ./bridge-ui/package.json ./bridge-ui/package.json
COPY ./sdk/sdk-core ./sdk/sdk-core
COPY ./sdk/sdk-viem ./sdk/sdk-viem

RUN --mount=type=cache,id=pnpm,target=/pnpm/store apk add --virtual build-dependencies --no-cache python3 make g++ \
    && pnpm install --frozen-lockfile --prefer-offline

COPY ./bridge-ui ./bridge-ui

COPY $ENV_FILE ./bridge-ui/.env.production

RUN --mount=type=secret,id=revalidate_secret,env=REVALIDATE_SECRET \
    --mount=type=secret,id=contentful_space_id,env=CONTENTFUL_SPACE_ID \
    pnpm run -F bridge-ui build

RUN apk del build-dependencies

FROM node:${NODE_VERSION}-alpine AS runner

WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

USER node

COPY --from=builder --chown=node:node /app/bridge-ui/.next/standalone ./
COPY --from=builder --chown=node:node /app/bridge-ui/public ./bridge-ui/public
COPY --from=builder --chown=node:node /app/bridge-ui/.next/static ./bridge-ui/.next/static

CMD ["node", "./bridge-ui/server.js"]
