ARG NODE_VERSION=24.18.0

FROM node:${NODE_VERSION}-bookworm-slim AS build

ENV PNPM_HOME=/pnpm
ENV PATH=${PNPM_HOME}:${PATH}
RUN corepack enable && corepack prepare pnpm@11.9.0 --activate

WORKDIR /workspace
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml tsconfig.json .npmrc ./
COPY patches ./patches
COPY ts-libs/eslint-config ./ts-libs/eslint-config
COPY contracts ./contracts

RUN pnpm install --frozen-lockfile --filter @lfdt-lineth/forge-contract-deployer...
RUN pnpm --filter @lfdt-lineth/forge-contract-deployer build

# Custom bootstrap scripts run as plain Node children and may `require("ethers")`.
# The runtime image strips package managers, so do a self-contained isolated
# install of ethers whose node_modules those children can resolve. The version
# is read from pnpm-workspace.yaml's catalog (already COPY'd above) rather than
# hardcoded here a second time, so it can never silently drift from the version
# the rest of the deployer builds against.
RUN set -eux; \
  ethers_version="$(grep -oE '^[[:space:]]*ethers:[[:space:]]*[^[:space:]]+' pnpm-workspace.yaml | awk '{print $2}')"; \
  if [ -z "$ethers_version" ]; then \
    echo "failed to resolve the ethers catalog version from pnpm-workspace.yaml" >&2; \
    exit 1; \
  fi; \
  mkdir -p /bootstrap-libs/pkg; \
  printf '{ "name": "bootstrap-libs", "version": "0.0.0", "private": true, "dependencies": { "ethers": "%s" } }' "$ethers_version" \
    > /bootstrap-libs/pkg/package.json; \
  cd /bootstrap-libs/pkg; \
  COREPACK_ENABLE_STRICT=0 pnpm install --ignore-workspace --node-linker=isolated \
    --modules-dir /bootstrap-libs/node_modules --virtual-store-dir /bootstrap-libs/node_modules/.pnpm; \
  node -e "process.env.NODE_PATH='/bootstrap-libs/node_modules'; require('module').Module._initPaths(); const e=require('ethers'); console.log('ethers resolvable for bootstrap scripts:', e.version)"

FROM node:${NODE_VERSION}-bookworm-slim AS runtime

ENV NODE_ENV=production
WORKDIR /app

# The runner only needs the Node.js binary. Remove bundled package managers
# and their dependency trees from the runtime attack surface.
RUN rm -rf \
  /opt/yarn-* \
  /usr/local/lib/node_modules/corepack \
  /usr/local/lib/node_modules/npm \
  /usr/local/bin/corepack \
  /usr/local/bin/npm \
  /usr/local/bin/npx \
  /usr/local/bin/yarn \
  /usr/local/bin/yarnpkg

COPY --from=build --chown=node:node /workspace/contracts/forge-deployer/dist ./dist
COPY --from=build --chown=node:node /bootstrap-libs/node_modules ./bootstrap-libs/node_modules
ENV BOOTSTRAP_LIB_NODE_PATH=/app/bootstrap-libs/node_modules
COPY --from=build --chown=node:node \
  /workspace/contracts/local-deployments-artifacts/dynamic-artifacts/IntegrationTestTrueVerifier.json \
  ./dist/artifacts/dynamic-artifacts/IntegrationTestTrueVerifier.json

USER node
ENTRYPOINT ["node", "dist/cli.js"]
