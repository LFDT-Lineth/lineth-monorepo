# L2 Predeploy Contracts

[← Back to index](../README.md)

<br />

These are system-level contracts deployed to predetermined addresses on L2. They implement various EIPs and provide infrastructure functionality.

<br />

## EIP2935SystemContract

The EIP2935SystemContract is a system contract for historical block hashes according to [EIP-2935](https://github.com/ethereum/EIPs/blob/master/EIPS/eip-2935.md). This contract deploys to a predetermined address using a specific deployment transaction format. The deployment script automatically funds the required sender address if needed.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name             | Required | Input Value | Description |
| -------------------------- | -------- | ---------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| CUSTOM_RPC_URL     | true     | url | L2 RPC URL endpoint |

<br />

**Prerequisites:**
- The deployment script automatically funds the predetermined sender address (0x3462413Af4609098e1E27A490f554f260213D685) if needed
- The contract deploys to the fixed address: 0x0000F90827F1C53a10cb7A02335B175320002935

Base command:
```shell
npx hardhat deploy --network custom --tags EIP2935SystemContract
```

Base command with cli arguments:

```shell
DEPLOYER_PRIVATE_KEY=<key> CUSTOM_RPC_URL=<l2_rpc_url> npx hardhat deploy --network custom --tags EIP2935SystemContract
```

(make sure to replace `<key>` with actual values)

<br />

## EIP4788SystemContract

The EIP4788SystemContract is a system contract for the beacon block root according to [EIP-4788](https://github.com/ethereum/EIPs/blob/master/EIPS/eip-4788.md). Like EIP2935, this contract deploys to a predetermined address using a specific deployment transaction format.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name             | Required | Input Value | Description |
| -------------------------- | -------- | ---------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| CUSTOM_RPC_URL     | true     | url | L2 RPC URL endpoint |

<br />

**Prerequisites:**
- The deployment script automatically funds the predetermined sender address if needed

Base command:
```shell
npx hardhat deploy --network custom --tags EIP4788SystemContract
```

Base command with cli arguments:

```shell
DEPLOYER_PRIVATE_KEY=<key> CUSTOM_RPC_URL=<l2_rpc_url> npx hardhat deploy --network custom --tags EIP4788SystemContract
```

(make sure to replace `<key>` with actual values)

<br />

## UpgradeableWithdrawalQueuePredeploy

The UpgradeableWithdrawalQueuePredeploy is an upgradeable predeploy contract that implements EIP-7002 execution layer triggerable withdrawals. This deploys as a placeholder implementation that can be upgraded later to provide full functionality.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name             | Required | Input Value | Description |
| -------------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags UpgradeableWithdrawalQueuePredeploy
```

Base command with cli arguments:

```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> npx hardhat deploy --network linea_sepolia --tags UpgradeableWithdrawalQueuePredeploy
```

(make sure to replace `<key>` with actual values)

<br />

## UpgradeableConsolidationQueuePredeploy

The UpgradeableConsolidationQueuePredeploy is an upgradeable predeploy contract that implements EIP-7251 execution layer consolidation requests. This deploys as a placeholder implementation that can be upgraded later to provide full functionality.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name             | Required | Input Value | Description |
| -------------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags UpgradeableConsolidationQueuePredeploy
```

Base command with cli arguments:

```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> npx hardhat deploy --network linea_sepolia --tags UpgradeableConsolidationQueuePredeploy
```

(make sure to replace `<key>` with actual values)

<br />

## UpgradeableBeaconChainDepositPredeploy

The UpgradeableBeaconChainDepositPredeploy is an upgradeable predeploy contract that implements EIP-6110 execution layer deposits. This deploys as a placeholder implementation that can be upgraded later.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name             | Required | Input Value | Description |
| -------------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags UpgradeableBeaconChainDepositPredeploy
```

Base command with cli arguments:

```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> npx hardhat deploy --network linea_sepolia --tags UpgradeableBeaconChainDepositPredeploy
```

(make sure to replace `<key>` with actual values)
