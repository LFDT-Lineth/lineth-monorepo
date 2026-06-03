# Contract Address Registry

This directory contains the PR-reviewed, per-network deployed address registry for the Linea protocol. Files may be edited manually or generated from an external source of truth, then validated before use.

## Files

| File | Network | Chain ID |
|------|---------|----------|
| `mainnet.json` | Ethereum mainnet | 1 |
| `sepolia.json` | Ethereum Sepolia testnet | 11155111 |
| `hoodi.json` | Ethereum Hoodi testnet | 560048 |
| `linea_mainnet.json` | Linea mainnet (L2) | 59144 |
| `linea_sepolia.json` | Linea Sepolia (L2 testnet) | 59141 |
| `schema.json` | JSON Schema for all registry files | — |

## How Addresses Are Used

Deploy scripts call one of three registry helpers:

- `requireAddressFromRegistryOrEnv(networkName, contractKey, envVarName)` — single required address; hard fails if neither source provides one.
- `requireAddressesFromRegistryOrEnv(networkName, contractKey, envVarName)` — comma-delimited required list; hard fails if neither source provides one.
- `getAddressesFromRegistryOrEnv(networkName, contractKey, envVarName)` — comma-delimited **optional** list; returns `[]` if neither source provides one. Used for reserved-token address lists.

Lookup tries `contractKey` first, then `envVarName` when they differ (so exports keyed by env var name also work). The resolution order is:

| Registry entry | Env var set | Outcome |
|---|---|---|
| Present (non-zero) | Not set | Registry address used |
| Present (non-zero) | **Matches** registry | Registry address used |
| Present (non-zero) | **Conflicts** with registry | **Hard fail before any transaction** |
| Absent or zero address | Set | Env var used (format-validated) |
| Absent or zero address | Not set | Hard fail (required helpers) or `[]` / `undefined` (optional helpers) |

Networks without a registry file (`custom`, `zkevm_dev`, `l2`) skip the registry entirely
and fall back to requiring the env var (or returning the default for optional helpers).

## How to Update an Address

1. Edit the appropriate network JSON file with the new EIP-55 checksummed address.
2. Open a pull request — the registry change is the auditable record that the address changed.
3. Get a review from a team member familiar with the deployment.
4. Remove any corresponding env var from `.env` files to avoid a conflict error on the next deploy.

## Address Entry Format

Each contract key may use either a single address or an address list.

Single address:

```json
{
  "ContractKey": {
    "address": "0x...",
    "notes": "Optional human-readable notes"
  }
}
```

Multiple addresses (for env vars such as `LINEA_ROLLUP_OPERATORS`):

```json
{
  "LINEA_ROLLUP_OPERATORS": {
    "addresses": [
      { "address": "0x...", "notes": "L1 Finalization Operator EOA" },
      { "address": "0x...", "notes": "L1 Data Submission Operator EOA" }
    ]
  }
}
```

- `address` / each `addresses[].address` must be a valid EIP-55 checksummed Ethereum address.
- An entry must define either `address` or `addresses`, not both.
- Zero address (`0x0000...0000`) is treated as a placeholder meaning "not yet populated".
  Registry entries initialised with the zero address are ignored and env vars are used instead.
  For `addresses` arrays, either every item is zero (placeholder) or every item must be non-zero.
- `notes` is free-form text for context (proxy type, multisig info, etc.).

## Validation

After editing registry files (or exporting from an external source of truth), run:

```shell
pnpm -F contracts run validate:address-registry
```

This validates JSON shape, network/chainId metadata, EIP-55 checksums, duplicate list entries, and
zero/non-zero consistency before deploy scripts consume the data. It also scans the raw JSON for
duplicate object keys, because `JSON.parse` would otherwise silently keep only the last value.

## Registry Key Mapping

Deploy scripts look up `contractKey` first, then `envVarName` when they differ. The following keys are recognised by deploy scripts:

| Key | Env var equivalent | Notes |
|-----|--------------------|-------|
| `LineaRollup` | `LINEA_ROLLUP_ADDRESS` | Transparent upgradeable proxy (L1) |
| `Validium` | — | Transparent upgradeable proxy (L1) |
| `L2MessageService` | `L2_MESSAGE_SERVICE_ADDRESS` | Transparent upgradeable proxy (L2) |
| `TokenBridge_L1` | `TOKEN_BRIDGE_ADDRESS` (when `DEPLOY_TOKEN_BRIDGE_ON_L1=true`) | L1 token bridge proxy |
| `TokenBridge_L2` | `TOKEN_BRIDGE_ADDRESS` (when `DEPLOY_TOKEN_BRIDGE_ON_L1` unset) | L2 token bridge proxy |
| `CallForwardingProxy` | — | CallForwardingProxy (L1) |
| `YieldManager` | `YIELD_MANAGER_ADDRESS` | YieldManager proxy (L1) |
| `AddressFilter` | `LINEA_ROLLUP_ADDRESS_FILTER` | AddressFilter (L1) |
| `RollupRevenueVault` | `ROLLUP_REVENUE_VAULT_ADDRESS` | RollupRevenueVault proxy (L2) |
| `L1_SECURITY_COUNCIL` | `L1_SECURITY_COUNCIL` | L1 security council multisig |
| `L2_SECURITY_COUNCIL` | `L2_SECURITY_COUNCIL` | L2 security council multisig |
| `NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS` | `NATIVE_YIELD_AUTOMATION_SERVICE_ADDRESS` | Yield automation service operator (L1) |
| `VAULT_HUB` | `VAULT_HUB` | Lido VaultHub proxy (L1) |
| `VAULT_FACTORY` | `VAULT_FACTORY` | Lido Staking Vault Factory (L1) |
| `STETH` | `STETH` | Lido stETH token proxy (L1) |
| `LINEA_TOKEN` | `LINEA_TOKEN` | Linea token for the current chain registry |
| `LINEA_ROLLUP_OPERATORS` | `LINEA_ROLLUP_OPERATORS` | Comma-delimited L1 operator EOAs |
| `VALIDIUM_OPERATORS` | `VALIDIUM_OPERATORS` | Comma-delimited Validium operator EOAs |
| `L1_RESERVED_TOKEN_ADDRESSES` | `L1_RESERVED_TOKEN_ADDRESSES` | Optional comma-delimited L1 reserved token addresses (defaults to `[]` if absent) |
| `L2_RESERVED_TOKEN_ADDRESSES` | `L2_RESERVED_TOKEN_ADDRESSES` | Optional comma-delimited L2 reserved token addresses (defaults to `[]` if absent) |

> **Note:** `VERIFIER_ADDRESS` rotates with each proof system upgrade.
