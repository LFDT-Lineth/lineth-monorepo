# Linea Bridge UI

## Deployment

### Config

The config file `.env.production` is used for public configuration variables.

Private configuration variables are store on GitHub Secrets.

### Get a Frontend Tag

- Retrieve an existing tag.
  - Get a Tag from [GitHub Actions](https://github.com/Consensys/linea-monorepo/actions) on `Bridge UI Build and Publish` job, `Set Docker Tag` action.
- Or create a new tag.
  - Create a PR and merge the last version to develop branch, and get a Tag from [GitHub Actions](https://github.com/Consensys/linea-monorepo/actions) on `Bridge UI Build and Publish` job, `Set Docker Tag` action.

Example:

In `Set Docker Tag`

```
Run echo "DOCKER_TAG=${GITHUB_SHA:0:7}-$(date +%s)-bridge-ui-${{ steps.package-version.outputs.current-version }}" | tee $GITHUB_ENV
DOCKER_TAG=f3afe33-1705598198-bridge-ui-0.5.3
```

The Tag is `f3afe33-1705598198-bridge-ui-0.5.3`

### Deployment on Dev

To publish updates to [https://bridge.dev.linea.build/](https://bridge.dev.linea.build/):

#### Update a Frontend Tag on Linea cluster

1. Get a Frontend Tag
2. Go to [zk-apps-dev](https://github.com/Consensys/zk-apps-dev) project, create a branch from `main` branch.
3. Modify [values.yaml](https://github.com/ConsenSys/zk-apps-dev/blob/main/argocd/bridge-ui/values.yaml) by replacing it with the specified tag.

```
---
image:
  bridge_ui:
    repository: consensys/bridge-ui
    tag: f3afe33-1705598198-bridge-ui-0.5.3
    [...]
```

- Push the branch and create a merge request

The update should appear on [https://bridge.dev.linea.build/](https://bridge.dev.linea.build/).

#### Check ArgoCD deployment

To check the deployment, go to: [argocd.dev.zkevm.consensys.net](https://argocd.dev.zkevm.consensys.net/applications/argocd/bridge-ui?resource=)

Access are in [1password](https://consensys.1password.com/vaults/blu7ljyqd5zgkj5vbjlmooayya/allitems/ccljvnh2hgs6jh5zc4s5apww4i)

### Deployment on Production

To publish updates to [https://bridge.linea.build/](https://bridge.linea.build/):

#### Update a Frontend Tag on Linea cluster

1. Get a Frontend Tag
2. Go to [zk-apps-prod](https://github.com/Consensys/zk-apps-prod) project, create a branch from `main` branch.
3. Modify [values.yaml](https://github.com/Consensys/zk-apps-prod/blob/main/argocd/bridge-ui/values.yaml) by replacing it with the specified tag.

Example:

```
---
image:
  bridge_ui:
    repository: consensys/bridge-ui
    tag: f3afe33-1705598198-bridge-ui-0.5.3
    [...]
```

3. Push the branch and create a merge request

The update should appear on [https://bridge.linea.build/](https://bridge.linea.build/).

## Development

### Run development server

To start Linea Bridge UI for development:

1. Create a `.env` file by copying `.env.template` and add your private API keys.

```shell
cp .env.template .env
```

2. Install packages:

```shell
pnpm i
```

3. Start the development server, execute:

```shell
pnpm run dev
```

Frontend should be available at: http://localhost:3000

### Build and test Docker image

Commands to test locally the Docker image used in production.

Build the image:

```shell
# build local image
docker build --build-arg ENV_FILE=.env.production -t linea/bridge-ui .
```

Replace with `ENV_FILE=.env.production` to test dev env.

Run the image:

```shell
# run local image
docker run -p 3000:3000 linea/bridge-ui
```

Frontend should be available at: http://localhost:3000

### End to end tests

E2E tests are run in the CI but can also be run locally.
Make sure to set the`NEXT_PUBLIC_E2E_TEST_MODE=true` .env

1. Copy `.env.template` to `.env` file
2. Build the Bridge UI in local in a terminal: `pnpm run build`
3. Start the local docker stack by running the following command from the root folder: `make start-env-with-tracing-v2-ci`
3. Run the command: `pnpm run test:e2e:headful` in another terminal

## Config

The config variables are:

| Var                                           | Description                                    | Values                                                                                                    |
|-----------------------------------------------|------------------------------------------------| --------------------------------------------------------------------------------------------------------- |
| NEXT_PUBLIC_MAINNET_L1_TOKEN_BRIDGE           | Linea Token Bridge on Ethereum mainnet         | 0x051F1D88f0aF5763fB888eC4378b4D8B29ea3319                                                                |
| NEXT_PUBLIC_MAINNET_LINEA_TOKEN_BRIDGE        | Linea Token Bridge on Linea mainnet            | 0x353012dc4a9A6cF55c941bADC267f82004A8ceB9                                                                |
| NEXT_PUBLIC_MAINNET_L1_MESSAGE_SERVICE        | Linea Message Service on Ethereum mainnet      | 0xd19d4B5d358258f05D7B411E21A1460D11B0876F                                                                |
| NEXT_PUBLIC_MAINNET_LINEA_MESSAGE_SERVICE     | Linea Message Service on Linea mainnet         | 0x508Ca82Df566dCD1B0DE8296e70a96332cD644ec                                                                |
| NEXT_PUBLIC_MAINNET_L1_USDC_BRIDGE            | Linea USDC Bridge on Ethereum mainnet          | 0x504A330327A089d8364C4ab3811Ee26976d388ce                                                                |
| NEXT_PUBLIC_MAINNET_LINEA_USDC_BRIDGE         | Linea USDC Bridge on Linea mainnet             | 0xA2Ee6Fce4ACB62D95448729cDb781e3BEb62504A                                                                |
| NEXT_PUBLIC_MAINNET_GAS_ESTIMATED             | Linea gas estimated on mainnet                 | 100000                                                                                                    |
| NEXT_PUBLIC_MAINNET_DEFAULT_GAS_LIMIT_SURPLUS | Linea gas limit surplus on mainnet             | 6000                                                                                                      |
| NEXT_PUBLIC_MAINNET_PROFIT_MARGIN             | Linea profit margin on mainnet                 | 2                                                                                                         |
| NEXT_PUBLIC_MAINNET_TOKEN_LIST                | Linea Token list on mainnet                    | https://raw.githubusercontent.com/Consensys/linea-token-list/main/json/linea-mainnet-token-shortlist.json |
|                                               |                                                |                                                                                                           |
| NEXT_PUBLIC_SEPOLIA_L1_TOKEN_BRIDGE           | Linea Token Bridge on Ethereum Sepolia         | 0x5A0a48389BB0f12E5e017116c1105da97E129142                                                                |
| NEXT_PUBLIC_SEPOLIA_LINEA_TOKEN_BRIDGE        | Linea Token Bridge on Linea Sepolia            | 0x93DcAdf238932e6e6a85852caC89cBd71798F463                                                                |
| NEXT_PUBLIC_SEPOLIA_L1_MESSAGE_SERVICE        | Linea Message Service on Ethereum Sepolia      | 0xB218f8A4Bc926cF1cA7b3423c154a0D627Bdb7E5                                                                |
| NEXT_PUBLIC_SEPOLIA_LINEA_MESSAGE_SERVICE     | Linea Message Service on Linea Sepolia         | 0x971e727e956690b9957be6d51Ec16E73AcAC83A7                                                                |
| NEXT_PUBLIC_SEPOLIA_L1_USDC_BRIDGE            | Linea USDC Bridge on Ethereum Sepolia          | 0x32D123756d32d3eD6580935f8edF416e57b940f4                                                                |
| NEXT_PUBLIC_SEPOLIA_LINEA_USDC_BRIDGE         | Linea USDC Bridge on Linea Sepolia             | 0xDFa112375c9be9D124932b1d104b73f888655329                                                                |
| NEXT_PUBLIC_SEPOLIA_GAS_ESTIMATED             | Linea gas estimated on Sepolia                 | 6100000000                                                                                                |
| NEXT_PUBLIC_SEPOLIA_DEFAULT_GAS_LIMIT_SURPLUS | Linea gas limit surplus on Sepolia             | 6000                                                                                                      |
| NEXT_PUBLIC_SEPOLIA_PROFIT_MARGIN             | Linea profit margin on Sepolia                 | 2                                                                                                         |
| NEXT_PUBLIC_SEPOLIA_TOKEN_LIST                | Linea Token list on Sepolia                    | https://raw.githubusercontent.com/Consensys/linea-token-list/main/json/linea-sepolia-token-shortlist.json |
|                                               |                                                |                                                                                                           |
| NEXT_PUBLIC_WALLET_CONNECT_ID                 | Wallet Connect Api Key                         |                                                                                                           |
| NEXT_PUBLIC_INFURA_ID                         | Infura API Key                                 |                                                                                                           |
| NEXT_PUBLIC_ALCHEMY_API_KEY                   | Alchemy API Key                                |                                                                                                           |
| NEXT_PUBLIC_STORAGE_MIN_VERSION               | Local storage version for reseting the storage | 1                                                                                                         |
| NEXT_PUBLIC_WEB3_AUTH_CLIENT_ID               | Web3Auth client key                            |                                                                                                           |
| NEXT_PUBLIC_CONNECTOR_GOOGLE_ID               | Web3Auth Google OAuth connection ID (staging)  | Obtained from Web3Auth dashboard                                                                          |
| NEXT_PUBLIC_CONNECTOR_PASSWORDLESS_ID         | Web3Auth Email passwordless connection ID      | Obtained from Web3Auth dashboard                                                                          |
| NEXT_PUBLIC_CONNECTOR_GROUPED_ID              | Web3Auth Grouped auth connection ID            | Links multiple auth methods to same account                                                               |
| NEXT_PUBLIC_QUICKNODE_ID                      | QuickNode API key                              |                                                                                                           |
| NEXT_PUBLIC_LIFI_API_KEY                      | LiFi API key                                   |                                                                                                           |
| NEXT_PUBLIC_ONRAMPER_API_KEY                  | OnRamper API key                               |                                                                                                           |
| NEXT_PUBLIC_IS_CCTP_ENABLED                   | Feature flag for CCTPV2                        | false                                                                                                     |
| NEXT_PUBLIC_LAYERSWAP_API_KEY                 | LayerSwap API key                              |                                                                                                           |

## About

This is a [Next.js](https://nextjs.org/) project bootstrapped with [`create-next-app`](https://github.com/vercel/next.js/tree/canary/packages/create-next-app).
