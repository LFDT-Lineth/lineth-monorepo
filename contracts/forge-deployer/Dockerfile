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
COPY --from=build --chown=node:node \
  /workspace/contracts/local-deployments-artifacts/dynamic-artifacts/IntegrationTestTrueVerifier.json \
  ./dist/artifacts/dynamic-artifacts/IntegrationTestTrueVerifier.json

USER node
ENTRYPOINT ["node", "dist/cli.js"]
