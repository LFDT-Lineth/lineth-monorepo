# RollupRevenueVault

[← Back to index](../README.md)

<br />

## RollupRevenueVault (Fresh Deploy)

Deploys the RollupRevenueVault contract as an upgradeable proxy.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| L2_SECURITY_COUNCIL  | true      | address | L2 Security Council Address |
| ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE | true | uint256 | Last invoice date timestamp |
| ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER | true | address | Invoice submitter address |
| ROLLUP_REVENUE_VAULT_BURNER | true | address | Burner address |
| ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER | true | address | Invoice payment receiver address |
| ROLLUP_REVENUE_VAULT_TOKEN_BRIDGE | true | address | Token bridge address |
| L2_MESSAGE_SERVICE_ADDRESS | true | address | L2 Message Service address |
| ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER | true | address | L1 Linea token burner address |
| ROLLUP_REVENUE_VAULT_LINEA_TOKEN | true | address | Linea token address |
| ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER | true | address | DEX swap adapter address |

<br />

Base command:
```shell
npx hardhat deploy --network linea_sepolia --tags RollupRevenueVault
```

(make sure to replace `<key>` `<address>` `<value>` with actual values)

<br />

## Upgrade Deployments

### RollupRevenueVaultWithReinitialization

Deploys a new RollupRevenueVault implementation and generates encoded upgrade calldata with `initializeRolesAndStorageVariables`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| ROLLUP_REVENUE_VAULT_ADDRESS | true | address | Existing RollupRevenueVault proxy address |
| ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE | true | uint256 | Last invoice date timestamp |
| L2_SECURITY_COUNCIL | true | address | L2 Security Council Address |
| ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER | true | address | Invoice submitter address |
| ROLLUP_REVENUE_VAULT_BURNER | true | address | Burner address |
| ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER | true | address | Invoice payment receiver address |
| ROLLUP_REVENUE_VAULT_TOKEN_BRIDGE | true | address | Token bridge address |
| L2_MESSAGE_SERVICE_ADDRESS | true | address | L2 Message Service address |
| ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER | true | address | L1 Linea token burner address |
| ROLLUP_REVENUE_VAULT_LINEA_TOKEN | true | address | Linea token address |
| ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER | true | address | DEX swap adapter address |

```shell
npx hardhat deploy --network linea_sepolia --tags RollupRevenueVaultWithReinitialization
```
