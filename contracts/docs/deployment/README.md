# Linea Deployment Scripts

## Environment Variables Naming Convention

Environment variables follow a consistent naming pattern:

| Prefix | Usage |
|--------|-------|
| `L1_` | Ethereum L1 (e.g. `L1_SECURITY_COUNCIL`, `L1_RPC_URL`) |
| `L2_` | Linea L2 (e.g. `L2_SECURITY_COUNCIL`, `L2_RPC_URL`, `L2_MESSAGE_SERVICE_ADDRESS`) |
| `LINEA_ROLLUP_*` | Linea Rollup contract (L1) — product-specific |

**Shared per layer:**

- `L1_SECURITY_COUNCIL` — shared across all L1 contracts (Linea Rollup, Validium, Token Bridge L1, RecoverFunds, Yield Manager)
- `L2_SECURITY_COUNCIL` — shared across all L2 contracts (L2 Message Service, Rollup Revenue Vault, Token Bridge L2)

**Shared L1 base (Linea Rollup & Validium):** `INITIAL_L2_STATE_ROOT_HASH`, `INITIAL_L2_BLOCK_NUMBER`, `L2_GENESIS_TIMESTAMP` — common to both products via shared contract base

**Product-specific:** `LINEA_ROLLUP_OPERATORS`, `LINEA_ROLLUP_RATE_LIMIT_*`, `VALIDIUM_OPERATORS`, `VALIDIUM_RATE_LIMIT_*`

**RPC endpoints:** `L1_RPC_URL`, `L2_RPC_URL`, `CUSTOM_RPC_URL` (replaces legacy `BLOCKCHAIN_NODE`, `L2_BLOCKCHAIN_NODE`, `CUSTOM_BLOCKCHAIN_URL`)

<br />

This document aims to explain how to get started with deploying the Linea deployment scripts. There are several ways the scripts can be executed dependent on: 
- If you're storing deployment variables in an environment file (.env)
- If you plan to deploy an individual script which will deploy a single contract.
- If you plan to deploy a chained deployment script that will include multiple contracts.

<br />
Running the script with an .env file set, you will need to make sure that the correct variables are set in the .env file, considering the network that you're deploying on. In this way when the script is being run, it will take the variables it needs to execute the script from that .env file. <br />
<br />

Running the script without an .env file will require you to place the variables as command-line arguments.
The command-line arguments will create or replace existing .env (only in memory) environment variables. If the variables are provided in the terminal as command-line arguments, they will have priority over the same variables if they are defined in the .env file. These need not exist in the .env file.

Furthermore, you can also specify a general set of variables in the .env file (VERIFY_CONTRACT, DEPLOYER_PRIVATE_KEY, ETHERSCAN_API_KEY, INFURA_API_KEY) and provide only the script-specific variables as command-line arguments, when you run each script.

Setting `VERIFY_CONTRACT=true` will start the verifying stage after the contract is deployed, provided that there is a `ETHERSCAN_API_KEY` available in the .env or provided as CLI argument.

<br />

## Network Specific Variables

Dependent on which network you are using, the corresponding API Key or RPC URL must be set.  The block explorer parameter name may also differ. The following table highlights which deployer key and API config is used per network. For all networks, use `DEPLOYER_PRIVATE_KEY` (e.g. `DEPLOYER_PRIVATE_KEY=<key> INFURA_API_KEY=<key>`).

| Network       | Private key parameter name   | API Key / RPC URL | Block explorer parameter name |
| ------------- | ----------------- | ---- | ----------------- | 
| sepolia    | DEPLOYER_PRIVATE_KEY    | INFURA_API_KEY  | ETHERSCAN_API_KEY |
| linea_sepolia | DEPLOYER_PRIVATE_KEY   | INFURA_API_KEY  | ETHERSCAN_API_KEY |
| mainnet   | DEPLOYER_PRIVATE_KEY | INFURA_API_KEY | ETHERSCAN_API_KEY |
| linea_mainnet | DEPLOYER_PRIVATE_KEY |  INFURA_API_KEY  | ETHERSCAN_API_KEY |
| custom    | DEPLOYER_PRIVATE_KEY | CUSTOM_RPC_URL | ETHERSCAN_API_KEY |
| zkevm_dev | DEPLOYER_PRIVATE_KEY | L1_RPC_URL or L2_RPC_URL | n/a |

<br />

## Generalized Command Format

```shell
<possible CLI environment arguments> npx hardhat deploy --network sepolia --tags <contract tags, comma delimitted list>
```

<br />

## Order of Precedence

 When deploying, if required variables such as deployed contract addresses are not defined in the .env or provided as CLI arguments, the script will look and check if it can use the addresses stored in the deployments/<network_name>/ folder. 
 <br />
 The order of priority (unless specified otherwise) will be:
 - CLI arguments, 
 - .env variables ,
 - deployments/<network_name>/

<br />

## Deployments

### L1 Contracts (Ethereum)

| Contract | Doc | Tags |
|----------|-----|------|
| PlonkVerifier | [verifier.md](l1/verifier.md) | `PlonkVerifier` |
| LineaRollup | [linea-rollup.md](l1/linea-rollup.md) | `LineaRollup`, `LineaRollupWithReinitialization`, `LineaRollupV8WithReinitialization` |
| Validium | [validium.md](l1/validium.md) | `Validium` |
| Timelock | [timelock.md](l1/timelock.md) | `Timelock` |
| YieldManager | [yield-manager.md](l1/yield-manager.md) | `YieldManager`, `YieldManagerArtifacts`, `YieldManagerImplementation` |
| RecoverFunds | [recover-funds.md](l1/recover-funds.md) | `RecoverFunds` |
| CallForwardingProxy | [call-forwarding-proxy.md](l1/call-forwarding-proxy.md) | `CallForwardingProxy` |
| L1LineaTokenBurner | [l1-linea-token-burner.md](l1/l1-linea-token-burner.md) | `L1LineaTokenBurner` |

### L2 Contracts (Linea)

| Contract | Doc | Tags |
|----------|-----|------|
| L2MessageService | [l2-message-service.md](l2/l2-message-service.md) | `L2MessageService`, `L2MessageServiceLineaMainnet`, `L2MessageServiceWithReinitialization` |
| L2 Predeploys | [predeploys.md](l2/predeploys.md) | `EIP2935SystemContract`, `EIP4788SystemContract`, `UpgradeableWithdrawalQueuePredeploy`, `UpgradeableConsolidationQueuePredeploy`, `UpgradeableBeaconChainDepositPredeploy` |
| RollupRevenueVault | [rollup-revenue-vault.md](l2/rollup-revenue-vault.md) | `RollupRevenueVault`, `RollupRevenueVaultWithReinitialization` |
| V3DexSwapAdapter | [v3-dex-swap-adapter.md](l2/v3-dex-swap-adapter.md) | `V3DexSwapAdapter` |
| LineaSequencerUptimeFeed | [sequencer-uptime-feed.md](l2/sequencer-uptime-feed.md) | `LineaSequencerUptimeFeed` |

### Dual-Chain Contracts (L1 or L2)

| Contract | Doc | Tags |
|----------|-----|------|
| TokenBridge | [token-bridge.md](dual-chain/token-bridge.md) | `BridgedToken`, `CustomBridgedToken`, `TokenBridge`, `TokenBridgeWithReinitialization` |

### Generic / Testing

| Contract | Doc | Tags |
|----------|-----|------|
| ImplementationForProxy | [implementation-for-proxy.md](generic/implementation-for-proxy.md) | `ImplementationForProxy` |
| PaymentSplitterWrapper | [payment-splitter.md](generic/payment-splitter.md) | `PaymentSplitterWrapper` |
| TestERC20 | [test-erc20.md](generic/test-erc20.md) | `TestERC20` |

### Chained Deployments

Multi-contract deployment sequences: [chained-deployments.md](chained-deployments.md)
