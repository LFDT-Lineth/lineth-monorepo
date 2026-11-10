# Hardhat 3 + viem conventions (`contracts/`)

How code in this package talks to Hardhat 3 and the chain after the Hardhat 2 → 3 migration. It is the reference for the Phase 4/5 rewrite (see `hardhat-v3-migration-plan.md`) and for new code. Follow it exactly; if something here does not fit a case, stop and report instead of improvising a new pattern.

## 1. Ground rules

- **ESM everywhere.** Relative imports end in `.js` (`./x.js`, `./dir/index.js`); JSON imports are default imports with `with { type: "json" }` (no named JSON imports); `import.meta.dirname` / `import.meta.filename` instead of `__dirname` / `__filename`.
- **Type-only imports use `import type { X } from "..."`** (statement form). Never `import { type X }` when every specifier is a type: under `verbatimModuleSyntax` that form still loads the module at runtime.
- **No ethers on the Hardhat side.** `ethers` stays only in the Hardhat-free island: `local-deployments-artifacts/*.ts` and the helpers only they use (`common/helpers/deployments.ts`, `common/helpers/feeOverrides.ts`, `common/helpers/normalize-address-args.ts`, `scripts/utils.ts` fee helpers). Everything else uses viem.
- **No TypeChain.** `typechain-types` is gone. Contract types come from hardhat-viem: `import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types"` → `ContractReturnType<"LinethRollup">`.
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

`hre.artifacts` stays on the HRE. `networkName` is `"default"` for the in-process network (was `"hardhat"`).

## 3. ethers → viem cheat sheet

| ethers / Hardhat 2 | viem / Hardhat 3 |
| --- | --- |
| `ethers.getSigners()` | tests: `getTestAccounts()` / `getAccountsFixture()`; scripts: `viem.getWalletClients()` |
| `SignerWithAddress` | tests: `TestAccount`; scripts: `WalletClient` from `@nomicfoundation/hardhat-viem/types` |
| `signer.address` | `account.address` (TestAccount) / `wallet.account.address` |
| `ethers.getContractFactory("X").deploy(a, b)` + `waitForDeployment()` | `viem.deployContract("X", [a, b], { client: { wallet }, libraries: { Mimc: mimc.address } })` |
| `ethers.getContractAt("X", addr, signer)` | `viem.getContractAt("X", addr, { client: { wallet } })` |
| `contract.connect(signer).fn(a)` | `contract.write.fn([a], { account: wallet.account })` (also `simulate`, `estimateGas`, `read` when `msg.sender` matters), as in Hardhat's viem testing guide |
| `contract.fn(a)` (view) | `contract.read.fn([a])` |
| `contract.fn.staticCall(a)` | `contract.simulate.fn([a])` → `.result` |
| `contract.fn.estimateGas(a)` | `contract.estimateGas.fn([a])` |
| `contract.fn(a, { value, gasLimit })` | `contract.write.fn([a], { value, gas })` |
| `await contract.getAddress()` / `contract.target` | `contract.address` |
| `tx.wait()` / receipt | `await publicClient.waitForTransactionReceipt({ hash })` |
| `contract.interface.encodeFunctionData("fn", [a])` | `encodeFunctionData({ abi: contract.abi, functionName: "fn", args: [a] })` |
| `contract.interface.parseLog(log)` | `decodeEventLog({ abi, data: log.data, topics: log.topics })` / `parseEventLogs` |
| `AbiCoder.defaultAbiCoder().encode(types, values)` | `encodeAbiParameters(parseAbiParameters("uint256, bytes32"), values)` or `encodeData(types, values)` helper |
| `ethers.solidityPacked(types, values)` | `encodePacked(types, values)` |
| `ethers.keccak256`, `concat`, `toUtf8Bytes`, `hexlify` | `keccak256`, `concat`, `stringToBytes`/`toBytes`, `bytesToHex`/`toHex` |
| `zeroPadValue(x, 32)` / `zeroPadBytes(x, n)` | `pad(x, { size: 32 })` (left) / `pad(x, { dir: "right", size: n })` |
| `toBeHex(n, 32)` | `toHex(n, { size: 32 })` |
| `parseEther`, `parseUnits(x, "gwei")`, `formatEther` | `parseEther`, `parseGwei(x)`, `formatEther` |
| `ZeroAddress`, `ZeroHash`, `MaxUint256` | `zeroAddress`, `zeroHash`, `maxUint256` |
| `ethers.getAddress`, `isAddress` | `getAddress`, `isAddress` |
| `ethers.provider.getBalance(a)` / `getBlock` / `getCode` | `publicClient.getBalance({ address })` / `getBlock()` / `getCode({ address })` (returns `undefined` for no code) |
| `ethers.provider.send("hardhat_setBalance", [a, v])` | `networkHelpers.setBalance(a, v)` or `testClient.setBalance({ address, value })` |
| `hardhat_impersonateAccount` / `stopImpersonating` | `networkHelpers.impersonateAccount(a)` / `testClient.stopImpersonatingAccount({ address })`; then `viem.getWalletClient(a)` |
| `evm_mine`, `evm_setAutomine`, `hardhat_setCoinbase`, `hardhat_setStorageAt` | `networkHelpers.mine()`, `testClient.setAutomine(bool)`, `testClient.setCoinbase({ address })`, `networkHelpers.setStorageAt(...)` |
| `hardhat_reset` | **Not supported by Hardhat 3's simulated network.** The node:test runner uses `isolation: "none"` (not configurable): all spec files share one process and one chain, exactly like Mocha under Hardhat 2. If a file's assertions do not depend on a pristine chain beyond what `loadFixture` provides, drop the reset and leave a one-line comment. If they do (fixed addresses, nonces, block numbers or timestamps), do not improvise: report it so a shared solution can be provided. |
| `loadFixture(fn)` from network-helpers | `networkHelpers.loadFixture(fn)` (fixture functions must be named, stable references) |
| `time.latest()` / `time.increase(n)` / `time.setNextBlockTimestamp(t)` | `networkHelpers.time.latest()` / `.increase(n)` / `.setNextBlockTimestamp(t)`; or test helpers `getLastBlockTimestamp`, `setFutureTimestampForNextBlock` |
| `new Wallet(pk)` / `HDNodeWallet` signing | `privateKeyToAccount(pk)` / `mnemonicToAccount(m, { addressIndex })` from `viem/accounts` |
| `Transaction.from({ type: 3, blobs, kzg })` + sign + broadcast | `walletClient.sendTransaction({ blobs, kzg: setupKzg(cKzg, trustedSetupPath), maxFeePerBlobGas, to, data })` |
| `signer.signTypedData(domain, types, value)` | `walletClient.signTypedData({ domain, types, primaryType, message })` + `parseSignature` |
| `upgrades.*` (OZ 2.x, `import { upgrades } from "hardhat"`) | `const oz = await upgrades(hre, connection)` from `@openzeppelin/hardhat-upgrades/viem`; contracts by **name**: `oz.deployProxy("X", args, opts)`, `oz.upgradeProxy(proxy, "X", opts)`, `oz.validateUpgrade(proxy, "X", { kind: "transparent" })`, `oz.erc1967.getAdminAddress(proxy)`; `admin.getInstance()` and `deployProxyAdmin()` no longer exist |
| Fully qualified contract names, e.g. `oz.upgradeProxy(p, "src/x/C.sol:C")` | Unchanged: `"src/x/C.sol:C"` (artifact names have no `project/` prefix; that prefix only appears in solc source names inside build-info). Short names work when unique. |

## 4. Tests (`test/hardhat/**`)

Runner: `node:test` through `hardhat test nodejs` (`hardhat test` runs it with the Solidity tests). Import `describe`, `it`, `before`, `beforeEach`, `after`, `afterEach` from `"node:test"`. Value assertions keep chai: `import { expect } from "chai"` (chai 6).

Foundation (do not modify from a batch; report needed changes):

- `test/hardhat/common/helpers/accounts.ts`: `TestAccount` (wallet client + checksummed `address`) and `getTestAccounts()`. Act as an account by passing its account to the call: `contract.write.fn([args], { account: admin.account })`; same for `simulate` (otherwise it simulates as the default account). There is no `connect()` helper; helpers that act for someone take the `TestAccount` (e.g. `submitCalldataBeforeFinalization(linethRollup, operator, config)`).
- `test/hardhat/common/helpers/before.ts`: `getAccountsFixture()` (same named accounts as before).
- `test/hardhat/common/deployment.ts`: `deployFromFactory(name, args?, config?)` (args are **one array**), `deployUpgradableFromFactory(name, args?, opts?, { libraries }?)`, `deployUpgradableWithConstructorArgs(...)`, `reinitializeUpgradeableProxy(proxy, abi, fn, args)` (returns the tx hash), `getUpgrades()` (OZ viem API for the shared connection, e.g. `(await getUpgrades()).silenceWarnings()`).
- `test/hardhat/common/helpers/expectations.ts`: `expectRevertWithCustomError(contract, call, name, args?)`, `expectRevertWithReason(call, reason)`, `expectEvent(contract, txHash, name, args?)`, `expectEvents(contract, txHash, [{ name, args }])`, `expectNoEvent(contract, txHash, name)`, `expectEventDirectFromReceiptData(contract, receipt, name, args, logIndex)`. Use these in specs. They are implemented with Hardhat's `viem.assertions` (`revertWith`, `revertWithCustomError[WithArgs]`, `emit[WithArgs]`) on `getConnection()`, so `useFreshChain()` suites assert on their own chain; only `expectNoEvent` and `expectEventDirectFromReceiptData` decode receipts themselves (no `viem.assertions` equivalent). `call` is any promise (write, read, simulation, deployment); for a deployment revert, `contract` can be an artifact (`{ abi }`). Expected args are type-checked against the contract's ABI and compared like `viem.assertions`: strictly (bigints for integers wider than 48 bits, so `1n` not `1`; `Hex` values, so wrap JSON fixture strings in `hex()`; structs as viem decodes them), addresses case-insensitively. Pass args inline or as a typed tuple, not through an untyped array variable. A position can be a `(value) => boolean` predicate, e.g. `anyValue` from `@nomicfoundation/hardhat-viem-assertions/predicates`. Events match only logs emitted by `contract.address`. Shared helpers that name errors by string widen the contract to `AssertionTarget`. Plain `viem.assertions.revert(call)` (revert for any reason) is used directly.
- `test/hardhat/common/helpers/general.ts`: `hex(string)`, `address(string)` validators for JSON fixture data, `generateRandomBytes`, `buildAccessErrorMessage(account, role)`.
- `test/hardhat/common/helpers/{accessControl,pauseManager,initialization,rollingHash,dataGeneration}.ts`: same helpers, viem shapes (`contract.read.x([...])`, `contract.write.x([...])`).
- `test/hardhat/common/types.ts`: shared data types now use `Hex` / `Address` (and `PauseTypeRole.pauseType: number`, `Eip1559Transaction.yParity: number`).
- `test/hardhat/common/helpers/connection.ts`: `getConnection()` (what every foundation helper uses) and `useFreshChain()`. A suite whose assertions depend on a pristine chain calls `useFreshChain()` first thing inside its top-level `describe` and gets its connection with `getConnection()`; it then runs on a brand-new simulated chain (Hardhat 3 has no `hardhat_reset`). Other suites keep sharing one chain.
- `test/hardhat/common/chai-bigint.d.ts`: chai comparers (`greaterThan`, `lessThan`, `within`, …) accept bigints.
- Addresses: `TestAccount.address` and the instances returned by the deployment helpers are checksummed, like ethers.
- OpenZeppelin beacons: deploy `pinnedUpgradeableBeaconArtifact` from `ignition/modules/lib/pinned.ts` (OZ v4.4.1, the beacon the Hardhat 2 plugin deployed), not `oz.deployBeacon` (OZ 5 beacon); see `scripts/tokenBridge/test/deployBridgedTokenBeacon.ts`.

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

- A fixture returning several values returns an object; keep fixture functions at module scope (stable identity for `loadFixture`). Fixtures may nest `loadFixture` (supported through the repo's `hardhat-network-helpers` patch, as under Hardhat 2).
- Passing a struct to a write: pass the object with the ABI field names (viem encodes by name).
- `readonly` tuples: viem returns struct reads as objects (named) or arrays (unnamed outputs); adjust assertions accordingly.
- `chai`'s `expect(x).to.equal(y)`: compare bigints with bigints (`1n`), numbers with numbers; addresses from reads are checksummed.
- `describe.skip` / `it.skip` keep working (`_manualTesting/*`).

Verification per file:

```bash
pnpm exec tsc --noEmit -p tsconfig.json 2>&1 | grep '^<path>' | cut -c1-220
pnpm exec hardhat test nodejs <path>
```

## 5. Scripts and tasks (`scripts/**`, tasks)

- A task action is a default-exported `async function (taskArgs: TaskArguments, hre: HardhatRuntimeEnvironment)`; acquire `const connection = await hre.network.getOrCreate()` inside it.
- `hardhat run` scripts: `import hre, { network } from "hardhat"`, then `await network.getOrCreate()` inside `main()`.
- Signing goes through the signer bridge API (§6); never read `DEPLOYER_PRIVATE_KEY` directly.
- OZ upgrades: `const oz = await upgrades(hre, connection)` from `@openzeppelin/hardhat-upgrades/viem`; pass `{ client: { public: publicClient, wallet } }` for the signer.
- Hardhat-deploy lookups (`hre.deployments.get`, `deployments/<network>/<X>.json`) are gone: use the address registry (`requireAddressFromRegistryOrEnv`) or Ignition's `ignition/deployments/<id>/deployed_addresses.json`.

## 6. Signer UI (`hardhat-plugins/signer-ui/`)

`--signer-ui` (or `HARDHAT_SIGNER_UI=true`) turns on browser signing for the code a task runs inside `withSignerUi`: for that call, a `network.onRequest` handler registered on `hre.hooks` answers `eth_accounts`/`eth_requestAccounts` with the browser wallet address and routes `eth_sendTransaction` to the browser, returning the hash. Everything that sends through the Hardhat provider (viem wallet clients, OZ Upgrades, Ignition) signs in the browser with no per-call code; nothing outside a scope does.

```ts
withSignerUi<T>(hre: HardhatRuntimeEnvironment, label: string, fn: () => Promise<T>): Promise<T>  // session for fn, closed when it settles
getUiWalletClient(connection: NetworkConnection): Promise<WalletClient>           // UI wallet or DEPLOYER_PRIVATE_KEY account (+ EXPECTED_SIGNER_ADDRESS check)
setUiTransactionContext(details: UiTransactionDetails): void
warnIfUsingPrivateKeySigning(details?: { networkName?: string; scriptContext?: string; uiSupported?: boolean }): void
```

Every broadcast in UI mode is appended to `cache/signer-ui/broadcasts-<chainId>.jsonl` (`{ sender, nonce, hash, at }`) so `hardhat deploy:recover` can resolve an Ignition deployment stuck on an unsent `TRANSACTION_PREPARE_SEND` (see the plan's Phase 0 results).

## 7. Deployments (Ignition)

- Modules live in `ignition/modules/<src folder>/<Name>.ts` (`buildModule("<Name>", ...)`; the file name is the module id, so `hardhat ignition deploy <Name>` finds it), composites in `stacks/`, implementation upgrades in `upgrades/`, helpers in `lib/`.
- Upgradeable production contracts use the **pinned OpenZeppelin 4.x** (v4.8.3, byte-identical to mainnet) `ProxyAdmin` / `TransparentUpgradeableProxy` artifacts from `deployments/bytecode/mainnet-proxy` (via `deployPinnedTransparentProxy` / `deployPinnedBeacon` in `ignition/modules/lib/proxy.ts`, which also declare each implementation's `unsafeAllow`), never the OZ plugin's `deployProxy` (which deploys OZ 5.0 proxies). Beacons likewise use the pinned OpenZeppelin v4.4.1 `UpgradeableBeacon` artifact from the same directory, never `oz.deployBeacon` (OZ 5 beacon).
- Inputs come from `m.getParameter(...)`; the module's `<Name>.deploy.ts` builds them from the same env vars and registry rules the Hardhat 2 scripts used (`parameters`), and may run post-deploy steps (`afterDeploy`).
- Storage-layout safety: the deploy plugin (`hardhat-plugins/deploy/`, an override of `ignition deploy`) runs OZ `validateImplementation` / `validateUpgrade` before deploying and `forceImport` after.
- Verification: `hardhat ignition deploy ... --verify` / `hardhat ignition verify <id>`; one-off: `verifyContract({ address, constructorArgs, libraries, contract, provider: "etherscan" }, hre)` from `@nomicfoundation/hardhat-verify/verify`.
