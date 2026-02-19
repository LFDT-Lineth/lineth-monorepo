# Chained Deployments

[← Back to index](README.md)

<br />

This section describes the scripts that can be run to deploy multiple contracts in a sequence.

<br />

## L1MessageService Chained Deployments

This will run the script that deploys PlonkVerifier, LineaRollup, Timelock contracts.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| INITIAL_L2_STATE_ROOT_HASH   | true      | bytes | Initial State Root Hash (shared base) |
| INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number (shared base) |
| L2_GENESIS_TIMESTAMP | true | uint256 | Genesis timestamp (shared base) |
| L1_SECURITY_COUNCIL  | true      | address | Security Council Address |
| LINEA_ROLLUP_OPERATORS     | true      | address | Operators Addresses (comma-delimited if multiple) |
| LINEA_ROLLUP_RATE_LIMIT_PERIOD     | true  | uint256   | L1 Rate Limit Period |
| LINEA_ROLLUP_RATE_LIMIT_AMOUNT     | true  | uint256   | L1 Rate Limit Amount |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |
| VERIFIER_CONTRACT_NAME | true | string | PlonkVerifier contract name that should be deployed |
| VERIFIER_PROOF_TYPE | true | string | The proof type that the verifier should be mapped to |
| VERIFIER_CHAIN_ID | true | uint256 | Chain ID passed to the verifier constructor |
| VERIFIER_BASE_FEE | true | uint256 | Base fee passed to the verifier constructor |
| VERIFIER_COINBASE | true | address | Coinbase address passed to the verifier constructor |
| L2_MESSAGE_SERVICE_ADDRESS | true | address | L2 Message Service address passed to the verifier constructor |
| YIELD_MANAGER_ADDRESS | true | address | Yield Manager contract address |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags PlonkVerifier,LineaRollup,Timelock
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> INITIAL_L2_STATE_ROOT_HASH=<bytes> INITIAL_L2_BLOCK_NUMBER=<value> L2_GENESIS_TIMESTAMP=<value> L1_SECURITY_COUNCIL=<address> LINEA_ROLLUP_OPERATORS=<address> LINEA_ROLLUP_RATE_LIMIT_PERIOD=<value> LINEA_ROLLUP_RATE_LIMIT_AMOUNT=<value> YIELD_MANAGER_ADDRESS=<address> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> VERIFIER_CONTRACT_NAME=PlonkVerifierForMultiTypeDataAggregation npx hardhat deploy --network sepolia --tags PlonkVerifier,LineaRollup,Timelock
```

(make sure to replace `<value>` `<bytes>` `<key>` `<address>` with actual values)

<br />

## L2MessageService Chained Deployments

This will run the script that deploys Timelock, L2MessageService contracts.

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_SECURITY_COUNCIL | true   | address | L2 Security council address |
| L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER  | true  |  address | L1L2 Message Setter address on L2 |
| L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD    | true  |  uint256 | L2 Rate Limit Period |
| L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT    | true  |  uint256 | L2 Rate Limit Amount |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags L2MessageService,Timelock
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> L2_SECURITY_COUNCIL=<address> L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER=<address> L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD=<value> L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT=<value> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> npx hardhat deploy --network linea_sepolia --tags L2MessageService,Timelock
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />

## TokenBridge & BridgedToken Chained Deployments

This will run the script that deploys the TokenBridge and BridgedToken contracts.

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contract. |
| \**DEPLOYER_PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_MESSAGE_SERVICE_ADDRESS    | true  | address   | L2 Message Service address used when deploying TokenBridge.    |
| LINEA_ROLLUP_ADDRESS         | true    | address       | L1 Rollup address used when deploying Token Bridge.   |
| REMOTE_CHAIN_ID       | true      |   uint256     | ChainID of the remote (target) network |
| REMOTE_SENDER_ADDRESS | true | address | Remote sender address (the TokenBridge on the other chain) |
| TOKEN_BRIDGE_L1       | false     |true\|false| If Token Bridge is deployed on L1, TOKEN_BRIDGE_L1 should be set to `true`. Otherwise it should be `false`|
| L1_SECURITY_COUNCIL | conditional | address | L1 Security Council address. Required when `TOKEN_BRIDGE_L1=true` |
| L2_SECURITY_COUNCIL | conditional | address | L2 Security Council address. Required when `TOKEN_BRIDGE_L1=false` |
| L1_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=true, then L1_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|
| L2_RESERVED_TOKEN_ADDRESSES | false   | address   | If TOKEN_BRIDGE_L1=false, then L2_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|


Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags BridgedToken,TokenBridge
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true ETHERSCAN_API_KEY=<key> DEPLOYER_PRIVATE_KEY=<key> INFURA_API_KEY=<key> REMOTE_CHAIN_ID=<uint256> TOKEN_BRIDGE_L1=true L1_SECURITY_COUNCIL=<address> L1_RESERVED_TOKEN_ADDRESSES=<address> L2_MESSAGE_SERVICE_ADDRESS=<address> LINEA_ROLLUP_ADDRESS=<address> REMOTE_SENDER_ADDRESS=<address> npx hardhat deploy --network linea_sepolia --tags BridgedToken,TokenBridge
```
(make sure to replace `<value>` `<key>` `<address>` with actual values)
