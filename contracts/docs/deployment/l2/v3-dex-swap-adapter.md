# V3DexSwapAdapter

[← Back to index](../README.md)

<br />

Deploys the V3 DEX Swap Adapter contract for WETH deposits.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| V3_DEX_SWAP_ADAPTER_ROUTER | true | address | Uniswap V3 router address |
| V3_DEX_SWAP_ADAPTER_WETH_TOKEN | true | address | WETH token address |
| V3_DEX_SWAP_ADAPTER_LINEA_TOKEN | true | address | Linea token address |
| V3_DEX_SWAP_ADAPTER_POOL_TICK_SPACING | true | int24 | Pool tick spacing |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags V3DexSwapAdapter
```
