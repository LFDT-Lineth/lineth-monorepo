ARG NODE_VERSION=lts

FROM node:${NODE_VERSION}-slim AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

RUN corepack enable

FROM base AS builder

WORKDIR /usr/src/app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY patches/ ./patches/
COPY ts-libs/eslint-config/ ./ts-libs/eslint-config/
COPY operations/cli/ ./operations/cli/

RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts
RUN pnpm run -F operations build
RUN pnpm --config.allow-unused-patches=true --config.block-exotic-subdeps=false deploy --legacy --filter=./operations/cli --prod ./prod

FROM node:${NODE_VERSION}-slim AS release

ENV NODE_ENV=production

WORKDIR /usr/src/app

USER node

COPY --from=builder /usr/src/app/prod ./

ENTRYPOINT ["./bin/run.js"]

# Build-time metadata as defined at http://label-schema.org
# Values are injected by scripts/docker/build-image.sh.
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="linea-alltools" \
	org.label-schema.description="Lineth operations CLI" \
	org.label-schema.url="https://consensys.io/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/LFDT-Lineth/lineth-monorepo" \
	org.label-schema.vendor="ConsenSys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0"
