# LinethRollup

[← Back to index](../README.md)

<br />

## LinethRollup (Fresh Deploy)

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
| LINETH_ROLLUP_OPERATORS     | registry\|env | address | L1 Operators Addresses (comma-delimited if multiple). Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_RATE_LIMIT_PERIOD     | true  | uint256   | L1 Rate Limit Period |
| LINETH_ROLLUP_RATE_LIMIT_AMOUNT     | true  | uint256   | L1 Rate Limit Amount |
| VERIFIER_ADDRESS | registry\|env | address | PlonkVerifier contract address. Read from registry on stable networks; env var used as fallback (set automatically when deploying Verifier in same chain). |
| YIELD_MANAGER_ADDRESS | registry\|env | address | Yield Manager contract address. Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_ADDRESS_FILTER | registry\|env | address | AddressFilter contract address. Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_VERIFIER_KEYS | false | bytes32 list | Comma-delimited guest-program verifier keys to seed at initialization (defaults to empty). `SET_VERIFIER_KEY_ROLE` / `UNSET_VERIFIER_KEY_ROLE` are granted to the security council via default role assignments. |

<br />

Base command:
```shell
pnpm exec hardhat deploy --network sepolia --tags LinethRollup
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> INITIAL_L2_BLOCK_HASH=<bytes> INITIAL_L2_BLOCK_NUMBER=<value> L2_GENESIS_TIMESTAMP=<value> L1_SECURITY_COUNCIL=<address> LINETH_ROLLUP_OPERATORS=<address> LINETH_ROLLUP_RATE_LIMIT_PERIOD=<value> LINETH_ROLLUP_RATE_LIMIT_AMOUNT=<value> YIELD_MANAGER_ADDRESS=<address> pnpm exec hardhat deploy --network sepolia --tags LinethRollup
```

(make sure to replace `<value>` `<key>` `<bytes>` `<address>` with actual values).

<br />

## Upgrade Deployments

Live proxies moving to ABI `"9.0"` perform a single cutover:

- **Blob-spanning cutover** — `LinethRollupV10WithReinitialization` → `reinitializeLineaRollupV10(bytes32)` (`reinitializer(10)`, ABI `"9.0"`→`"10.0"`).

The bridge reinterprets the finalized 3-arg shnarf slot as the previous end dataRollingHash, anchors it, and reseals the slot as a fresh-start position commitment `keccak256(shnarf ‖ 0)`. The caller MUST supply the exact current `currentFinalizedShnarf` value; the bridge reverts with `BridgedShnarfMismatch` if live state has drifted from what governance approved.

### LinethRollupWithReinitialization

Deploys a new LinethRollup implementation and generates encoded upgrade calldata with `reinitializeV8`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| L1_SECURITY_COUNCIL | registry\|env | address | Security Council address. Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_ADDRESS | registry\|env | address | Existing LinethRollup proxy address. Read from registry on stable networks; env var used as fallback. |

```shell
pnpm exec hardhat deploy --network sepolia --tags LinethRollupWithReinitialization
```

<br />

### LinethRollupV10WithReinitialization

Deploys a new LinethRollup implementation and generates encoded `upgradeAndCall` calldata for `reinitializeLineaRollupV10(bytes32)` (blob-spanning dataRollingHash cutover). Submit the printed calldata through the Security Council Safe targeting the ProxyAdmin.

`reinitializeLineaRollupV10` anchors the bridged finalized shnarf as a dataRollingHash, reseals the position-commitment slot, and emits `LineaRollupVersionChanged("9.0", "10.0")`. Verifier keys and `SET_VERIFIER_KEY_ROLE` / `UNSET_VERIFIER_KEY_ROLE` are configured separately after upgrade via `grantRole` and `setVerifierKeys`.

| Parameter name | Required | Input value | Description |
|---|---|---|---|
| \**DEPLOYER_PRIVATE_KEY* | true | key | Network-specific private key |
| LINETH_ROLLUP_ADDRESS | registry\|env | address | Existing LinethRollup proxy address. Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_CURRENT_FINALIZED_SHNARF | true | bytes32 | The exact on-chain `currentFinalizedShnarf` value at upgrade time; the bridge reverts with `BridgedShnarfMismatch` on drift. |

```shell
pnpm exec hardhat deploy --network sepolia --tags LinethRollupV10WithReinitialization
```
