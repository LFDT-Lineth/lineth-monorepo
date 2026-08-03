# SDK Migration Guide: `LineaRollup` → `LinethRollup`

This renames every SDK type, class, interface, constant, and parameter that refers to the **`LineaRollup` smart contract** to **`LinethRollup`**, matching the on-chain contract rename (`contracts/src/rollup/LinethRollupBase.sol` etc.). It is a **breaking change** for `@lfdt-lineth/sdk` (sdk-ethers, unpublished/internal-only). For the published `@lfdt-lineth/sdk-viem` package, the old `lineaRollupAddress` parameter still works — see below.

> **Not renamed:** the "Linea" **network/chain** name itself. Anything referring to the Linea network (e.g. `linea` / `lineaSepolia` chain imports from `viem/chains`, "Linea Mainnet", "message on Linea" in docs/JSDoc) is unaffected — only the rollup **contract** name changed.

**See also:** [`sdk-core` CHANGELOG](./sdk-core/CHANGELOG.md) · [`sdk-viem` CHANGELOG](./sdk-viem/CHANGELOG.md)

## Why

The `LineaRollup` L1 contract was renamed to `LinethRollup`. To keep the SDKs aligned with the deployed contract name, every type/class/parameter that mirrors it was renamed too.

## Affected packages

| Package | npm name | Published? |
|---|---|---|
| `sdk-core` | `@lfdt-lineth/sdk-core` | Yes |
| `sdk-ethers` | `@lfdt-lineth/sdk` | No (internal monorepo use only) |
| `sdk-viem` | `@lfdt-lineth/sdk-viem` | Yes |

## `@lfdt-lineth/sdk-core`

No public API changes — `getContractsAddressesByChainId()` keeps the same name, signature, and return shape. Only the *internal* constants it's built from were renamed (not exported from the package's `index.ts`, so this only matters if you deep-import a specific file):

| Old | New |
|---|---|
| `LINEA_ROLLUP_MAINNET_ADDRESS` | `LINETH_ROLLUP_MAINNET_ADDRESS` |
| `LINEA_ROLLUP_SEPOLIA_ADDRESS` | `LINETH_ROLLUP_SEPOLIA_ADDRESS` |

**Action required:** none, unless you import from `@lfdt-lineth/sdk-core/dist/constants/address` (or similar internal path) instead of the package root.

## `@lfdt-lineth/sdk` (sdk-ethers)

### Renamed exports (from `src/index.ts` / `clients/ethereum` / `core/clients/ethereum`)

| Old | New | Kind |
|---|---|---|
| `LineaRollupClient` | `LinethRollupClient` | class |
| `EthersLineaRollupLogClient` | `EthersLinethRollupLogClient` | class |
| `LineaRollupMessageRetriever` | `LinethRollupMessageRetriever` | class |
| `ILineaRollupClient` | `ILinethRollupClient` | interface |
| `ILineaRollupLogClient` | `ILinethRollupLogClient` | interface |
| `LineaRollup` | `LinethRollup` | typechain contract type |
| `LineaRollup__factory` | `LinethRollup__factory` | typechain factory |
| `testingHelpers.generateLineaRollupClient` | `testingHelpers.generateLinethRollupClient` | test helper |

### `LineaSDK` class (method names unchanged — only return types renamed)

| Method | Old return type | New return type |
|---|---|---|
| `getL1Contract()` | `LineaRollupClient` | `LinethRollupClient` |
| `getL1ContractEventLogClient()` | `EthersLineaRollupLogClient` | `EthersLinethRollupLogClient` |

**Action required:**
- Update imports of any renamed class/interface/type.
- Update explicit type annotations, e.g.:

```ts
// Before
const l1Contract: LineaRollupClient = sdk.getL1Contract();

// After
const l1Contract: LinethRollupClient = sdk.getL1Contract();
```

- If you use `testingHelpers` in tests, rename `generateLineaRollupClient` → `generateLinethRollupClient`. Its returned object's keys were also renamed:

| Old key | New key |
|---|---|
| `lineaRollupClient` | `linethRollupClient` |
| `lineaRollupLogClient` | `linethRollupLogClient` |

```ts
// Before
const { lineaRollupClient, lineaRollupLogClient } = testingHelpers.generateLineaRollupClient(...);

// After
const { linethRollupClient, linethRollupLogClient } = testingHelpers.generateLinethRollupClient(...);
```

## `@lfdt-lineth/sdk-viem`

### Parameter rename (non-breaking): `lineaRollupAddress` → `linethRollupAddress`

Every action/decorator that accepts a custom L1 rollup contract address gained a new `linethRollupAddress` field. The old `lineaRollupAddress` field is **deprecated but still fully functional** — existing code keeps working unchanged. If both are provided, `linethRollupAddress` takes precedence.

| Function / decorator | Parameter type | Field | Required? |
|---|---|---|---|
| `publicActionsL1(params)` | `PublicActionsL1Parameters` | `linethRollupAddress` (new) / `lineaRollupAddress` (deprecated) | optional |
| `walletActionsL1(params)` | `WalletActionsL1Parameters` | `linethRollupAddress` (new) / `lineaRollupAddress` (deprecated) | optional |
| `claimOnL1(client, params)` | `ClaimOnL1Parameters` | `linethRollupAddress` (new) / `lineaRollupAddress` (deprecated) | optional |
| `deposit(client, params)` | `DepositParameters` | `linethRollupAddress` (new) / `lineaRollupAddress` (deprecated) | optional |
| `getL2ToL1MessageStatus(client, params)` | `GetL2ToL1MessageStatusParameters` | `linethRollupAddress` (new) / `lineaRollupAddress` (deprecated) | optional |
| `getMessageProof(client, params)` | `GetMessageProofParameters` | `linethRollupAddress` (new) / `lineaRollupAddress` (deprecated) | optional |

**Action required:** none — `lineaRollupAddress` keeps working. We recommend migrating to `linethRollupAddress` at your own pace; `lineaRollupAddress` will be removed in a future major version.

```ts
// Still works (deprecated)
const l1Client = createPublicClient({ chain: sepolia, transport: http() }).extend(
  publicActionsL1({
    lineaRollupAddress: '0xYourCustomL1Rollup',
    l2MessageServiceAddress: '0xYourCustomL2MessageService',
  }),
);

const proof = await getMessageProof(client, {
  l2Client,
  messageHash,
  lineaRollupAddress: '0xYourCustomL1Rollup',
});
```

```ts
// Recommended going forward
const l1Client = createPublicClient({ chain: sepolia, transport: http() }).extend(
  publicActionsL1({
    linethRollupAddress: '0xYourCustomL1Rollup',
    l2MessageServiceAddress: '0xYourCustomL2MessageService',
  }),
);

const proof = await getMessageProof(client, {
  l2Client,
  messageHash,
  linethRollupAddress: '0xYourCustomL1Rollup',
});
```

**Note:** if you don't pass a custom address (i.e. you rely on the default resolved via `getContractsAddressesByChainId`), you have nothing to change here.

## What did NOT change

- Package names and npm scopes (`@lfdt-lineth/sdk-core`, `@lfdt-lineth/sdk`, `@lfdt-lineth/sdk-viem`).
- Function/method names — only types were renamed in `sdk-ethers`.
- `sdk-viem`'s `lineaRollupAddress` parameter — deprecated, not removed.
- `L2MessageService`-related names (unaffected by this rename).
- Any reference to the Linea network/chain itself (`linea`, `lineaSepolia` from `viem/chains`; "Linea Mainnet/Sepolia" in docs).

## Migration checklist

1. `sdk-ethers` (`@lfdt-lineth/sdk`, internal-only): rename imports `LineaRollupClient`, `EthersLineaRollupLogClient`, `LineaRollupMessageRetriever`, `ILineaRollupClient`, `ILineaRollupLogClient`, `LineaRollup`, `LineaRollup__factory` → their `Lineth*` counterparts, and `testingHelpers.generateLineaRollupClient` → `generateLinethRollupClient` in tests.
2. `sdk-viem` (`@lfdt-lineth/sdk-viem`, published): no action required. Optionally rename `lineaRollupAddress` → `linethRollupAddress` at your own pace across actions/decorators.
3. Rebuild and re-run your test suite.
