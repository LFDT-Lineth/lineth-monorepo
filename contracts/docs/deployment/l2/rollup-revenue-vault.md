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
| L2_SECURITY_COUNCIL  | registry\|env | address | L2 Security Council Address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE | true | uint256 | Last invoice date timestamp |
| ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER | registry\|env | address | Invoice submitter address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_BURNER | registry\|env | address | Burner address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER | registry\|env | address | Invoice payment receiver address. Read from registry if present; env var used as fallback. |
| TOKEN_BRIDGE_ADDRESS | registry\|env | address | Token bridge (L2) address. Looks up `TokenBridge_L2` from registry on stable networks; env var used as fallback. |
| L2_MESSAGE_SERVICE_ADDRESS | registry\|env | address | L2 Message Service address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER | registry\|env | address | L1 Linea token burner address. Read from registry if present; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_LINEA_TOKEN | registry\|env | address | Linea token address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER | registry\|env | address | DEX swap adapter address. Read from registry if present; env var used as fallback. |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network linea_sepolia --tags RollupRevenueVault
```

(make sure to replace `<key>` `<address>` `<value>` with actual values)

<br />

## Upgrade Deployments

### RollupRevenueVaultWithReinitialization

Deploys a new RollupRevenueVault implementation and generates encoded upgrade calldata with `initializeRolesAndStorageVariables`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| ROLLUP_REVENUE_VAULT_ADDRESS | registry\|env | address | Existing RollupRevenueVault proxy address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_LAST_INVOICE_DATE | true | uint256 | Last invoice date timestamp |
| L2_SECURITY_COUNCIL | registry\|env | address | L2 Security Council Address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_INVOICE_SUBMITTER | registry\|env | address | Invoice submitter address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_BURNER | registry\|env | address | Burner address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_INVOICE_PAYMENT_RECEIVER | registry\|env | address | Invoice payment receiver address. Read from registry if present; env var used as fallback. |
| TOKEN_BRIDGE_ADDRESS | registry\|env | address | Token bridge (L2) address. Looks up `TokenBridge_L2` from registry on stable networks; env var used as fallback. |
| L2_MESSAGE_SERVICE_ADDRESS | registry\|env | address | L2 Message Service address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_L1_LINEA_TOKEN_BURNER | registry\|env | address | L1 Linea token burner address. Read from registry if present; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_LINEA_TOKEN | registry\|env | address | Linea token address. Read from registry on stable networks; env var used as fallback. |
| ROLLUP_REVENUE_VAULT_DEX_SWAP_ADAPTER | registry\|env | address | DEX swap adapter address. Read from registry if present; env var used as fallback. |

```shell
pnpm exec hardhat deploy --network linea_sepolia --tags RollupRevenueVaultWithReinitialization
```
