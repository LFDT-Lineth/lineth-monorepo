# Hardhat 3 + viem conventions (`contracts/`)

How TypeScript code in this package talks to Hardhat 3 and the chain.

## 1. Ground rules

- **ESM everywhere.** Relative imports end in `.js` (`./x.js`, `./dir/index.js`); JSON imports are default imports with `with { type: "json" }` (no named JSON imports); `import.meta.dirname` / `import.meta.filename` instead of `__dirname` / `__filename`.
- **Type-only imports use `import type { X } from "..."`** (statement form). Never `import { type X }` when every specifier is a type: under `verbatimModuleSyntax` that form still loads the module at runtime.
- **No ethers on the Hardhat side.** `ethers` stays only in the Hardhat-free island: `local-deployments-artifacts/*.ts` and the helpers only they use (`common/helpers/deployments.ts`, `common/helpers/feeOverrides.ts`, `common/helpers/normalize-address-args.ts`, `scripts/utils.ts` fee helpers). Everything else uses viem.
- **Contract types come from hardhat-viem:** `import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types"` → `ContractReturnType<"LinethRollup">`.
- **No `any`, no `@ts-ignore`/`@ts-expect-error`, no `as unknown as X`.** Narrow with type guards (`isHex`, `isAddress`), validate untyped data with `hex()` / `address()` (test helpers) or `getAddress()`.
- **Never acquire a network connection at module top level.** Call `await network.getOrCreate()` (or `hre.network.getOrCreate()`) inside functions, fixtures, hooks or tests. `getOrCreate()` at runtime returns the shared connection; at module scope it does not share state.
- **Hex and addresses are typed.** Bytes, hashes and calldata are `Hex`; addresses are `Address` (both from `viem`). Integers are `bigint`, except Solidity types up to 48 bits (`uint8`, `uint16`, `uint32`, enums), which viem reads and writes as `number`.

## 2. Connection

```ts
import hre, { network } from "hardhat";

const { viem, networkHelpers, networkName, networkConfig, provider } = await network.getOrCreate();
const publicClient = await viem.getPublicClient();
const [deployer] = await viem.getWalletClients();
const testClient = await viem.getTestClient(); // simulated network only
```

`hre.artifacts` stays on the HRE. `networkName` is `"default"` for the in-process network.

## 3. Non-obvious viem patterns

- Act as an account by passing it to the call: `contract.write.fn([a], { account: wallet.account })`; the same for `simulate` (otherwise it simulates as the default account). A `staticCall` is `contract.simulate.fn([a])` → `.result`.
- `publicClient.getCode({ address })` returns `undefined` (not `"0x"`) when there is no code.
- Solidity integers up to 48 bits (`uint8` … `uint48`, enums) are `number`; wider ones are `bigint`.
- Hardhat 3's simulated network has no `hardhat_reset`; all spec files share one chain. A suite that needs a pristine chain calls `useFreshChain()` (§4).
- OpenZeppelin Upgrades: `const oz = await upgrades(hre, connection)` from `@openzeppelin/hardhat-upgrades/viem`; contracts are passed by **name** (`oz.upgradeProxy(proxy, "X", opts)`, `oz.validateUpgrade(proxy, "X", { kind: "transparent" })`, `oz.erc1967.getAdminAddress(proxy)`). Fully qualified names are `"src/x/C.sol:C"` (no `project/` prefix).
- Blob transactions: `walletClient.sendTransaction({ blobs, kzg: setupKzg(cKzg, trustedSetupPath), maxFeePerBlobGas, to, data })`.

## 4. Tests (`test/hardhat/**`)

Runner: `node:test` through `hardhat test nodejs` (`hardhat test` runs it with the Solidity tests). Import `describe`, `it`, `before`, `beforeEach`, `after`, `afterEach` from `"node:test"`. Value assertions keep chai: `import { expect } from "chai"` (chai 6).

Shared helpers:

- `test/hardhat/common/helpers/accounts.ts`: `TestAccount` (wallet client + checksummed `address`) and `getTestAccounts()`. Act as an account by passing its account to the call: `contract.write.fn([args], { account: admin.account })`; same for `simulate` (otherwise it simulates as the default account). There is no `connect()` helper; helpers that act for someone take the `TestAccount` (e.g. `submitCalldataBeforeFinalization(linethRollup, operator, config)`).
- `test/hardhat/common/helpers/before.ts`: `getAccountsFixture()`.
- `test/hardhat/common/deployment.ts`: `deployFromFactory(name, args?, config?)` (args are **one array**), `deployUpgradableFromFactory(name, args?, opts?, { libraries }?)`, `deployUpgradableWithConstructorArgs(...)`, `reinitializeUpgradeableProxy(proxy, abi, fn, args)` (returns the tx hash), `getUpgrades()` (OZ viem API for the shared connection, e.g. `(await getUpgrades()).silenceWarnings()`).
- `test/hardhat/common/helpers/expectations.ts`: `expectRevertWithCustomError(contract, call, name, args?)`, `expectRevertWithReason(call, reason)`, `expectEvent(contract, txHash, name, args?)`, `expectEvents(contract, txHash, [{ name, args }])`, `expectNoEvent(contract, txHash, name)`, `expectEventDirectFromReceiptData(contract, receipt, name, args, logIndex)`. Use these in specs. They are implemented with Hardhat's `viem.assertions` (`revertWith`, `revertWithCustomError[WithArgs]`, `emit[WithArgs]`) on `getConnection()`, so `useFreshChain()` suites assert on their own chain; only `expectNoEvent` and `expectEventDirectFromReceiptData` decode receipts themselves (no `viem.assertions` equivalent). `call` is any promise (write, read, simulation, deployment); for a deployment revert, `contract` can be an artifact (`{ abi }`). Expected args are type-checked against the contract's ABI and compared like `viem.assertions`: strictly (bigints for integers wider than 48 bits, so `1n` not `1`; `Hex` values, so wrap JSON fixture strings in `hex()`; structs as viem decodes them), addresses case-insensitively. Pass args inline or as a typed tuple, not through an untyped array variable. A position can be a `(value) => boolean` predicate, e.g. `anyValue` from `@nomicfoundation/hardhat-viem-assertions/predicates`. Events match only logs emitted by `contract.address`. Shared helpers that name errors by string widen the contract to `AssertionTarget`. Plain `viem.assertions.revert(call)` (revert for any reason) is used directly.
- `test/hardhat/common/helpers/general.ts`: `hex(string)`, `address(string)` validators for JSON fixture data, `generateRandomBytes`, `buildAccessErrorMessage(account, role)`.
- `test/hardhat/common/helpers/{accessControl,pauseManager,initialization,rollingHash,dataGeneration}.ts`: same helpers, viem shapes (`contract.read.x([...])`, `contract.write.x([...])`).
- `test/hardhat/common/types.ts`: shared data types now use `Hex` / `Address` (and `PauseTypeRole.pauseType: number`, `Eip1559Transaction.yParity: number`).
- `test/hardhat/common/helpers/connection.ts`: `getConnection()` (what every foundation helper uses) and `useFreshChain()`. A suite whose assertions depend on a pristine chain calls `useFreshChain()` first thing inside its top-level `describe` and gets its connection with `getConnection()`; it then runs on a brand-new simulated chain (Hardhat 3 has no `hardhat_reset`). Other suites keep sharing one chain.
- `test/hardhat/common/chai-bigint.d.ts`: chai comparers (`greaterThan`, `lessThan`, `within`, …) accept bigints.
- Addresses: `TestAccount.address` and the instances returned by the deployment helpers are checksummed.
- OpenZeppelin beacons: deploy `pinnedUpgradeableBeaconArtifact` from `ignition/modules/lib/pinned.ts` (OZ v4.4.1), not `oz.deployBeacon` (OZ 5 beacon); see `scripts/tokenBridge/test/deployBridgedTokenBeacon.ts`.

Patterns:

```ts
import { expect } from "chai";
import { network } from "hardhat";
import { beforeEach, describe, it } from "node:test";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

import { deployUpgradableFromFactory } from "../common/deployment.js";
import { type TestAccount, expectEvent, getAccountsFixture } from "../common/helpers/index.js";

async function deployFixture() {
  return deployUpgradableFromFactory("TestPauseManager", [pauseTypeRoles, unpauseTypeRoles]);
}

describe("PauseManager", () => {
  let admin: TestAccount;
  let pauseManager: ContractReturnType<"TestPauseManager">;

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ admin } = await networkHelpers.loadFixture(getAccountsFixture));
    pauseManager = await networkHelpers.loadFixture(deployFixture);
  });

  it("pauses", async () => {
    await expectEvent(pauseManager, pauseManager.write.pauseByType([GENERAL_PAUSE_TYPE], { account: admin.account }), "Paused", [
      admin.address,
      GENERAL_PAUSE_TYPE,
    ]);
    expect(await pauseManager.read.isPaused([GENERAL_PAUSE_TYPE])).to.equal(true);
  });
});
```

- A fixture returning several values returns an object; keep fixture functions at module scope (stable identity for `loadFixture`). Fixtures may nest `loadFixture` (supported through the repo's `hardhat-network-helpers` patch).
- Passing a struct to a write: pass the object with the ABI field names (viem encodes by name).
- `readonly` tuples: viem returns struct reads as objects (named) or arrays (unnamed outputs); adjust assertions accordingly.
- `chai`'s `expect(x).to.equal(y)`: compare bigints with bigints (`1n`), numbers with numbers; addresses from reads are checksummed.
- `describe.skip` / `it.skip` keep working (`_manualTesting/*`).

## 5. Scripts and tasks (`scripts/**`, tasks)

- A task action is a default-exported `async function (taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment)`; acquire `const connection = await hre.network.getOrCreate()` inside it.
- `hardhat run` scripts: `import hre, { network } from "hardhat"`, then `await network.getOrCreate()` inside `main()`.
- Signing goes through the signer bridge API (§6); never read `DEPLOYER_PRIVATE_KEY` directly.
- OZ upgrades: `const oz = await upgrades(hre, connection)` from `@openzeppelin/hardhat-upgrades/viem`; pass `{ client: { public: publicClient, wallet } }` for the signer.
- Addresses come from the address registry (`requireAddressFromRegistryOrEnv`) or Ignition's `ignition/deployments/<id>/deployed_addresses.json`.

## 6. Signer UI (`hardhat-plugins/signer-ui/`)

Code that signs runs inside `withSignerUi`, so `--signer-ui` routes it to the browser wallet ([signer-ui/README.md](../signer-ui/README.md)):

```ts
withSignerUi<T>(hre: HardhatRuntimeEnvironment, label: string, fn: () => Promise<T>): Promise<T>  // session for fn, closed when it settles
getUiWalletClient(connection: NetworkConnection): Promise<WalletClient>           // UI wallet or DEPLOYER_PRIVATE_KEY account (+ EXPECTED_SIGNER_ADDRESS check)
warnIfUsingPrivateKeySigning(details?: { networkName?: string; scriptContext?: string }): void
```

## 7. Deployments (Ignition)

- Modules live in `ignition/modules/<src folder>/<Name>.ts` (`buildModule("<Name>", ...)`; the file name is the module id, so `hardhat ignition deploy <Name>` finds it), composites in `stacks/`, implementation upgrades in `upgrades/`, helpers in `lib/`.
- Upgradeable production contracts use the **pinned OpenZeppelin 4.x** (v4.8.3, byte-identical to mainnet) `ProxyAdmin` / `TransparentUpgradeableProxy` artifacts from `deployments/bytecode/mainnet-proxy` (via `deployPinnedTransparentProxy` / `deployPinnedBeacon` in `ignition/modules/lib/proxy.ts`, which also declare each implementation's `unsafeAllow`), never the OZ plugin's `deployProxy` (which deploys OZ 5.0 proxies). Beacons likewise use the pinned OpenZeppelin v4.4.1 `UpgradeableBeacon` artifact from the same directory, never `oz.deployBeacon` (OZ 5 beacon).
- Inputs come from `m.getParameter(...)`; the module's `<Name>.deploy.ts` builds them from env vars and the address registry (`parameters`), and may run post-deploy steps (`afterDeploy`).
- Storage-layout safety: the deploy plugin (`hardhat-plugins/deploy/`, an override of `ignition deploy`) runs OZ `validateImplementation` / `validateUpgrade` before deploying and `forceImport` after.
- Verification: `hardhat ignition deploy ... --verify` / `hardhat ignition verify <id>`; one-off: `verifyContract({ address, constructorArgs, libraries, contract, provider: "etherscan" }, hre)` from `@nomicfoundation/hardhat-verify/verify`.
