# ForcedTransactionGateway

[← Back to index](../README.md)

<br />

Deploys the ForcedTransactionGateway contract on L1. This contract allows users to submit forced transactions that must be included by the sequencer. It requires a pre-deployed Mimc library and AddressFilter contract.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| LINEA_ROLLUP_ADDRESS | true | address | LineaRollup contract address |
| L1_SECURITY_COUNCIL | true | address | L1 Security Council address (default admin) |
| FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID | true | uint256 | Destination L2 chain ID |
| FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER | true | uint256 | L2 block buffer for forced transaction inclusion |
| FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT | true | uint256 | Maximum gas limit for forced transactions |
| FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER | true | uint256 | Maximum input length buffer |
| FORCED_TRANSACTION_ADDRESS_FILTER | true | address | AddressFilter contract address |
| FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS | true | uint256 | L2 block duration in seconds |
| FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER | true | uint256 | Block number deadline buffer |
| MIMC_LIBRARY_ADDRESS | true | address | Pre-deployed Mimc library address |

<br />

**Prerequisites:**
- Mimc library must be deployed (address provided via `MIMC_LIBRARY_ADDRESS`)
- AddressFilter contract must be deployed (address provided via `FORCED_TRANSACTION_ADDRESS_FILTER`)

Base command:
```shell
npx hardhat deploy --network sepolia --tags ForcedTransactionGateway
```

Base command with cli arguments:
```shell
DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> LINEA_ROLLUP_ADDRESS=<address> L1_SECURITY_COUNCIL=<address> FORCED_TRANSACTION_GATEWAY_L2_CHAIN_ID=<value> FORCED_TRANSACTION_GATEWAY_L2_BLOCK_BUFFER=<value> FORCED_TRANSACTION_GATEWAY_MAX_GAS_LIMIT=<value> FORCED_TRANSACTION_GATEWAY_MAX_INPUT_LENGTH_BUFFER=<value> FORCED_TRANSACTION_ADDRESS_FILTER=<address> FORCED_TRANSACTION_L2_BLOCK_DURATION_SECONDS=<value> FORCED_TRANSACTION_BLOCK_NUMBER_DEADLINE_BUFFER=<value> MIMC_LIBRARY_ADDRESS=<address> npx hardhat deploy --network sepolia --tags ForcedTransactionGateway
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)
