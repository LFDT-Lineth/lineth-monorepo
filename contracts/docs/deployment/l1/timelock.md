# Timelock

[← Back to index](../README.md)

<br />

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags Timelock
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> npx hardhat deploy --network sepolia --tags Timelock
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)
