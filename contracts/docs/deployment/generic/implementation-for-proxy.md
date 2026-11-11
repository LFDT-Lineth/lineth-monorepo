# ImplementationForProxy (upgrade modules)

[← Back to index](../README.md)

<br />

Deploys a new implementation contract for an existing upgradeable proxy and prints the encoded calldata for upgrading the proxy via the Security Council Safe (`ProxyAdmin.upgrade`, or `ProxyAdmin.upgradeAndCall` for the reinitializer modules). Before deploying, the new implementation is checked with OpenZeppelin Upgrades against the proxy (`validateUpgrade`) when the proxy's current implementation is in `.openzeppelin/<network>.json`, or on its own otherwise. Afterwards the new implementation is registered there.

One Ignition module per contract, in `ignition/modules/upgrades/` (this replaces `CONTRACT_NAME`):

| Module | Contract | Proxy address (registry key, else env var) |
| ------ | -------- | ------------------------------------------ |
| `LinethRollupUpgrade` | LinethRollup | `LinethRollup`, else `LINETH_ROLLUP_ADDRESS` |
| `ValidiumUpgrade` | Validium | `Validium`, else `PROXY_ADDRESS` |
| `L2MessageServiceUpgrade` | L2MessageService | `L2MessageService`, else `L2_MESSAGE_SERVICE_ADDRESS` |
| `TokenBridgeUpgrade` | TokenBridge | `TokenBridge_L1` / `TokenBridge_L2` (by `DEPLOY_TOKEN_BRIDGE_ON_L1`), else `TOKEN_BRIDGE_ADDRESS` |
| `YieldManagerUpgrade` | YieldManager (constructor `linethRollup`: `LinethRollup`, else `LINETH_ROLLUP_ADDRESS`) | `YieldManager`, else `YIELD_MANAGER_ADDRESS` |
| `RollupRevenueVaultUpgrade` | RollupRevenueVault | `RollupRevenueVault`, else `ROLLUP_REVENUE_VAULT_ADDRESS` |

Reinitializer variants: `LinethRollupReinitializeV9`, `L2MessageServiceReinitializeV3`, `TokenBridgeReinitializeV3`, `RollupRevenueVaultReinitialize` (see the per-contract pages).

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract (or `--signer-ui`) |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. |
| proxy env var (table above) | on networks without a registry entry | address | Address of the existing proxy contract |

<br />

Base command:
```shell
pnpm exec hardhat ignition deploy LinethRollupUpgrade --network sepolia
```

Base command with cli arguments:
```shell
DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> LINETH_ROLLUP_ADDRESS=<address> pnpm exec hardhat ignition deploy LinethRollupUpgrade --network sepolia
```

(make sure to replace `<key>` `<address>` with actual values)

Re-running with unchanged bytecode resumes the same deployment (deployment id `<Module>-<chainId>-<bytecode hash>`); new bytecode deploys a new implementation.
