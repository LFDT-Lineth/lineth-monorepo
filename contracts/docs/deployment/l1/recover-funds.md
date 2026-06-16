# RecoverFunds

[← Back to index](../README.md)

<br />

Deploys the RecoverFunds contract as an upgradeable proxy.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network.|
| L1_SECURITY_COUNCIL  | registry\|env | address | L1 Security Council Address. Read from registry on stable networks; env var used as fallback. |
| RECOVERFUNDS_EXECUTOR_ADDRESS | registry\|env | address | Executor address for the RecoverFunds contract. Read from registry if present; env var used as fallback. |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network sepolia --tags RecoverFunds
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> L1_SECURITY_COUNCIL=<address> RECOVERFUNDS_EXECUTOR_ADDRESS=<address> pnpm exec hardhat deploy --network sepolia --tags RecoverFunds
```

(make sure to replace `<key>` `<address>` with actual values)
