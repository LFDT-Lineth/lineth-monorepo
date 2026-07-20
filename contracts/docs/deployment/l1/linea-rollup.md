# LineaRollup

[← Back to index](../README.md)

<br />

## LineaRollup (Fresh Deploy)

ABI string: `CONTRACT_VERSION()` returns `"9.0"`. Fresh `initialize` uses OpenZeppelin `reinitializer(10)`.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input value | Description |
| --------------------- | -------- | -------------- | ----------- |
| VERIFY_CONTRACT    | false    | true\|false | Verifies the deployed contract |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network.|
| INITIAL_L2_BLOCK_HASH   | true      | bytes32 | Initial L2 block hash at genesis (shared base); passed as `initialBlockHash` on-chain. |
| INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number (shared base) |
| L2_GENESIS_TIMESTAMP | true | uint256 | Genesis timestamp (shared base) |
| L1_SECURITY_COUNCIL  | registry\|env | address | L1 Security Council Address. Read from registry on stable networks; env var used as fallback. |
| LINEA_ROLLUP_OPERATORS     | registry\|env | address | L1 Operators Addresses (comma-delimited if multiple). Read from registry on stable networks; env var used as fallback. |
| LINEA_ROLLUP_RATE_LIMIT_PERIOD     | true  | uint256   | L1 Rate Limit Period |
| LINEA_ROLLUP_RATE_LIMIT_AMOUNT     | true  | uint256   | L1 Rate Limit Amount |
| VERIFIER_ADDRESS | registry\|env | address | PlonkVerifier contract address. Read from registry on stable networks; env var used as fallback (set automatically when deploying Verifier in same chain). |
| YIELD_MANAGER_ADDRESS | registry\|env | address | Yield Manager contract address. Read from registry on stable networks; env var used as fallback. |
| LINEA_ROLLUP_ADDRESS_FILTER | registry\|env | address | AddressFilter contract address. Read from registry on stable networks; env var used as fallback. |
| LINEA_ROLLUP_VERIFIER_KEYS | false | bytes32 list | Comma-delimited guest-program verifier keys to seed at initialization (defaults to empty). `SET_VERIFIER_KEY_ROLE` / `UNSET_VERIFIER_KEY_ROLE` are granted to the security council via default role assignments. |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network sepolia --tags LineaRollup
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> INITIAL_L2_BLOCK_HASH=<bytes> INITIAL_L2_BLOCK_NUMBER=<value> L2_GENESIS_TIMESTAMP=<value> L1_SECURITY_COUNCIL=<address> LINEA_ROLLUP_OPERATORS=<address> LINEA_ROLLUP_RATE_LIMIT_PERIOD=<value> LINEA_ROLLUP_RATE_LIMIT_AMOUNT=<value> YIELD_MANAGER_ADDRESS=<address> pnpm exec hardhat deploy --network sepolia --tags LineaRollup
```

(make sure to replace `<value>` `<key>` `<bytes>` `<address>` with actual values).

<br />

## Upgrade Deployments

Upgrade order for live proxies moving to ABI `"9.0"`:

1. **Forced-tx cutover** — `LineaRollupV8WithReinitialization` → `reinitializeLineaRollupV9` (`reinitializer(9)`, ABI `"7.1"`→`"8.0"`).
2. **Blockhash / RISC-V cutover** — `LineaRollupV9WithReinitialization` → `reinitializeLineaRollupV10` (`reinitializer(10)`, ABI `"8.0"`→`"9.0"`).

OZ `reinitializer(9)` is consumed by the forced-tx step; the blockhash ABI bump requires a new OZ slot (`10`).

### LineaRollupWithReinitialization

Deploys a new LineaRollup implementation and generates encoded upgrade calldata with `reinitializeV8`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| L1_SECURITY_COUNCIL | registry\|env | address | Security Council address. Read from registry on stable networks; env var used as fallback. |
| LINEA_ROLLUP_ADDRESS | registry\|env | address | Existing LineaRollup proxy address. Read from registry on stable networks; env var used as fallback. |

```shell
pnpm exec hardhat deploy --network sepolia --tags LineaRollupWithReinitialization
```

<br />

### LineaRollupV8WithReinitialization

Deploys a new LineaRollup implementation and generates encoded `upgradeAndCall` calldata for `reinitializeLineaRollupV9` (forced transactions). Submit the printed calldata through the Security Council Safe targeting the ProxyAdmin.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| LINEA_ROLLUP_ADDRESS | registry\|env | address | Existing LineaRollup proxy address. Read from registry on stable networks; env var used as fallback. |
| LINEA_ROLLUP_FORCED_TRANSACTION_FEE_IN_WEI | true | uint256 | Forced transaction fee in wei (must be > 0) |
| LINEA_ROLLUP_ADDRESS_FILTER | registry\|env | address | AddressFilter contract address. Read from registry if present; env var used as fallback. |

```shell
pnpm exec hardhat deploy --network sepolia --tags LineaRollupV8WithReinitialization
```

<br />

### LineaRollupV9WithReinitialization

Deploys a new LineaRollup implementation and generates encoded `upgradeAndCall` calldata for `reinitializeLineaRollupV10` (blockhash-centric / RISC-V ABI cutover). Submit the printed calldata through the Security Council Safe targeting the ProxyAdmin.

`reinitializeLineaRollupV10` only bumps the ABI version and emits `LineaRollupVersionChanged("8.0", "9.0")`. Does **not** populate `blockHashes[currentL2BlockNumber]` — the first post-upgrade finalization takes the one-way migration path from `stateRootHashes`. Verifier keys and `SET_VERIFIER_KEY_ROLE` / `UNSET_VERIFIER_KEY_ROLE` are configured separately after upgrade via `grantRole` and `setVerifierKeys`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| LINEA_ROLLUP_ADDRESS | registry\|env | address | Existing LineaRollup proxy address. Read from registry on stable networks; env var used as fallback. |

```shell
pnpm exec hardhat deploy --network sepolia --tags LineaRollupV9WithReinitialization
```
