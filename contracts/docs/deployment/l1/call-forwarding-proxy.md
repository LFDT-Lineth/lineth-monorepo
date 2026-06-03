# CallForwardingProxy

[← Back to index](../README.md)

<br />

Deploys a CallForwardingProxy that forwards calls to the LineaRollup contract.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| LINEA_ROLLUP_ADDRESS | registry\|env | address | LineaRollup contract address to forward calls to. Read from registry on stable networks; env var used as fallback. |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network sepolia --tags CallForwardingProxy
```
