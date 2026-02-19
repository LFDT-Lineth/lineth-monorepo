# PaymentSplitterWrapper

[← Back to index](../README.md)

<br />

Deploys a PaymentSplitterWrapper contract for splitting payments among multiple payees.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| PAYMENT_SPLITTER_PAYEES | true | address | Comma-separated list of payee addresses |
| PAYMENT_SPLITTER_SHARES | true | uint256 | Comma-separated list of shares per payee |

<br />

Base command:
```shell
npx hardhat deploy --network sepolia --tags PaymentSplitterWrapper
```
