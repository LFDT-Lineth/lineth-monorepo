# ImplementationForProxy

[← Back to index](../README.md)

<br />

Deploys a new implementation contract for an existing upgradeable proxy. Outputs encoded calldata for upgrading the proxy via the Security Council Safe.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| CONTRACT_NAME | true | string | Name of the contract to deploy as new implementation |
| PROXY_ADDRESS | true | address | Address of the existing proxy contract |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags ImplementationForProxy
```

Base command with cli arguments:
```shell
DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> CONTRACT_NAME=<string> PROXY_ADDRESS=<address> npx hardhat deploy --network sepolia --tags ImplementationForProxy
```

(make sure to replace `<key>` `<address>` `<string>` with actual values)
