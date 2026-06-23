# L1LineaTokenBurner

[← Back to index](../README.md)

<br />

Deploys the L1 Linea Token Burner contract.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| LINEA_ROLLUP_ADDRESS | registry\|env | address | LineaRollup address (L1 message service). Read from registry on stable networks; env var used as fallback. |
| LINEA_TOKEN | true | address | L1 Linea token address |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network sepolia --tags L1LineaTokenBurner
```
