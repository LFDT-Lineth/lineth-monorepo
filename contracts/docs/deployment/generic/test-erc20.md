# TestERC20

[← Back to index](../README.md)

<br />

Deploys a test ERC20 token (for testing purposes only).

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| TEST_ERC20_NAME | true | string | Token name |
| TEST_ERC20_SYMBOL | true | string | Token symbol |
| TEST_ERC20_INITIAL_SUPPLY | true | uint256 | Initial token supply |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags TestERC20
```
