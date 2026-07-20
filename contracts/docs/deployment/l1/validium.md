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
| VERIFIER_ADDRESS | registry\|env | address | PlonkVerifier contract address. Read from registry on stable networks; env var used as fallback. |
| INITIAL_L2_BLOCK_HASH   | true      | bytes32 | Initial L2 block hash at genesis (shared base); passed as `initialBlockHash` on-chain. |
| INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number (shared base) |
| L2_GENESIS_TIMESTAMP | true | uint256 | Genesis timestamp (shared base) |
| L1_SECURITY_COUNCIL  | registry\|env | address | L1 Security Council Address. Read from registry on stable networks; env var used as fallback. |
| VALIDIUM_OPERATORS     | true      | address | Validium Operators Addresses (comma-delimited if multiple) |
| VALIDIUM_RATE_LIMIT_PERIOD     | true  | uint256   | Validium Rate Limit Period |
| VALIDIUM_RATE_LIMIT_AMOUNT     | true  | uint256   | Validium Rate Limit Amount |
| LINEA_ROLLUP_ADDRESS_FILTER | registry\|env | address | AddressFilter contract address. Read from registry on stable networks; env var used as fallback. |
| VALIDIUM_VERIFIER_KEYS | false | bytes32 list | Comma-delimited guest-program verifier keys to seed at initialization (defaults to empty). `SET_VERIFIER_KEY_ROLE` / `UNSET_VERIFIER_KEY_ROLE` are granted to the security council via default role assignments. |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network sepolia --tags Validium
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> INITIAL_L2_BLOCK_HASH=<bytes> INITIAL_L2_BLOCK_NUMBER=<value> L2_GENESIS_TIMESTAMP=<value> L1_SECURITY_COUNCIL=<address> VALIDIUM_OPERATORS=<address> VALIDIUM_RATE_LIMIT_PERIOD=<value> VALIDIUM_RATE_LIMIT_AMOUNT=<value> pnpm exec hardhat deploy --network sepolia --tags Validium
```

(make sure to replace `<value>` `<key>` `<bytes>` `<address>` with actual values).
