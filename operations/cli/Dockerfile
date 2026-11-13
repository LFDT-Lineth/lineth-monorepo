ARG NODE_VERSION=24.22.1
# Digest of node:${NODE_VERSION}-slim, kept in sync with .nvmrc. If you override
# NODE_VERSION at build time, override NODE_IMAGE_DIGEST to match.
ARG NODE_IMAGE_DIGEST=sha256:be8204d4bd327363357f532187b752ff62759c31bedf98e9e18fc08c9cbd605c

FROM node:${NODE_VERSION}-slim@${NODE_IMAGE_DIGEST} AS base

ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"

RUN corepack enable

FROM base AS builder

WORKDIR /usr/src/app

COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json ./
COPY patches/ ./patches/
COPY ts-libs/eslint-config/ ./ts-libs/eslint-config/
COPY operations/cli/ ./operations/cli/

# kics-scan ignore-line
# Versions are pinned by pnpm-lock.yaml (--frozen-lockfile); this is a workspace
# install from the lockfile, not a single-package add. KICS: NPM Install Without Pinned Version.
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile --prefer-offline --ignore-scripts
RUN pnpm run -F operations build
RUN pnpm --config.allow-unused-patches=true --config.block-exotic-subdeps=false deploy --legacy --filter=./operations/cli --prod ./prod

FROM node:${NODE_VERSION}-slim@${NODE_IMAGE_DIGEST} AS release

ENV NODE_ENV=production

WORKDIR /usr/src/app

USER node

COPY --from=builder /usr/src/app/prod ./
COPY LICENSE-APACHE LICENSE-MIT /licenses/

ENTRYPOINT ["./bin/run.js"]

# Build-time metadata as defined at http://label-schema.org
# Values are injected by scripts/docker/build-image.sh.
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL org.label-schema.build-date=$BUILD_DATE \
	org.label-schema.name="linea-alltools" \
	org.label-schema.description="Lineth operations CLI" \
	org.label-schema.url="https://consensys.com/" \
	org.label-schema.vcs-ref=$VCS_REF \
	org.label-schema.vcs-url="https://github.com/LFDT-Lineth/lineth-monorepo" \
	org.label-schema.vendor="Consensys" \
	org.label-schema.version=$VERSION \
	org.label-schema.schema-version="1.0" \
	org.opencontainers.image.licenses="MIT OR Apache-2.0"
