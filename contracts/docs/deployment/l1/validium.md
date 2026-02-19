# Validium

[← Back to index](../README.md)

<br />

The Validium contract is a permutation of LineaRollup that uses off-chain data availability. It shares the same base contract (same initial state root, block number, and genesis timestamp) but has its own operators and rate limits.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network.|
| PLONKVERIFIER_ADDRESS | true | address | PlonkVerifier contract address |
| INITIAL_L2_STATE_ROOT_HASH   | true      | bytes | Initial State Root Hash (shared base) |
| INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number (shared base) |
| L2_GENESIS_TIMESTAMP | true | uint256 | Genesis timestamp (shared base) |
| L1_SECURITY_COUNCIL  | true      | address | L1 Security Council Address |
| VALIDIUM_OPERATORS     | true      | address | Validium Operators Addresses (comma-delimited if multiple) |
| VALIDIUM_RATE_LIMIT_PERIOD     | true  | uint256   | Validium Rate Limit Period |
| VALIDIUM_RATE_LIMIT_AMOUNT     | true  | uint256   | Validium Rate Limit Amount |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags Validium
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> INITIAL_L2_STATE_ROOT_HASH=<bytes> INITIAL_L2_BLOCK_NUMBER=<value> L2_GENESIS_TIMESTAMP=<value> L1_SECURITY_COUNCIL=<address> VALIDIUM_OPERATORS=<address> VALIDIUM_RATE_LIMIT_PERIOD=<value> VALIDIUM_RATE_LIMIT_AMOUNT=<value> npx hardhat deploy --network sepolia --tags Validium
```

(make sure to replace `<value>` `<key>` `<bytes>` `<address>` with actual values).
