# Hardhat 2 → Hardhat 3 migration plan (`contracts/`), viem edition

Status: **Phase 6 done** (gate passed), plus the post-migration plugin restructure (§6, end). Phases 0–6 done, uncommitted; the migration is complete. Remaining: the runtime checks listed under Phase 6 and the §7 follow-ups. Written 2026-11-09 against `main` at `d59dcbfa66`.
Method: the repo skill `.claude/skills/migrate-hardhat2-to-hardhat3` (six sequential phases, user checkpoint after each), adapted to this package. This revision targets the **viem** stack that Hardhat 3 recommends (`hardhat-toolbox-viem`: viem + `node:test` + viem assertions + Ignition), replacing the ethers/Mocha/TypeChain/hardhat-deploy stack.

---

## 1. Summary

The `contracts` package is the only Hardhat consumer in the monorepo. It is a production deployment toolchain: 33 hardhat-deploy scripts, 11 custom tasks, a 2,300-line browser-signer bridge built on ethers `AbstractSigner`, 50 Mocha spec files (279 `describe`, ~1,180 `it`), 18 Hardhat-free `ts-node` deploy scripts used by the e2e stack, and five plugins with **no Hardhat 3 port**.

Hard constraint: `@openzeppelin/contracts` and `@openzeppelin/contracts-upgradeable` stay at **4.9.6**. Hardhat 3 does not care about this. The OpenZeppelin *Hardhat Upgrades plugin* v4 (needed for HH3) deploys OZ **5.0** proxy bytecode, which is the one point of friction. The viem + Ignition design below removes it: Ignition deploys the pinned OZ 4.9.6 `TransparentUpgradeableProxy`/`ProxyAdmin` bytecode already committed in `deployments/bytecode/mainnet-proxy`, and the OZ plugin is used only for storage-layout validation and manifest tracking (§3, §5.3).

Choosing viem turns two of the six phases from a port into a rewrite:

| Area | Ethers plan (previous revision) | viem plan (this revision) |
| --- | --- | --- |
| Tests | Mechanical: `.reverted` → `.revert(ethers)`, `loadFixture` → `networkHelpers.loadFixture`, keep Mocha/Chai/TypeChain | Rewrite contract interaction in 50 specs + helpers: ethers `Contract` → viem `read`/`write`/`simulate`, signers → wallet clients, TypeChain types → `ContractReturnType<"Name">`, Mocha → `node:test`, 3 assertion wrappers reimplemented on `viem.assertions` |
| Deploy scripts | Keep hardhat-deploy shape under a thin in-repo runner | Redesign as Ignition modules + a parameters layer fed from env/registry; 6 non-deployment "scripts" become tasks |
| Signer bridge | Swap `hre.ethers` for a connection | Rebuild signing as an `eth_sendTransaction` interceptor on the Hardhat provider (network `onRequest` hook) so Ignition, viem wallet clients and the OZ plugin all route through it |

Recommended shape of the work:

| Step | What | Lands as |
| --- | --- | --- |
| 0 | Unblock: scope the workspace-wide `zod`/`undici` pnpm overrides; take the §4 decisions; PoC the signer-bridge interceptor against Ignition | small PR + decisions + 2–3 day spike |
| 1–6 | The skill's six phases on one migration branch, one checkpoint commit per phase | one PR (CI cannot pass half-migrated) |
| 7 | Follow-ups that do not block CI: docgen script, `local-deployments-artifacts` to viem, Solidity tests in CI, keystore | separate PRs |

Rough effort: **4–6 engineer-weeks** for steps 1–6 (vs 2–3 for the ethers path), split roughly: setup/config 0.5, Ignition modules + parameters + bridge 1.5–2, test rewrite 2–3, validation/CI/docs 0.5. Phases 4 and 5 should be fanned out to sub-agents per the skill thresholds.

---

## 2. Current state (inventory)

### 2.1 Toolchain

| Item | Current | Notes |
| --- | --- | --- |
| Node | 24.22.1 (`engines >=24.22.1`) | HH3 needs ≥22.13. OK. |
| Package manager | pnpm 12.6.0 workspace, `minimumReleaseAge: 4320` (3 days), `trustPolicy: no-downgrade` | All HH3 packages below are ≥3 days old. |
| Hardhat | 2.29.1, CommonJS, `ts-node` via `TS_NODE_TRANSPILE_ONLY=1` | Target `hardhat@3.21.2`. |
| TypeScript | 5.9.3 (catalog) | Root `tsconfig.json` already `module: nodenext`, `strict`, `exactOptionalPropertyTypes`. |
| viem | 2.57.11 (catalog) | Already used by `e2e` (67 files, incl. `viem/linea`), `postman`, `operations/*`, `ts-libs/sdk-viem`, `contracts/signer-ui`, `contracts/integrity-verifier/verifier-viem`. HH3 viem plugins need `viem ^2.47.6`: satisfied. |
| solc | 0.8.33, `evmVersion: osaka`, `viaIR` via `ENABLE_VIA_IR`, per-file optimizer overrides in `hardhat_overrides.ts` | Config shape backwards-compatible in HH3. |
| Foundry | `foundry.toml` (`src/`, `cancun`, `test/foundry/*.t.sol`) | Forge tests are **not in CI**; CI installs Foundry only because `hardhat-foundry` requires it. |

### 2.2 Hardhat plugins and their HH3 status

| Package (current) | Used for | HH3 status | Plan |
| --- | --- | --- | --- |
| `@nomicfoundation/hardhat-toolbox` 4.0.0 | ethers, chai matchers, network-helpers, verify, typechain, gas reporter, coverage | Replaced by `@nomicfoundation/hardhat-toolbox-viem@5.0.7` (peers: `hardhat-viem`, `hardhat-viem-assertions`, `hardhat-node-test-runner`, `hardhat-network-helpers`, `hardhat-verify`, `hardhat-keystore`, `hardhat-ignition`, `hardhat-ignition-viem`, `ignition-core`, `viem`) | Swap. On pnpm every peer is installed explicitly. |
| `@nomicfoundation/hardhat-ethers` 3.1.3, `ethers` 6.17 (chain interaction) | `hre.ethers` everywhere | Dropped from the Hardhat side | `connection.viem` (`getPublicClient`, `getWalletClients`, `getTestClient`, `deployContract`, `getContractAt`). `ethers` stays only as a plain dependency of `local-deployments-artifacts/*.ts` until §7. |
| `@nomicfoundation/hardhat-network-helpers` 1.1.2 | `loadFixture` (48 files), `time` (13), `setBalance`, `setNonce` | v3.0.12 | `connection.networkHelpers.*` (unchanged by the viem choice). |
| `@nomicfoundation/hardhat-chai-matchers` (via toolbox), `chai` 4.5 | `.reverted*`, `.emit`, `.withArgs` | Replaced by `@nomicfoundation/hardhat-viem-assertions@3.1.5` (`viem.assertions.revert*`, `emit*`, `balancesHaveChanged`) | Reimplement the three in-repo wrappers (§5.4). `chai` kept (v6, ESM) for value assertions, or swapped for `node:assert`; decision §4.3. |
| `@nomicfoundation/hardhat-verify` 2.1.3 | `run("verify:verify")` from a spawned child `hardhat run` | v3.1.4; in-process `verifyContract(args, hre)` from `@nomicfoundation/hardhat-verify/verify`; Ignition has `ignition verify` | Deploys verify via Ignition; one-off verifications via `verifyContract`. Delete `run-verify-task.ts`/`verify-task-args.ts`. |
| `@typechain/hardhat` 9.1.0, `@typechain/ethers-v6`, `typechain` (catalog) | `typechain-types/` (67 importing files) | Not used with viem | Remove packages and the two catalog entries (only `contracts` uses them). Types come from `hardhat-viem` (`ContractReturnType<"Name">`, ABI-typed `read`/`write`). |
| `hardhat-deploy` 0.14.1 | `DeployFunction` (71), `func.tags` (33), `hre.deployments.get` fallback (5 tasks, dead: `save` never called), `getNamedAccounts`, `TASK_DEPLOY_RUN_DEPLOY` override | v2.3.0 = rocketh rewrite (viem, own proxies, own signer protocols) | **Replace with Hardhat Ignition** (decision §4.1). |
| `@openzeppelin/hardhat-upgrades` 2.5.1 | `deployProxy` (26), `upgradeProxy` (8), `deployImplementation` (6), `forceImport` (6), `validateUpgrade` (4), beacons (9), `admin.*` (3), `erc1967` (1), `deployProxyAdmin` (1), `silenceWarnings` (3) | v4.1.0: HH3, ESM-only, `upgrades(hre, connection)` from `@openzeppelin/hardhat-upgrades/viem`, contracts by **name**, returns viem instances | Keep for validation (`validateImplementation`, `validateUpgrade`, `forceImport`), `erc1967.*`, operational `upgradeProxy`/`admin.*` scripts, and test fixtures. Not used to deploy production proxies (§3). |
| `hardhat-gas-reporter` 2.3.0 | `REPORT_GAS=true hardhat test` | Built in: `hardhat test --gas-stats` | Remove. |
| `solidity-coverage` 0.8.17 + `.solcover.js` | `coverage/coverage-final.json` → Codecov | Built in: `hardhat test --coverage` → `coverage/lcov.info` + HTML; no istanbul JSON, no documented exclude list | Remove; CI uploads `lcov.info`; `skipFiles` → `codecov.yml` `ignore:` (§5.7). |
| `@nomicfoundation/hardhat-foundry` 1.2.1 | Load forge remappings | v3.0.5 exists but only for forge-generated remappings | **Drop.** All `src/` imports are npm-scoped (`@openzeppelin/*`, `@chainlink/*`, `solady/*`), resolved natively by HH3. |
| `hardhat-storage-layout` 0.1.7 | Config import only; `storageLayout/` empty | **No HH3 port** | Drop; document `forge inspect <C> storage-layout`. |
| `solidity-docgen` 0.6.0-beta.36 | `docs/api/*.mdx` with custom templates | **No HH3 port**; exports `docgen()` | Drop plugin registration; script in §7. |
| `hardhat-tracer` 2.8.3 | Already disabled | Not HH3-compatible | Remove. HH3 `-vvvv`. |
| custom subtask `solcWarningFilter.ts` | Suppress solc 2018 in `SafeExecutionConditions.sol` | Compile subtasks gone | `hardhat-ignore-warnings@0.3.0` (peer `hardhat ^3.1`) or accept the warning. |

Independent of Hardhat, keep: `solhint`, `prettier-plugin-solidity`, `forge-std`, `solady`, `c-kzg` (+ `node-gyp` CI rebuild; viem's `setupKzg` consumes it directly), `@chainlink/contracts`, `@lidofinance/lsv-cli`, `@lodestar/types`, `@chainsafe/persistent-merkle-tree`.

### 2.3 Code that touches Hardhat or ethers APIs

Counted over `deploy/`, `scripts/`, `test/`, `common/` (`*.ts`, excluding generated). Test-only counts are marked (T).

| Pattern | Count | viem / HH3 replacement |
| --- | --- | --- |
| `import … from "hardhat"` (`ethers`, `upgrades`, `network`, `artifacts`) | 88 files | `import hre from "hardhat"`; `const { viem, networkHelpers, networkName, networkConfig, provider } = await hre.network.getOrCreate()` at point of use (never module top level). |
| `SignerWithAddress` | 41 files | `WalletClient` from `viem` (or `Awaited<ReturnType<typeof viem.getWalletClients>>[number]`). |
| `@nomicfoundation/hardhat-network-helpers` imports | 51 files | `networkHelpers.*` |
| `getContractFactory(` / `waitForDeployment` | 87 (T) / 49 (T), 33 / 19 (scripts) | `viem.deployContract("Name", args, { client, libraries })`; `viem.getContractAt("Name", address, { client })`. |
| `.connect(signer)` | 931 (T), 9 (scripts) | `contract.write.fn(args, { account })` or `viem.getContractAt(..., { client: { wallet } })` per actor; the fixtures return per-role contract handles instead of re-connecting. |
| `.getAddress()` / `.address` | 614 / 736 (T) | `contract.address` / `walletClient.account.address`. |
| `ethers.provider.*` (`getBalance`, `getBlock`, `getCode`, `getFeeData`, `send("hardhat_*")`) | 251 (T) | `publicClient.*`, `testClient.*` (`setBalance`, `impersonateAccount`, `mine`, `setCoinbase`, `setStorageAt`, `reset`) or `networkHelpers`. 83 raw `provider.send` sites across 15 files. |
| `staticCall` / `estimateGas` | 44 (T) | `contract.simulate.fn` / `publicClient.estimateContractGas`. |
| `AbiCoder`, `Interface`, `encodeFunctionData`, `.interface.` | 47 + 8 + 31 + 31 (T); 44 + 9 (scripts) | `encodeAbiParameters`, `decodeAbiParameters`, `encodeFunctionData({ abi, functionName, args })`, `parseAbi`. |
| `keccak256`, `solidityPacked`, `toBeHex`/`zeroPadValue`/`hexlify`, `parseEther`, `ZeroAddress`/`ZeroHash` | 98 / 3 / 106 / 91 / 167 (T) | `keccak256`, `encodePacked`, `toHex`/`pad`, `parseEther`, `zeroAddress`/`zeroHash` from `viem`. Pure utilities; mechanical. |
| Blob (EIP-4844) transactions: `Transaction.from({ type: 3, kzg, blobs })`, `HDNodeWallet.signTransaction`, `broadcastTransaction` | 215 (T, `rollup/helpers/blob.ts`, `BlobSubmission.ts`, `Finalization.ts`) | `setupKzg(cKzg, trustedSetupPath)` from `viem`, `walletClient.sendTransaction({ blobs, kzg, maxFeePerBlobGas, to, data })`, `publicClient.waitForTransactionReceipt`. `getWalletForIndex` → `mnemonicToAccount(..., { addressIndex })`. |
| EIP-712 permit (`signTypedData`) | 1 helper (`bridging/token/helpers/permitHelper.ts`) | `walletClient.signTypedData({ domain, types, primaryType: "Permit", message })`, `parseSignature`. |
| EIP-7702 scripts (`scripts/testEIP7702/*`) | 3 scripts | viem `signAuthorization` / `sendTransaction({ authorizationList })`. |
| Chai contract matchers raw | 7 sites, 3 files | `viem.assertions.*` |
| In-repo assertion wrappers `expectRevertWithCustomError` / `expectEvent` / `expectRevertWithReason` / `expectEvents` | 435 / 152 / 75 / 1 call sites | Reimplement the 4 wrappers in `test/hardhat/common/helpers/expectations.ts` on `viem.assertions.revertWithCustomErrorWithArgs`, `emitWithArgs`, `revertWith`. Call sites change signature only where they pass ethers objects. |
| Mocha globals `describe`/`it`/`before*` | 279 / ~1,180 / 80 sites, 50 spec files, 0 `this.timeout`, 7 `.skip/.only` | `import { describe, it, before, beforeEach } from "node:test"` (decision §4.3). |
| `hardhat-deploy/types` `DeployFunction` + `func.tags` | 33 scripts | Ignition modules (§5.3) + tasks for the 6 non-deployments. |
| `hre.network.name` | ~45 sites | `networkName` from the connection; branches on `"hardhat"` must also accept `"default"`. |
| `upgrades.admin.getInstance()`, `deployProxyAdmin` | 2 sites | Removed in plugin v3; use `erc1967.getAdminAddress` + `viem.getContractAt("ProxyAdmin", admin)`; drop `deployProxyAdmin`. |
| Bare self-imports `from "contracts/…"` | 58 sites, 44 files | Only resolve via the pnpm workspace self-link + ts-node; rewrite to relative `.js` imports. |
| Extensionless relative imports / JSON imports / `__dirname` | 552 / 62 / 17 | `.js` (+`/index.js` barrels), `with { type: "json" }`, `import.meta.dirname`. |
| `createRequire` escape hatches | 2 (`hardhat.config.ts`, `signer-ui-bridge.ts`) | Removed by lazy `setAction(() => import(...))` tasks. |
| `task(...).addOptionalParam().setAction()` | 11 task files | Builder pattern + `addOption({ name, defaultValue: "" })`, registered in `tasks: []`. |

### 2.4 Non-Hardhat code affected by `"type": "module"`

| Location | Runner today | Impact |
| --- | --- | --- |
| `local-deployments-artifacts/*.ts` (18 ethers+dotenv scripts, no Hardhat) | `pnpm exec ts-node …` from `makefile-contracts.mk` (included by the root `Makefile`; drives the local/e2e stack: `make deploy-contracts`, `deploy-lineth-rollup-v*`, …) | `ts-node` CJS stops working under ESM → `tsx` (catalog). Keep them on `ethers` in this PR (decision §4.4); viem conversion in §7. |
| `scripts/deployment/validate*.ts`, `validate:*` scripts | `ts-node` | `tsx`; `validateDeployScripts.ts` must learn Ignition modules/parameters instead of `deploy/*.ts` env lookups. |
| `package.json` `balance` → `scripts/balanceOf.ts` | n/a | File missing; delete the script. |
| `.solcover.js` | solidity-coverage | Deleted with the plugin. |
| `eslint.config.mjs`, `prettier.config.mjs` | ESM already | No change; ESLint has only `import/order`. |
| `signer-ui/` (Next.js, viem + wagmi, own package) | independent | Bridge protocol unchanged; `shared/chainCatalog.json` read via JSON import. |
| `integrity-verifier/*` | independent | No change. |
| `e2e/scripts/generateAbi.ts` | reads `local-deployments-artifacts/**.json` (abi, bytecode, linkReferences) | HH3 artifacts keep those fields; regenerate once and diff. |

### 2.5 Workspace-level blockers (must be fixed before Phase 1)

`pnpm-workspace.yaml` applies **unscoped** overrides to the whole workspace:

| Override | HH3 requirement | Effect if left |
| --- | --- | --- |
| `zod: '4.6.7'` | `hardhat@3.21.2` → `zod ^3.23.8`; `hardhat-zod-utils` peer `zod ^3.23.8` | Hardhat forced onto zod 4; config validation fails. |
| `undici: '6.29.1'` | `@nomicfoundation/hardhat-utils@5` → `undici ^7.29.0` | Forced downgrade of HH3's HTTP client. |
| `ws@>=8.0.0 <8.21.0: '8.22.0'`, `chokidar@4.0.3` trust exclusion | `ws ^8.21.1`, `chokidar ^5` | Compatible. |

Fix (applied in Phase 0): `undici@<6.29.1: '6.29.1'` and `zod@<3.23.8: '4.6.7'`. These keep every current consumer on its current version and only free the HH3 ranges; `^6`/`^4` selectors were tested and rejected because they move `@reown/appkit-wallet` (zod 3.22.4) and Hardhat 2's plugins (undici ^5) off today's versions.

---

## 3. What the OpenZeppelin 4.9.6 constraint does and does not affect

- **Implementations, storage layouts, `src/`:** untouched. `@openzeppelin/contracts{,-upgradeable}` stay pinned at 4.9.6, with the existing `pnpm-workspace.yaml` overrides forcing `<4.9.6 → 4.9.6`.
- **`@openzeppelin/upgrades-core` validations** (`.openzeppelin/*.json` network files, `deploy/V1/*/validations.json` used by `forceImport`): independent of the contracts version. Plugin 4.1.0 pulls upgrades-core 1.48, same manifest format.
- **Beacons:** same story as proxies: plugin v3+ `deployBeacon` emits an OZ 5.0 `UpgradeableBeacon`; production and tests deploy the pinned OZ v4.4.1 `UpgradeableBeacon` (`deployments/bytecode/mainnet-proxy/UpgradeableBeacon.json`) instead (Phase 4).
- **Proxy bytecode for new upgradeable deployments:** plugin v3+ `deployProxy` emits OZ **5.0** `TransparentUpgradeableProxy` with a fresh `ProxyAdmin` per proxy; `deployProxyAdmin` removed; `admin.changeProxyAdmin` works only on 4.x proxies. Production today (plugin 2.5.1) deploys OZ 4.x (v4.8.3) proxies with a shared admin, matching `deployments/bytecode/mainnet-proxy`. **With Ignition this stops being a decision:** production modules deploy the pinned OZ 4.x proxy and ProxyAdmin artifacts via `m.contract(name, artifact, args)`, so the fleet stays homogeneous, `changeProxyAdmin`/`transferProxyAdminOwnership` keep working, and the OZ plugin only validates (`validateImplementation` before deploy, `forceImport` after) — see §5.3. Test fixtures may use the plugin's `deployProxy` (5.0 proxies are fine for unit tests).

---

## 4. Decisions

### 4.1 Deployment framework: Hardhat Ignition (chosen) vs hardhat-deploy v2 (rocketh) vs thin in-repo runner

Today hardhat-deploy is only a **tagged script runner**: contracts are deployed via ethers factories or the OZ plugin, `deployments.save` is never called, `namedAccounts` aliases signer 0, and the `TASK_DEPLOY_RUN_DEPLOY` override only holds one browser-signer session across a `--tags` batch.

| | **Ignition** (chosen) | hardhat-deploy v2 / rocketh | Thin in-repo `deploy` task |
| --- | --- | --- | --- |
| Maintenance | Official, bundled in `hardhat-toolbox-viem`, returns viem instances (`connection.ignition.deploy`) | Single-maintainer rocketh 0.x packages; capability gaps (`skip` ignored, `hre.deployments` on tasks replaced) | Ours, ~150 lines |
| Model | Declarative modules, dependency graph, journal + resume, nonce/gas/retry handling, `ignition verify`, `ignition status`, `deployed_addresses.json` | Imperative scripts with `tags`/`dependencies`, own `deployments/` format, `--tags` CLI kept | Imperative, same scripts as today |
| OZ 4.x proxies | `m.contract("TransparentUpgradeableProxy", pinnedArtifact, [impl, admin, initData])`: exact mainnet bytecode, no plugin involved | `deployViaProxy` uses rocketh's own proxy Solidity (`SharedAdminOpenZeppelinTransparentProxy`), not the pinned bytecode | Via `deployContractFromArtifacts`, already exists |
| Storage-layout safety | Not built in → OZ `validateImplementation`/`validateUpgrade` as a pre-flight task, `forceImport` after | Not built in | OZ plugin as today |
| Signing | Through the Hardhat provider (`from` = configured account): the `eth_sendTransaction` interceptor in §5.5 serves it; Ledger via `hardhat-ledger` | Own `signerProtocols` seam; signer-ui needs a custom protocol | Reuse bridge as is |
| Env-driven inputs, address registry, chained handoff | `--parameters` JSON + `m.getParameter`; a wrapper task builds the parameters object from env + `deployments/addresses/*.json`; module composition (`m.useModule`) replaces the in-process handoff store | Fits as-is | Fits as-is |
| Cost | Redesign 27 deployment scripts as modules (not mechanical) | Rewrite 33 scripts to `deployScript()` for little gain | Lowest |
| Operator CLI | `hardhat ignition deploy ignition/modules/<X>.ts --network <n> --parameters ignition/parameters/<n>.json` (docs change) | `hardhat deploy --tags` kept | kept |

Why not rocketh: it replaces one script runner with another, brings a second proxy implementation and a second signer model, and does not give the pinned-bytecode guarantee. Why Ignition over the thin runner: journaling/resume and nonce management are real operational gains for multi-transaction mainnet deploys, verification and address export are built in, and it is the maintained path. The thin runner remains the **fallback** if the §5.5 signer PoC fails.

### 4.2 Scope of "switch to viem"

Everything that goes through Hardhat (tests, Ignition modules, tasks, `hardhat run` scripts, signer bridge, OZ plugin usage) moves to viem. `@nomicfoundation/hardhat-ethers` is removed. The `ethers` npm package stays **only** as a plain dependency of the Hardhat-free `local-deployments-artifacts/*.ts` scripts until §7 converts them; no Hardhat-side code imports it.

### 4.3 Test runner and assertions: `node:test` + `viem.assertions` + chai `expect` for values

`hardhat-toolbox-viem` bundles `hardhat-node-test-runner`. Mocha is still available (`@nomicfoundation/hardhat-mocha@4.0.1`) but keeping it means composing plugins by hand and carrying `mocha`/`@types/mocha` for no benefit: the suite uses no Mocha-specific features (0 `this.timeout`, no root hooks, no `.mocharc`). `describe`/`it`/`before*` exist in `node:test` with the same shape; only explicit imports are added (50 files). Chai is runner-agnostic; keep `chai@6` (ESM) for `expect(x).to.equal(y)` style value assertions so the ~1,180 tests do not also change their equality syntax. Contract matchers go to `viem.assertions`. Alternative: drop chai for `node:assert` later.

### 4.4 Foundry `.t.sol` tests: try `hardhat test solidity`, fall back to `forge`

As before: in Phase 3, point `paths.tests.solidity` at `test/foundry`; add `remappings.txt` (`src/=src/`) if root-relative imports fail; if it does not converge within the phase, point `paths.tests.solidity` at an empty directory and keep `forge test` (follow-up: add it to CI). `test/tmp/` is git-ignored and out of scope.

### 4.5 `.env` loading

Keep `import "dotenv/config"` as the first line of `hardhat.config.ts` (deliberate deviation from the skill) so the operator workflow (`.env.template*`, `VERIFY_CONTRACT`, `HARDHAT_SIGNER_UI`, dozens of script variables) keeps working; use `configVariable()` for the URLs/keys the config itself declares; keystore later (§7).

### 4.6 Ignition journal vs address registry

Ignition writes `ignition/deployments/<id>/{journal.jsonl,deployed_addresses.json,artifacts}`. The repo already has a PR-reviewed registry in `deployments/addresses/*.json`. Proposal: commit Ignition deployments for stable networks (`chain-1`, `chain-11155111`, `chain-560048`, `chain-59144`, `chain-59141`) so resumes and `ignition status` work, keep `deployments/addresses` as the human-reviewed source of truth, and add a check (extend `validateAddressRegistry.ts`) that every registry address appears in the matching `deployed_addresses.json` or is annotated as pre-Ignition. Local/dev chains (`default`, `zkevm_dev`, `custom`, `l2`) are git-ignored.

---

## 5. Target design

### 5.1 `hardhat.config.ts` (sketch)

```ts
import "dotenv/config"; // §4.5
import hardhatToolboxViem from "@nomicfoundation/hardhat-toolbox-viem";
import hardhatUpgrades from "@openzeppelin/hardhat-upgrades/viem";
import hardhatIgnoreWarnings from "hardhat-ignore-warnings"; // optional, §2.2
import { configVariable, defineConfig, task } from "hardhat/config";
import { linea, lineaSepolia, hoodi } from "viem/chains";

import { overrides } from "./hardhat_overrides.js";
import { SupportedChainIds } from "./common/supportedNetworks.js";
import { resolveDeployerAccounts } from "./scripts/hardhat/deployer-accounts.js";

export default defineConfig({
  plugins: [hardhatToolboxViem, hardhatUpgrades /*, hardhatIgnoreWarnings */],
  tasks: [
    // 11 operational tasks + the 6 former deploy scripts that are not deployments, each `setAction(() => import("./scripts/.../x.js"))`
    // + `deploy:prepare-params` (env + registry → ignition/parameters/<network>.json) and `deploy:validate-upgrade` (OZ validateImplementation/validateUpgrade)
  ],
  paths: { sources: "./src", artifacts: "./build", tests: { nodejs: "test/hardhat", solidity: "test/foundry" /* §4.4 */ } },
  solidity: {
    compilers: [{ version: "0.8.33", settings: { viaIR: useViaIR, optimizer: { enabled: true, runs: 10_000 }, evmVersion: "osaka" } }],
    overrides, // keys stay "src/…"
  },
  networks: {
    default: { type: "edr-simulated", chainType: "l1", hardfork: "osaka", allowUnlimitedContractSize: true },
    mainnet: { type: "http", chainType: "l1", url: configVariable("MAINNET_RPC_URL"), accounts: resolveDeployerAccounts() },
    sepolia: { type: "http", chainType: "l1", url: configVariable("SEPOLIA_RPC_URL"), accounts: resolveDeployerAccounts() },
    hoodi: { type: "http", chainType: "l1", chainId: SupportedChainIds.HOODI, url: configVariable("HOODI_RPC_URL"), accounts: resolveDeployerAccounts() },
    linea_mainnet: { type: "http", chainId: 59144, url: configVariable("LINEA_MAINNET_RPC_URL"), accounts: resolveDeployerAccounts() },
    linea_sepolia: { type: "http", chainId: SupportedChainIds.LINEA_SEPOLIA, url: configVariable("LINEA_SEPOLIA_RPC_URL"), accounts: resolveDeployerAccounts() },
    custom: { type: "http", url: configVariable("CUSTOM_RPC_URL"), accounts: resolveDeployerAccounts() },
    zkevm_dev: { type: "http", url: configVariable("L1_RPC_URL"), gasPrice: 1322222229, timeout: BLOCKCHAIN_TIMEOUT, accounts: resolveDeployerAccounts(), ...(chainId) },
    l2: { type: "http", url: configVariable("L2_RPC_URL"), accounts: resolveDeployerAccounts() }, // allowUnlimitedContractSize is edr-only
  },
  chainDescriptors: {
    59144: { name: "linea_mainnet", viemChain: linea, blockExplorers: { etherscan: { name: "lineascan", url: "https://lineascan.build/", apiUrl: "https://api.etherscan.io/v2/api?chainid=59144" } } },
    59141: { name: "linea_sepolia", viemChain: lineaSepolia, blockExplorers: { etherscan: { /* … */ } } },
    560048: { name: "hoodi", viemChain: hoodi, blockExplorers: { etherscan: { /* … */ } } },
  },
  verify: { etherscan: { apiKey: configVariable("ETHERSCAN_API_KEY") } },
  ignition: { requiredConfirmations: 1 /* per-network overrides for mainnet */ },
  test: { nodejs: { /* defaults */ }, solidity: { fuzz: { runs: 4 }, invariant: { runs: 4, depth: 4 } } },
});
```

Notes:
- `url: "https://…/" + process.env.INFURA_API_KEY` cannot be expressed with `configVariable`; introduce per-network `*_RPC_URL` variables (documented in `.env.template` with the Infura form). HH3 rejects empty URL strings at load time, so `custom`/`l2` must use `configVariable`.
- `viemChain` descriptors make `getWalletClients()` return clients bound to the right viem chain, which matters for Linea fee estimation (`viem/linea` `estimateGas` uses `linea_estimateGas`; this replaces `scripts/utils.ts` `LineaEstimateGasClient` and most of `common/helpers/feeOverrides.ts`).
- `resolveDeployerAccounts()` returns `[]` with no key; confirm HH3 accepts an empty `accounts` array for http networks (else use `"remote"` when empty, which is also what the signer-UI mode wants, §5.5).
- Removed sections: `gasReporter`, `mocha`, `etherscan`, `docgen`, `namedAccounts`.

### 5.2 Connection access pattern (shared by tests, tasks, scripts)

```ts
import hre from "hardhat";
const { viem, networkHelpers, networkName, provider } = await hre.network.getOrCreate(); // inside functions/hooks, never module top level
const publicClient = await viem.getPublicClient();
const [deployer, securityCouncil, operator] = await viem.getWalletClients();
const rollup = await viem.deployContract("TestLinethRollup", [init], { client: { wallet: deployer }, libraries: { Mimc: mimc.address } });
await rollup.write.submitBlobs([...], { account: operator.account });
await viem.assertions.revertWithCustomErrorWithArgs(rollup.write.pause([type], { account: nonAuthorized.account }), rollup, "AccessControlUnauthorizedAccount", [nonAuthorized.account.address, role]);
```

### 5.3 Deployments with Ignition

Layout:

```
contracts/ignition/
  modules/
    PlonkVerifier.ts             // Mimc library (reuse via parameter or m.library) + verifier with linking
    Timelock.ts
    LinethRollup.ts              // impl → pinned ProxyAdmin (shared, from parameters or m.contract(pinnedAdminArtifact)) → pinned TransparentUpgradeableProxy(impl, admin, initData) → m.contractAt("LinethRollup", proxy)
    Validium.ts, L2MessageService.ts, TokenBridge.ts (+ BridgedToken beacon), CustomBridgedToken.ts, RecoverFunds.ts,
    CallForwardingProxy.ts, LineaSequencerUptimeFeed.ts, UpgradeablePredeploys.ts (14/15/17), RollupRevenueVault.ts,
    L1LineaTokenBurner.ts, V3DexSwapAdapter.ts, AddressFilter.ts, YieldManager.ts, YieldManagerArtifacts.ts (pinned 2026-01-14 bytecode),
    YieldManagerUpgrade.ts, ForcedTransactionGateway.ts, PaymentSplitterWrapper.ts, SafeExecutionConditions.ts, TestERC20.ts
    chains/L1Protocol.ts         // m.useModule(PlonkVerifier) → LinethRollup → Timelock: replaces `--tags PlonkVerifier,LinethRollup,Timelock` and the handoff store
  artifacts/                     // thin re-exports of deployments/bytecode/**/*.json as Ignition artifacts
  parameters/                    // generated per network by `deploy:prepare-params`, git-ignored except committed templates
  deployments/                   // Ignition journal, §4.6
```

Rules:
- **Upgradeable contracts** use the pinned OZ 4.x `TransparentUpgradeableProxy` + `ProxyAdmin` artifacts (`m.contract("TransparentUpgradeableProxy", pinnedProxyArtifact, [impl, proxyAdmin, m.encodeFunctionCall(impl, "initialize", [...])])`). The existing shared ProxyAdmin address comes from parameters (registry) on stable networks or is deployed once per module graph on fresh chains. No OZ 5.0 proxies in production.
- **Storage safety** is a separate pre-flight task: `deploy:validate-upgrade --module LinethRollup` runs OZ `validateImplementation("LinethRollup", { unsafeAllow })` (new deploy) or `validateUpgrade(proxy, "LinethRollup", { kind: "transparent" })` (upgrade) via `upgrades(hre, connection)` from `@openzeppelin/hardhat-upgrades/viem`, then `forceImport` after the Ignition run so `.openzeppelin/<network>.json` stays in sync. The `deploy/V1/*` manifests feed `forceImport` as today.
- **Pinned-bytecode deploys** (`21_deploy_YieldManagerArtifacts.ts`, `04_deploy_L2MessageServiceV1.ts`) use `m.contract(name, artifact, args)`; `auditedDeployVerifier` logic becomes a post-deploy check task comparing `deployed` bytecode to the pinned artifact.
- **Library linking** (`Mimc` for PlonkVerifier / ForcedTransactionGateway): `m.library("Mimc")` or `m.contractAt("Mimc", param)` + `libraries: { Mimc }`.
- **Parameters** (`m.getParameter("l1SecurityCouncil")`, etc.) are produced by `deploy:prepare-params --network <n> --module <M>` from the same inputs as today (`getRequiredEnvVar`, `requireAddressFromRegistryOrEnv`, `generateRoleAssignments`, pause/unpause role defaults), keeping the env-var names and registry precedence rules documented in `docs/deployment/README.md`. Chained `--tags` become composite modules (`chains/*.ts`), so `common/helpers/deploymentHandoff.ts` is deleted.
- **Verification**: `hardhat ignition deploy … --verify` (or `ignition verify <deploymentId>`), replacing `tryVerifyContract*`; `VERIFY_CONTRACT=true` maps to the `--verify` flag in the wrapper docs.
- **Not deployments → tasks** (6 former deploy scripts): `03/04/06/18_*WithReinitialization` (deploy implementation via OZ `deployImplementation` + print `upgradeAndCall` calldata for the Security Council) and `13/16_deploy_EIP{2935,4788}SystemContract` (broadcast a presigned raw transaction: `publicClient.sendRawTransaction`). They become `hardhat <taskName> --network <n>`.
- **Operational scripts** (`scripts/upgrades/*`, `changeProxyAdmin.ts`, `transferProxyAdminOwnership.ts`, `scripts/tokenBridge/test/*`, `scripts/testEIP*`): stay `hardhat run` scripts on the viem OZ API (`upgradeProxy(proxy, "Name", { kind })`, `admin.changeProxyAdmin(proxy, newAdmin, walletClient)`, `admin.transferProxyAdminOwnership(proxy, owner, walletClient)`).

Operator CLI before/after:

```
pnpm exec hardhat deploy --network sepolia --tags PlonkVerifier,LinethRollup,Timelock
→
pnpm exec hardhat deploy:prepare-params --network sepolia --module L1Protocol   # env + registry → ignition/parameters/sepolia.json
pnpm exec hardhat deploy:validate-upgrade --network sepolia --module L1Protocol
pnpm exec hardhat ignition deploy ignition/modules/chains/L1Protocol.ts --network sepolia --parameters ignition/parameters/sepolia.json --verify
```

A small `deploy` wrapper task can chain the three for parity with today's one-liner.

### 5.4 Tests

- Fixtures (`test/hardhat/common/**`, `rollup/helpers/**`, `yield/helpers/**`, `operational/helpers/**`, `bridging/token/helpers/**`) acquire `viem`/`networkHelpers` inside the fixture function via `hre.network.getOrCreate()` and return viem contract instances plus wallet clients per role (`admin`, `securityCouncil`, `operator`, …). `networkHelpers.loadFixture` keeps snapshot semantics.
- `test/hardhat/common/deployment.ts`: `deployUpgradableFromFactory("Name", args, opts)` → `upgradesApi.deployProxy("Name", args, { initializer, unsafeAllow, constructorArgs, client })` (viem OZ API, contract by name); `reinitializeUpgradeableProxy` via `erc1967.getAdminAddress` + `viem.getContractAt("ProxyAdmin", admin)`.
- `expectations.ts` helpers implemented with `viem.assertions` (~670 call sites keep their `(contract, call, name, args)` shape); see the conventions doc.
- Blob tests: `setupKzg` from `viem` over `c-kzg` + `test/hardhat/_testData/trusted_setup.txt` (replaces `kzg-setup.ts`), `walletClient.sendTransaction({ blobs, kzg, maxFeePerBlobGas })`; `getWalletForIndex` → `mnemonicToAccount` on the default HD mnemonic.
- Raw `provider.send("hardhat_*")` (83 sites) → `testClient.setBalance/impersonateAccount/setCoinbase/setStorageAt/mine` or `networkHelpers`.
- `node:test`: files run in separate processes by default, so the per-process `c-kzg` trusted setup is fine; `describe.skip` manual tests (`_manualTesting/*`) stay skipped.
- `Makefile` `pnpm test -- --parallel`: flag no longer exists; `node:test` parallelism is the default. Drop the flag.

### 5.5 Signer-UI bridge on viem

Today the bridge returns an ethers `AbstractSigner` whose `sendTransaction` forwards to the browser wallet and resolves with the hash. With viem and Ignition, the cleanest single integration point is the **Hardhat provider**: register a `network` hook handler (`hre.hooks.registerHandlers("network", { onRequest })`) when `HARDHAT_SIGNER_UI=true` that intercepts `eth_sendTransaction` (route to the browser, return the hash), `eth_accounts`/`eth_requestAccounts` (return the connected wallet address), and `eth_signTypedData_v4` if ever needed, passing everything else to `next`. Then wallet clients created by `viem.getWalletClient(address)` with `accounts: "remote"`, the OZ viem API (`client` option) and Ignition (`from`/`--default-sender`) all sign through the UI with **no per-call-site code**. The session lifecycle (`withSignerUiSession`, batch mode, Next.js start/stop, `setUiTransactionContext` labels) is kept; the `DeployFunction` wrapper becomes a wrapper around the `ignition deploy` task.

Phase 0 spike result: the interceptor pattern works with Ignition, including wallets that pick their own nonce or change fees. The bridge must additionally log every `(sender, nonce, hash)` it broadcasts and back a `deploy:recover` step, because a wallet rejection or Ctrl-C leaves Ignition with an unsent `TRANSACTION_PREPARE_SEND` that it will not resume past on its own (see Phase 0 results). Still to do by hand before Phase 2 closes: one real MetaMask run on Sepolia through the existing Next.js `signer-ui`, since the spike's stand-in signer is a local key.

### 5.6 `tsconfig.json`, scripts, Makefile

- `contracts/tsconfig.json`: keep `extends`, add `verbatimModuleSyntax: true`, `types: ["node"]`, `target`/`lib` es2023, explicit `include`/`exclude` (include `hardhat.config.ts`, `common`, `ignition`, `scripts`, `test`, `local-deployments-artifacts`, `build/artifacts.d.ts`; exclude `signer-ui`, `integrity-verifier`, `build/**` except types, `cache`, `out`, `lib`, `docs`). Override `exactOptionalPropertyTypes: false` locally only if HH3/viem types force it.
- `package.json` scripts: `build` → `hardhat build`; `test` → `pnpm run autoupdate && hardhat test`; `test:reportgas` → `hardhat test --gas-stats`; `coverage` → `pnpm run autoupdate && hardhat test --coverage` (check for `SOLIDITY_COVERAGE` reads first); `validate:*` → `tsx`; delete `balance`; `solidity:docgen` → `tsx scripts/docgen.ts` (§7); `clean` adds `coverage`, `ignition/deployments/chain-31337` etc. New: `deploy:prepare-params`, `deploy:validate-upgrade` are Hardhat tasks, not npm scripts.
- `makefile-contracts.mk`: `ts-node` → `tsx` (18 targets); `force-compile` → `hardhat build --force`.
- Dependencies removed: `hardhat@2`, `@nomicfoundation/hardhat-toolbox`, `-ethers`, `-network-helpers@1`, `-verify@2`, `-foundry`, `@typechain/*`, `typechain`, `hardhat-deploy`, `hardhat-gas-reporter`, `hardhat-storage-layout`, `hardhat-tracer`, `solidity-coverage`, `chai@4`. Added: `hardhat@3.21.2`, `@nomicfoundation/hardhat-toolbox-viem@5` + all peers (`hardhat-viem`, `hardhat-viem-assertions`, `hardhat-node-test-runner`, `hardhat-network-helpers@3`, `hardhat-verify@3`, `hardhat-keystore`, `hardhat-ignition`, `hardhat-ignition-viem`, `ignition-core`), `viem` (catalog), `@openzeppelin/hardhat-upgrades@4.1`, `tsx` (catalog), `chai@6`, `@types/chai`, optionally `hardhat-ignore-warnings@0.3`. `ethers` stays until §7 (used only by `local-deployments-artifacts`). Catalog: keep `typechain` and `@typechain/ethers-v6`; `ts-libs/sdk/sdk-ethers` still uses them. Also removed with `solidity-coverage`: the `sc-istanbul@0.4.6` patch, its `patchedDependencies` entry and the `sc-istanbul>js-yaml` override (pnpm fails on unused patches).

### 5.7 CI (`.github/workflows/run-smc-tests.yml`)

| Step | Change |
| --- | --- |
| "Install Foundry" | Keep only if §4.4 ends with `forge test` in CI; the `hardhat-foundry` comment goes away. |
| Compiler cache `~/.cache/hardhat-nodejs` keyed on `hardhat.config.ts` + `hardhat_overrides.ts` | Verify the HH3 cache dir on Linux; keys unchanged. |
| `pnpm -F contracts run build` | unchanged command (`hardhat build` inside). |
| coverage → `coverage/coverage-final.json` | `coverage/lcov.info` in the artifact upload, `codecov-action` `files:` and `codecov-external-pr.yml` glob. Mirror `.solcover.js` `skipFiles` (`_testing`, `proxies`, `CustomBridgedToken.sol`, `_exampleImplementations`, `SparseMerkleProof.sol`) as `codecov.yml` `ignore:` globs. |
| `--coverage` side effects | HH3 forces unlimited contract size and disables gas caps under coverage; suite already runs unlimited. |

---

## 6. Execution plan (skill phases, adapted)

Skill conventions apply: no VCS commands by the agent (checkpoint commits suggested, engineer applies), pnpm for every install, strictly sequential phases with a user checkpoint, per-file filtered output.

### Phase 0 — Prerequisites and spike (before the migration branch)

1. PR: narrow `zod`/`undici` overrides in `pnpm-workspace.yaml` (§2.5); `pnpm install`; full workspace tests.
2. Spike (throwaway branch, 2–3 days): HH3 + `hardhat-toolbox-viem` + Ignition on a scratch module deploying the pinned OZ 4.x proxy artifacts; the §5.5 `onRequest` interceptor driving the existing `signer-ui` Next.js app on `default` and Sepolia. Exit criteria: one UI-signed multi-transaction Ignition deploy completes and resumes after a simulated interruption. Decide Ignition vs thin-runner fallback for UI-signed deploys.
3. Record §4 decisions; create the migration branch.

#### Phase 0 results (2026-11-09)

**Override fix: done, uncommitted on the working tree.** `pnpm-workspace.yaml` now uses `undici@<6.29.1: '6.29.1'` and `zod@<3.23.8: '4.6.7'` instead of the blanket `undici`/`zod` overrides. A scratch-workspace test showed these range selectors keep every legacy consumer on today's version (Hardhat 2 plugins declaring `undici ^5.14.0` still get 6.29.1; `@reown/appkit-wallet`'s exact `zod 3.22.4` still gets 4.6.7) while Hardhat 3's `zod ^3.23.8` and `undici ^7.29.0` resolve freely. `^4`/`^6` selectors were rejected because they silently moved those legacy consumers. Repo `pnpm install` (pnpm 12.6.0) changed **no resolved version**: the lockfile diff is the two override keys plus metadata (catalog entry recorded, `catalog:` specifiers and real peer ranges restored, two upstream `@smithy/core` deprecation notes). `pnpm install --frozen-lockfile` passes the supply-chain policy. Checks: postman, automation-service, lido-governance-monitor and verifier-ui build; automation-service 588/588 and lido-governance-monitor 229/229 tests pass; verifier-ui tests pass; `e2e` typechecks. postman's Jest suite panics inside the prover's Go `libcompressor` native library (environmental, unrelated: resolutions are identical to `main`).

**Spike: Ignition confirmed for all deploys, including UI-signed ones.** Throwaway workspace (not in the repo) replicating the repo's pnpm policy, with `hardhat@3.21.2`, `hardhat-toolbox-viem@5.0.7`, Ignition 3.1.x, `@openzeppelin/hardhat-upgrades@4.1.0`, OZ contracts 4.9.6, solc 0.8.33 / `osaka`.

| Check | Result |
| --- | --- |
| HH3 installs under the repo's `trustPolicy`/`minimumReleaseAge` with the narrowed overrides | Yes: HH3 gets `zod 3.25.76`, `undici 7.30.2`. Blocked only by the Defender items below. |
| OZ 4.9.6 upgradeable contracts compile on HH3 (0.8.33, `osaka`) | Yes (upstream `memory-safe-assembly` deprecation warning, same as today). |
| Ignition deploys the pinned `deployments/bytecode/mainnet-proxy` ProxyAdmin + TransparentUpgradeableProxy via `m.contract(name, artifact, args)` | Yes; on-chain runtime bytecode is **byte-identical** to the pinned artifacts. |
| OZ Upgrades v4 (viem API) on a proxy it did not deploy | `erc1967.getAdminAddress`, `validateImplementation`, `forceImport`, `validateUpgrade` work; a slot-inserting V3 is rejected ("New storage layout is incompatible"); a V2 without an initializer is rejected (OZ error 001), so modules must keep the repo's `initialize` + `reinitializeVN` pattern. |
| Upgrade via pinned `ProxyAdmin.upgradeAndCall(proxy, impl, reinitializeV2)` in an Ignition module | Yes, reinitializer runs, state survives. |
| `viem.assertions.revertWith` / `emitWithArgs` with `node:test` | Yes. |
| `eth_sendTransaction` interceptor (project-local plugin, `network.onRequest`) signing with a key Hardhat does not know | Ignition deploys through it in three wallet behaviours: honours Ignition's nonce/fees, picks its own pending nonce, bumps fees 25%. |
| Persisted journal on a node: second operator run chains `BoxUpgrade` (`m.useModule(BoxProxy)`) onto the first run's deployment | Yes, nothing redeployed; this replaces `--tags` chaining + the handoff store. |
| **Wallet rejects tx #2 (EIP-1193 4001), or Ctrl-C at the wallet prompt** | **Ignition gets stuck**: it journals `TRANSACTION_PREPARE_SEND` before `eth_sendTransaction`, so on resume it fails with HHE10410 and asks for `ignition track-tx <hash>`, but nothing was broadcast. Only built-in escape is `--reset` (redeploy everything at new addresses). |
| Guarded recovery: drop the trailing unsent `TRANSACTION_PREPARE_SEND` when the sender's pending nonce still equals it | Resume then completes and later modules chain normally. |
| Same, after a foreign tx from the same wallet consumed the nonce | Once the unsent entry is dropped, Ignition resumes at the next free nonce; it handles confirmed foreign txs itself. |

Consequences folded into the plan:
- **Bridge must own recovery (§5.5).** The bridge is the only broadcaster in UI mode, so it logs every `(sender, nonce, hash)` it broadcasts. A `deploy:recover` task (or automatic pre-resume step in the deploy wrapper) handles a trailing `TRANSACTION_PREPARE_SEND`: nonce in the bridge log → `ignition track-tx <hash>`; otherwise drop the entry. The spike's `scripts/recover-unsent.ts` is the starting point; it currently refuses when the nonce was consumed elsewhere, which the bridge log makes unnecessary. Private-key and Ledger deploys need the same recovery for a Ctrl-C between prepare and send.
- **Thin-runner fallback (§4.1) is not needed.**
- **Defender SDK trust flags (security review needed before Phase 1).** `@openzeppelin/hardhat-upgrades@4.1.0` hard-depends on `@openzeppelin/defender-sdk-{base,deploy,network}-client@2.7.1`, the latest releases, which are the first published **without npm provenance** (2.5.0–2.7.0 had it). The repo's `trustPolicy: no-downgrade` rejects all three. **Decision (2026-11-09): add the three exact versions to `trustPolicyExclude`**, matching how the repo already excludes the Defender 1.54.6 packages; pinning to 2.7.0 was rejected because it sits outside `deploy-client`'s declared `^2.7.1`. Applied in `pnpm-workspace.yaml`. Verified with a fresh resolution under the repo's exact policy: OZ Upgrades 4.1.0 resolves with the entries and is rejected without them. The repo lockfile is unaffected until Phase 1 installs the plugin, and `pnpm install --frozen-lockfile` still passes the supply-chain check.
- **Hardhat `node:test` reporter crash.** `@nomicfoundation/hardhat-node-test-reporter@3.1.1` throws `Cannot read properties of undefined (reading 'replace')` when formatting OZ Upgrades validation errors, hiding the real failure. Budget a workaround in Phase 5 (wrap OZ calls in a helper that rethrows a plain `Error` with the message) and report upstream.
- **Typed pinned ABIs.** Pinned artifacts loaded from JSON give viem `unknown` return types. Generate `as const` ABI modules for `deployments/bytecode/**` (or a typed loader) in Phase 4.
- **In-process `ignition.deploy` calls do not share a journal** on the simulated network. Tests that need a chained deployment deploy the composite module once; fixtures should mostly use `viem.deployContract` / the OZ plugin, not Ignition.
- **Peer-range warnings** (`ws` for `isows`, `zod` for `abitype`) come from the existing overrides rewriting peer ranges; harmless.

Spike location (throwaway, outside the repo): the session scratchpad `hh3-spike/` directory. Port the interceptor, pinned-artifact helper, module pattern and recovery script into `contracts/` in Phases 2 and 4.

### Phase 1 — Project setup (`env`, `tsconfig`, `package`)

`.env` exists (skip template copy); `tsconfig.json` per §5.6; `package.json` per §5.6 (ESM flag, dependency swap, scripts, `ts-node` → `tsx`); catalog cleanup; `makefile-contracts.mk`.

Gate: `pnpm install` clean under `trustPolicy`/`minimumReleaseAge`; `pnpm -F contracts exec hardhat --version` prints `3.x`.
Checkpoint: `[CHECK] HH3 project setup: env, tsconfig, package.json`

#### Phase 1 results (2026-11-09)

Gate passed: `pnpm install` clean under the supply-chain policy; `hardhat --version` prints `3.21.2`; the HH3 + viem + Ignition + OZ Upgrades 4.1.0 packages, `viem` 2.57.11, `chai` 6.3.0 and `tsx` are present in `contracts/node_modules`; `hardhat@3.21.2` is the only Hardhat version in the lockfile.

Changed: `contracts/package.json` (ESM, dependency swap, scripts), `contracts/tsconfig.json`, `contracts/Makefile` (`hardhat build --force`, `--parallel` dropped), `contracts/makefile-contracts.mk` (17 targets `ts-node` → `tsx`), `pnpm-workspace.yaml` (sc-istanbul patch + override removed), `patches/sc-istanbul@0.4.6.patch` (deleted), `pnpm-lock.yaml`.

Notes carried forward:
- `coverage` keeps `SOLIDITY_COVERAGE=true`: three `SparseMerkleProof` tests skip themselves under coverage for gas reasons; HH3 coverage lifts gas caps, so Phase 5 can decide whether to drop the skip.
- `solidity:docgen` still calls `hardhat docgen`, which no longer exists; replaced by the §7 docgen script. `solidity-docgen` stays installed (its plain `docgen()` export loads without Hardhat 2; only an unmet `hardhat ^2.8` peer warning remains).
- `immer@11.1.21` (from `ignition-core`) now also satisfies `zustand`'s optional peer in `signer-ui` and `verifier-ui`; neither imports `immer`, `signer-ui` typechecks and `verifier-ui` builds.
- `tsconfig.json` sets `types: ["node"]`, so the Mocha globals in the existing tests stop resolving until Phase 5 moves them to `node:test`; `tsc`/`lint:ts` are expected to fail until Phases 4–5, as the skill notes.
- `.solcover.js` (CommonJS) is now dead and parses as ESM; removed in Phase 4 with the other CJS files.

### Phase 2 — Config (`config`)

`hardhat.config.ts` per §5.1; `hardhat_overrides.ts` ESM; 11 operational tasks to builder + lazy `setAction`; `deploy:prepare-params`, `deploy:validate-upgrade`, and the 6 non-deployment tasks registered (bodies land in Phase 4); delete `solcWarningFilter.ts` (or add `warnings` config), `deployments.json`, `namedAccounts`, the `TASK_DEPLOY_RUN_DEPLOY` override.

Gate: `npx tsc --noEmit` has no error in `hardhat.config.ts`; `npx hardhat --help` lists `ignition`, the tasks; `npx hardhat build` compiles `src/` with per-file overrides intact (spot-check `LinethRollup` optimizer runs 200 in metadata) and emits `build/artifacts.d.ts` for viem typing.
Checkpoint: `[CHECK] HH3 config: declarative defineConfig (solidity, networks, plugins, tasks, verify)`

#### Phase 2 results (2026-11-09)

Gate passed: `tsc --noEmit` ran (no help banner, no compiler-option diagnostics) and reports no error in `hardhat.config.ts`, `scripts/hardhat/tasks.ts` or any config dependency (3,006 errors remain in source/test files for Phases 4–5); `hardhat --help` loads the config and lists all 11 operational tasks with unchanged `--kebab-case` options; `hardhat build --no-tests --force` compiles all 198 `src/` files (18 s) with every `hardhat_overrides.ts` optimizer override applied (verified per root file from build-info: 2×1, 2×200, 1×800, 193×10,000 runs, all `osaka`).

Changes:
- `hardhat.config.ts` rewritten on `defineConfig` (§5.1): `hardhat-toolbox-viem`, OZ Upgrades, `hardhat-ignore-warnings`; `default` EDR network (`osaka`, unlimited size); HTTP networks keep the `INFURA_API_KEY` contract via `configVariable("INFURA_API_KEY", "https://<net>.infura.io/v3/{variable}")` (lazy); `custom`/`l2` URLs via `configVariable` (HH3 rejects empty URLs at load); `verify.etherscan.apiKey` via `configVariable`.
- 11 operational tasks: builder definitions in `scripts/hardhat/tasks.ts`, actions are lazily imported default exports in the original files (bodies untouched until Phase 4).
- `scripts/hardhat/deployer-accounts.ts`: `.js` import extension (only config dependency that needed one).
- `hardhat-ignore-warnings@0.3.0` added (package handoff) to replace the deleted `solcWarningFilter.ts` subtask; proven equivalent: without its rule the three `SafeExecutionConditions` 2018 warnings reappear, with it they are gone.
- Deleted: `hardhat.config.old.ts` (after the gate), `deployments.json` (unused; nothing reads it).

Behaviour notes:
- `etherscan.customChains` dropped: HH3's built-in chain descriptors for Hoodi, Linea Sepolia and Linea carry the same explorer URLs and the Etherscan v2 API. Blockscout and Sourcify verification are disabled so `hardhat verify` stays Etherscan-only, as before.
- Removed config: `namedAccounts`, `gasReporter`, `mocha.timeout` (node:test has no default timeout), `docgen`, hardhat-deploy's `deploy:runDeploy` override, `allowUnlimitedContractSize` on the `l2` HTTP network (not an HTTP option; it had no effect on a remote node).
- HH3 source names are `project/src/...`, so solc metadata hashes, and therefore the trailing metadata bytes of fresh bytecode, differ from Hardhat 2 builds. Check the integrity verifier and any bytecode comparisons against this in Phase 6.
- Deviation from the plan: `deploy:prepare-params`, `deploy:validate-upgrade`, `deploy:recover` and the 6 non-deployment tasks are registered in Phase 4 together with their bodies, so the config never points at missing modules.
- Phase 4 item found: `common/helpers/readAddress.ts#getDeployedContractOnNetwork` (used by `setMessageServiceOnTokenBridge`) reads hardhat-deploy `deployments/<network>/<Contract>.json` files; replace with the address registry / Ignition `deployed_addresses.json`.

### Phase 3 — Solidity tests (`solidity-test`)

Per §4.4. Gate: `npx hardhat test solidity` passes or is explicitly skipped.
Checkpoint: `[CHECK] HH3 solidity test migration`

#### Phase 3 findings (2026-11-09)

- Scope: `test/foundry/ExampleFoundryTest.t.sol` (empty placeholder) and `test/foundry/LinethRollup.t.sol` (5 tests); cheatcodes used are `prank`, `startPrank`/`stopPrank`, `expectRevert`, all supported. `test/tmp/*.t.sol` is git-ignored scratch, outside `paths.tests.solidity`. No `src/_testing` contract is a false-positive suite (they live under `paths.sources`).
- `forge-std/Test.sol` and the OpenZeppelin imports resolve under Hardhat 3 with no remapping. The 6 root-relative `src/...` imports in `LinethRollup.t.sol` were converted to relative `../../src/...` imports (resolved by both Hardhat and forge).
- `hardhat test solidity test/foundry/ExampleFoundryTest.t.sol`: compiles on `osaka`, 1 passing. The runner works.
- **The Foundry suite was already broken before the migration** and is not in CI:
  - `forge build` fails on `main`: `foundry.toml` still forces `L2MessageService` onto `london`, but `L2MessageServiceV1` now uses transient storage (needs `cancun`). Forge-only; the Hardhat config compiles everything on `osaka`.
  - `LinethRollup.t.sol` is stale since the data-availability modularization (#1743): it calls `_calculateY` (moved to `CalldataBlobAcceptor`) and uses `ILinethRollup.CompressedCalldataSubmission` (no longer exists). Repairing it is a test rewrite against the modular DA API, not a migration fix.
  - Because Hardhat compiles every `.t.sol` under `paths.tests.solidity`, this file fails `hardhat test` as a whole (Solidity and node runners), so it must be removed, rewritten or moved out of the test path before Phase 5.

#### Phase 3 results (2026-11-09)

- **Decision: `test/foundry/LinethRollup.t.sol` deleted** (user decision). It compiled under neither tool; its scenarios (verifier set/unset permissions, calldata submission) are covered by the TypeScript `LinethRollup` tests; git keeps the history. Nothing else referenced it.
- Gate passed: `hardhat test solidity` exits 0 (`ExampleFoundryTest.t.sol`, 1 passing). Regression: `hardhat build --force` compiles 199 files (198 contracts + 1 test); `hardhat.config.ts` and its dependencies still typecheck.
- §4.4 resolved: `paths.tests.solidity` stays `./test/foundry`, run by Hardhat 3; no `remappings.txt` needed.
- Follow-ups (not migration work): fix or retire `foundry.toml`'s `london` compilation restriction for `L2MessageService`/`TokenBridge` (forge cannot build `L2MessageServiceV1` with transient storage); write a Solidity test for the modular data-availability API if Solidity-level coverage of `LinethRollup` is wanted; add `hardhat test solidity` to CI once there is a real suite (it already runs as part of `hardhat test`).

### Phase 4 — Source files (`esm`, viem rewrite of non-test code) — fan out

Dependency-first order:
1. `common/**`: ESM (`.js`, JSON attributes, `import.meta.dirname`), `contracts/…` self-imports → relative, ethers utils → viem utils (`encodeAbiParameters`, `keccak256`, `getAddress`, `isAddress`); delete `deploymentHandoff.ts`, `verifyContract.ts`; `feeOverrides.ts` reduced to the Linea fee path or removed in favour of `viem/linea`.
2. `scripts/hardhat/**`: `signer-ui-bridge.ts` → provider interceptor + session lifecycle (§5.5), `deployer-accounts.ts` unchanged, `utils.ts` deleted (helpers become Ignition modules / viem calls), `postCompile.ts` on `hre.artifacts`.
3. `ignition/modules/**`, `ignition/artifacts/**`, `scripts/hardhat/prepare-params.ts`, `scripts/hardhat/validate-upgrade.ts`: 27 modules + composites, mapping each `deploy/*.ts` env/registry lookups into parameters; the 6 non-deployment tasks; delete `deploy/*.ts` (keep `deploy/V1/*` manifests, move to `.openzeppelin-imports/` or `ignition/manifests/`).
4. `scripts/operational/**`, `scripts/upgrades/**`, `scripts/tokenBridge/**`, `scripts/testEIP*/**`, `scripts/deployment/*` (validator rewritten for modules/parameters), `local-deployments-artifacts/*.ts` (ESM + `tsx` only, still ethers).
5. `test/hardhat/**`: ESM + viem rewrite of helpers first (`common/**`, each `helpers/**`), then spec files in batches (behavioural failures are Phase 5).

Fan-out: batches of ~10 files, import-coupled files on one agent, `common/**` and `scripts/hardhat/**` first on a single agent; per-file `npx tsc --noEmit --pretty 2>&1 | grep <file>`.

Gate: `npx hardhat build --force` clean; `npx tsc --noEmit` zero errors; no new `any`/`@ts-ignore`; no ethers import outside `local-deployments-artifacts/`; `npx hardhat test solidity` still passes.
Checkpoint: `[CHECK] HH3 source file migration: ESM conversion, viem rewrite, Ignition modules`

#### Phase 4 progress (2026-11-09)

Foundation, done by the orchestrator:
- Mechanical ESM codemod over all 225 `.ts` files: `.js`/`/index.js` extensions, `contracts/...` self-imports made relative, 62 JSON imports converted to default imports `with { type: "json" }` (named JSON imports are not valid ESM), `import.meta.dirname`, then ESLint `consistent-type-imports` + `no-import-type-side-effects` (inline `import { type X }` still loads the module under `verbatimModuleSyntax`; this broke runtime loading until fixed) and `node:test` imports for the 53 specs (no Mocha-only features in use). tsc errors 3,006 → 565, all remaining ones API rewrites.
- Test foundation on viem: `TestAccount` + `connect()` (keeps `account.address` and `contract.connect(signer)` call shapes), revert/event wrappers with ethers-compatible arg matching (numbers≈bigints, case-insensitive hex, positional structs, any promise including deployments), OZ Upgrades viem deploy helpers, constants and shared helpers; shared data types retyped to `Hex`/`Address` with validating `hex()`/`address()` helpers. Smoke-tested at runtime.
- `common/` utilities on viem; `generateRoleAssignments` made generic so the ethers-only scripts keep working; `scripts/tokenBridge/test/*` fixtures on viem (OZ v4 proxies keep the L2 bridge counterfactual-address arithmetic). The ethers island is `local-deployments-artifacts/**` + `common/helpers/{deployments,feeOverrides,normalize-address-args}.ts`.
- `.solcover.js` deleted (dead since Phase 1); its `skipFiles` (`_testing`, `proxies`, `CustomBridgedToken.sol`, `rollup/_exampleImplementations`, `SparseMerkleProof.sol`) must become `codecov.yml` ignores in Phase 6.
- Conventions for all batches: `contracts/docs/hardhat-v3-viem-conventions.md`.

Shared decisions taken while batches run:
- `hardhat_reset` is not supported by Hardhat 3's simulated network, and `hardhat-node-test-runner` hardcodes `isolation: "none"`: all spec files share one process and one chain, as Mocha did under Hardhat 2. Resets are dropped (with a comment) only where no assertion depends on a pristine chain; files that do are escalated for a shared solution.
- `test/hardhat/common/chai-bigint.d.ts`: chai 6 compares bigints at runtime but `@types/chai` 5.x types the numeric comparers as `number | Date`; the augmentation adds bigint overloads instead of lossy `Number()` conversions.
- `@chainsafe/blst@2.2.0` declared as a contracts devDependency: `test/hardhat/yield/helpers/proof.ts` imports it directly, but it was only a transitive dependency of `@lidofinance/lsv-cli` and did not resolve from `contracts/` under pnpm's strict layout (same version as already locked).

- Fresh chains: `test/hardhat/common/helpers/connection.ts` adds `useFreshChain()` + `getConnection()` (all foundation helpers resolve their connection through it). A suite that needs a pristine chain runs on `network.create()`; safe because the runner is sequential (`concurrency: false`). Used by `messaging/l1/L1MessageService.ts`, whose merkle proof fixtures pin contract addresses to fixed deployer nonces (start nonce 2 instead of 1: Hardhat 2's OZ plugin also deployed a shared ProxyAdmin from the deployer).
- Checksums: `TestAccount.address` and deployment-helper instances are checksummed, as ethers returned them (hardhat-viem reports lowercase).
- **Beacons stay OpenZeppelin 4.x**: OZ Upgrades v4's `deployBeacon` deploys an OpenZeppelin 5 `UpgradeableBeacon` (owner checks revert with `OwnableUnauthorizedAccount` instead of `"Ownable: caller is not the owner"`). The Hardhat 2 plugin (2.5.1) deployed the OZ v4.4.1 beacon (solc 0.8.9) bundled in upgrades-core; that exact artifact is now pinned at `deployments/bytecode/mainnet-proxy/UpgradeableBeacon.json` (`pinnedUpgradeableBeaconArtifact`). The test fixture and the production Ignition module deploy it directly, and the Ignition spec asserts the deployed runtime code is byte-identical. OZ `validateImplementation(..., { kind: "beacon" })` is kept for the implementation. Token `BeaconProxy`s are created by `TokenBridge` itself (OZ 4.9.6, compiled from `src/`). The pinned proxies were checked the same way: upgrades-core's OZ 4 `TransparentUpgradeableProxy` / `ProxyAdmin` (v4.8.3) equal the mainnet artifacts byte for byte.
- `Eip1559Transaction.yParity` typed `number` (`uint8`).

Finished batches (each verified again by the orchestrator in one shared run: 534 passing, then 160/160 after the beacon fix):
| Batch | Result |
| --- | --- |
| bridging + security + governance + recovery + manual | 195 passing; the one failure (beacon owner check) fixed centrally as above; `_manualTesting` stays `describe.skip` |
| messaging | 164 passing, 1 skipped (already `it.skip` in Hardhat 2); `L1MessageService` on a fresh chain |
| operational + libraries | 170 passing (+4 `environmentLogging`); 5 `hardhat_reset` hooks dropped (no fresh-chain dependency); `RollupRevenueVault` init-log count filters to proxy-emitted logs (OZ 5 proxy's internal ProxyAdmin adds one log) |
| operational scripts + ethers island | All 11 task bodies and the `hardhat run` / standalone scripts on viem; new spec `test/hardhat/scripts/operational/operationalTasks.ts` runs the tasks end to end (10 passing); OZ admin/upgrade scripts exercised against pinned OZ 4.x proxies; EIP-7702 and EIP-4844 scripts run against a local node; every `local-deployments-artifacts` script loads under `tsx` and fails only on env validation |

| rollup + verifiers | 222 passing; the 2 `ForcedTransactionGateway` failures (JSON hex strings in `bigint` fields of `buildEip1559Transaction`) fixed centrally; blob transactions on viem (`defineKzg` over `c-kzg`, blobs/commitments/proofs unchanged); `ForcedTransaction_sample_0` stays `describe.skip` (its local fixture now passes the 9 constructor args the contract takes) |
| yield | 443 passing (1 `it.skip` as before); `@chainsafe/blst` declared |
| signer-UI bridge | Provider-level interceptor (`scripts/hardhat/signer-ui-plugin.ts`, `signer-ui-network-hooks.ts`), broadcast log, `deploy-recover.ts`, in-process verification (`verifyContract`), viem `scripts/hardhat/utils.ts`; 26 tests in `test/hardhat/scripts/*` incl. a new hook-routing spec; end-to-end smoke with the real Next.js app and a scripted fake browser (lazy session + `runSignerUiBatch`, one rejection then approval). Manual MetaMask run on Sepolia still to do |

**Shared run after integration (all batches except Ignition, 56 spec files, one process): 1,239 passing, 0 failing, 4 skipped** (all four skipped in Hardhat 2 too).

More shared fixes after the batches reported:
- (Superseded: `connect()` was later removed in favour of viem's `{ account }` call option, see below.)
- `buildEip1559Transaction` converts every JSON-RPC field to its declared type (`bigint`, `address()`, `hex()`), with a typed input instead of `any`.
- **pnpm patch `@nomicfoundation/hardhat-network-helpers@3.0.12`** (`patches/@nomicfoundation__hardhat-network-helpers@3.0.12.patch`): `loadFixture` writes a stale snapshot list back after a fixture that nests another (cached) `loadFixture`, so a later `loadFixture` fails with HHE60013. The yield and operational fixture helpers compose by nesting, as under Hardhat 2. The patch keeps the nested result and appends only the call's own snapshot (Hardhat 2 behaviour). Guarded by `test/hardhat/testInfrastructure/loadFixtureNesting.ts`, verified to fail with HHE60013 without the patch. Follow-up: report upstream to Nomic Foundation and drop the patch once fixed.

Phase 4 integration (after the Ignition batch), done:
- `deploy:recover` registered in `scripts/hardhat/tasks.ts` (`--deployment-id`, defaults to `chain-<chainId>`).
- `common/helpers/index.ts` no longer re-exports the ethers-only `deployments.ts` / `feeOverrides.ts`; the island imports them directly, so Hardhat-side barrel imports no longer load ethers.
- Signer-UI docs rewritten for the provider-level bridge (`contracts/AGENTS.md`, `signer-ui/README.md`, `docs/deployment/signer-ui-removal.md`); the 19 per-contract deployment pages now show `hardhat deploy --module …` / `deploy:*` commands. The `LinethRollupWithReinitialization` section of `l1/lineth-rollup.md` was removed: no script carried that tag before the migration either.
- Pinned OZ 4.x beacon (see Shared decisions); "OZ 4.9.6 proxy" wording corrected to OZ 4.x (v4.8.3 proxies, v4.4.1 beacon) in modules, tests and docs. `PinnedArtifact.bytecode` typed `Hex`. `solidity.npmFilesToBuild` removed (it only compiled the OZ 4.9.6 beacon).
- `test/hardhat/_foundationSmoke.ts` deleted.
- Build profiles: `hardhat ignition deploy` and `hardhat-verify` compile with the `production` profile (isolated builds), so a deploy recompiles once the first time. The "missing field `contracts`" EDR error seen once was a race between parallel batches building at the same time, not reproducible in a single build.
- Not done here: `.env.template` (~line 244) and `.env.template.uat` (~line 187) still show `hardhat deploy --network sepolia --tags LinethRollup`; replace with `--module LinethRollup` (the files are protected from agent edits).

#### Phase 4 results (2026-11-09)

| Gate | Result |
| --- | --- |
| `hardhat build --force` | Clean: 199 files, solc 0.8.33, `osaka` (only the upstream OZ `memory-safe-assembly` notices) |
| `tsc --noEmit` (whole project) | 0 errors (3,006 at the start of Phase 4) |
| `hardhat test solidity` | 1 passing |
| `hardhat test nodejs` (every spec, one process) | **1,280 passing, 0 failing, 4 skipped** (the four were skipped in Hardhat 2 too); includes the 42 Ignition specs and the `loadFixture` nesting guard |
| No `any` / `@ts-ignore` / `as unknown as` added | Holds; the two `as unknown as` in the island's own unit tests predate the migration |
| No ethers outside the island | Holds: `local-deployments-artifacts/**`, `common/helpers/{deployments,feeOverrides,normalize-address-args}.ts` and their unit tests, `scripts/utils.ts`; `integrity-verifier/verifier-ethers` is a separate package |

Assertions on `viem.assertions` (after the Phase 4 gate, on review): the `expectations.ts` helpers keep their names and call shapes (`expectRevertWithCustomError(contract, call, name, args?)`, `expectRevertWithReason`, `expectEvent`, `expectEvents`, `expectNoEvent`) but are now implemented with Hardhat's `viem.assertions` on `getConnection()` instead of hand-written revert-data and log decoding; only `expectNoEvent` and `expectEventDirectFromReceiptData` still decode receipts (no `viem.assertions` equivalent). The helpers are typed like `viem.assertions` (error/event names and args checked against the ABI), which surfaced 112 expectations the untyped helpers compared loosely (numbers for bigints, JSON strings for `Hex`, untyped arrays); all fixed at the call site, none dropped or loosened. Full suite unchanged: 1,280 passing, 0 failing, 4 skipped; a mutation check (wrong error name, wrong event argument) fails through the helpers as expected.

`connect()` removed (after the Phase 4 gate, on review against Hardhat's "Testing with viem and node:test" guide): the 890 `connect(contract, account).write/simulate.fn(...)` calls now pass the account as viem's call option, `contract.write.fn([...], { account: account.account })`, as the guide shows; an AST codemod (type checker for the args-vs-options position) did the rewrite. `TestAccount` drops its `publicClient` (3 uses moved to the wallet client's `getChainId()` / the connection's public client); `submitCalldataBeforeFinalization` and the role/pause test helpers take the acting `TestAccount` instead of a connected contract. Full suite unchanged: 1,280 passing, 0 failing, 4 skipped. Not adopted from the guide: one `network.create()` per spec file (shared fixtures resolve the connection through `getConnection()`; switching means passing the connection through every fixture) and `node:assert/strict` instead of chai (no gain over the existing chai `expect` calls).

Behaviour changes from the operational-scripts batch (review):
- `hre.deployments.get(name)` fallbacks (a dead path: nothing ever saved deployments) now look the name up in `deployments/addresses/<network>.json` via `scripts/operational/registryAddress.ts`. Verifier names have no registry key, so `setVerifierAddress` needs `--verifier-address` (or the env var) as in practice before.
- `setVerifierAddress` bug fix: the action read `taskArgs.verifierContractName` while the declared option is `verifierName`, so the CLI flag was silently ignored.
- `changeProxyAdmin.ts`, `transferProxyAdminOwnership.ts`, `scripts/upgrades/*`: now sign through the signer bridge, so `HARDHAT_SIGNER_UI=true` works for them (previously rejected). `transferProxyAdminOwnership` targets the ProxyAdmin of `PROXY_ADDRESS` read from the ERC-1967 slot (OZ Upgrades v4 needs the proxy), instead of the manifest's single admin; operator interface unchanged.
- `gasEstimation.ts` was already broken before the migration (`setRemoteTokenBridge` no longer exists); it now mirrors the `deployTokenBridges` fixture.
- Standalone scripts use viem fee estimation (`estimateFeesPerGas`, `viem/linea` `estimateGas` with the same 1.35/1.05 multipliers), so printed fee numbers differ slightly from ethers' `getFeeData`.
- `scripts/utils.ts` reduced to the ethers-only `get1559Fees` used by the island.

Batches (parallel, disjoint file ownership): tests rollup+verifiers, messaging, yield, bridging+security+governance+recovery+manual, operational+libraries; signer-UI bridge (provider interceptor, broadcast log, `deploy:recover`, in-process verification); Ignition (modules, pinned proxies, `deploy:prepare-params`, `deploy:validate-upgrade`, non-deployment tasks, docs); operational scripts + ethers island.

### Phase 5 — Test fix-up (`test`) — fan out

Run each of the 50 spec files with `npx hardhat test <file>` in batches of ~10, fix, then full re-sweep. Budget for:
- Fixture state sharing: connections acquired inside fixtures; `loadFixture` snapshot semantics with viem clients.
- Assertion wrapper parity: custom-error args matching (`anyValue` predicates from `@nomicfoundation/hardhat-toolbox-viem/predicates`), indexed `string`/`bytes` event args logged as hashes.
- Blob/KZG and forced-transaction tests (`BlobSubmission.ts`, `Finalization.ts`, `ForcedTransactionGateway.ts`, `ForcedTransaction_sample_0.ts`): viem blob sidecars vs ethers `Transaction` encoding; `trusted_setup.txt` path.
- `hardhat_reset`/fork tests and `evm_setAutomine` sequences via `testClient`.
- bigint/`0x` typing differences (viem returns `0x`-typed `Hex`, bigint everywhere) vs chai deep-equal expectations.
- `node:test` process-per-file parallelism vs any test that assumed a shared in-process EDR (there should be none; files were independent under Mocha).

Gate: `npx tsc --noEmit` clean; every spec passes individually; `npx hardhat test` passes; `npx hardhat test --coverage` writes `coverage/lcov.info`; `npx hardhat test solidity` still passes.
Checkpoint: `[CHECK] HH3 test suite fix-up: all tests passing under V3`

#### Phase 5 results (2026-11-09)

Most of the test fix-up happened in Phase 4 (the suite was rewritten on viem there and already passed as one shared run), so Phase 5 verified isolation and coverage:

| Gate | Result |
| --- | --- |
| Every spec file alone (`hardhat test nodejs <file>`, 56 files) | All pass, no fix needed. Per-file totals add up to the shared run: 1,280 passing, 0 failing, 4 skipped |
| `tsc --noEmit` | 0 errors |
| `hardhat test solidity` | 1 passing |

- Skips: the 4 `it.skip` (Finalization, LidoStVaultYieldProvider.basic, YieldManager.integration, L2MessageService) and the 3 `describe.skip` files (`_manualTesting/*`, `ForcedTransaction_sample_0`) are skipped at `HEAD` too.
- Test inventory vs `HEAD` (count of `it(` per spec file): identical for every pre-existing file except `scripts/signerModeAndTaskArgs.ts` (5 → 4). The dropped test covered `scripts/hardhat/verify-task-args.ts`, the bigint-safe serialization of verify arguments for the former `hardhat verify` subprocess; that module was deleted with the subprocess in Phase 4 (verification now runs in-process via `@nomicfoundation/hardhat-verify/verify`, which receives the constructor args as values). New specs: `ignition/modules.ts` (26), `ignition/tasks.ts` (15), `scripts/operational/operationalTasks.ts` (10), `scripts/signerUiNetworkHook.ts` (7), `testInfrastructure/loadFixtureNesting.ts` (1).

### Phase 6 — Script validation (interactive)

Suggested order: `build`, `test`, `coverage`, `test:reportgas`, `lint`, `validate:address-registry`, `validate:deploy-scripts`; then runtime checks the skill cannot gate:
- `make deploy-contracts` against the local stack (18 `tsx` scripts), then e2e smoke.
- `hardhat ignition deploy ignition/modules/TestERC20.ts --network default` and the `L1Protocol` composite on an EDR fork, with private key and with `HARDHAT_SIGNER_UI=true`; interrupt and resume once.
- `VERIFY_CONTRACT`/`--verify` on Sepolia (`chainDescriptors` + Etherscan v2).
- `deploy:validate-upgrade` + `upgradeProxy` dry run on a mainnet fork for one existing 4.x proxy (`forceImport` with `deploy/V1` manifests).
- Regenerate `local-deployments-artifacts/**.json` from HH3 artifacts; run `e2e/scripts/generateAbi.ts`; diff.
- CI changes (§5.7) green on the PR; compare the first coverage number with the last HH2 run.

Gate: every selected script passes; `npx tsc --noEmit` clean.
Checkpoint: `[CHECK] HH3 script validation: all package.json scripts passing`

Results (all 17 `package.json` scripts, run one at a time):

| Script | Result | Change made |
| --- | --- | --- |
| `build` | 199 files compiled (`--force`) | none |
| `lint:sol`, `lint:sol:fix` | 0 errors, 234 warnings (`src/` unchanged, pre-existing) | none |
| `lint:ts`, `lint:ts:fix` | clean | `eslint.config.mjs` ignores `signer-ui/**`: its own package, tsconfig and `lint` script; the contracts tsconfig excludes it since Phase 1, so the type-aware parser failed on its 7 `.ts` files. `signer-ui` lint passes on its own. |
| `prettier:sol`, `prettier:sol:fix`, `lint`, `lint:fix` | pass | none. Locally, two git-ignored, untracked `test/tmp/Glamsterdam*.t.sol` files are not Prettier-formatted; the scripts run on all `**/*.sol`. |
| `autoupdate` | builds the prover sample generator, regenerates `_testData/compressedData*` and the two data-aggregation verifiers | none; the regenerated (non-deterministic) fixtures were restored to `HEAD`. |
| `test` | 1,281 passing (1 Solidity, 1,280 Node.js), 4 skipped, against freshly regenerated fixtures | none |
| `test:reportgas` | 1,281 passing; gas tables printed | none |
| `coverage` | 1,281 passing; `coverage/lcov.info` (141 files, paths relative to `contracts/`) and `coverage/html/`; 62.68 % lines, 74.01 % statements | CI (§5.7): `run-smc-tests.yml` uploads `lcov.info` (artifact `smart-contract-coverage-<tag>.lcov`), prefixes `SF:` paths with `contracts/` first, and drops the "Install Foundry" step (nothing calls `forge`); `codecov-external-pr.yml` artifact name and glob follow; `codecov.yml` `ignore:` mirrors the former `.solcover.js` `skipFiles`. |
| `validate:address-registry` | passed | none |
| `validate:deploy-scripts` | passed (29 Ignition modules, 89 registry lookups) | none |
| `solidity:docgen` | pages written to `docs/api/` | New `scripts/docgen.ts` (`solidity-docgen`'s `docgen()` over `build/build-info`, `sourcesDir: "project/src"`, Hardhat 2 `docgen` settings); script is `hardhat build && tsx scripts/docgen.ts`; `docs/scripts/create-docs-website-pr-branch.sh` calls it. `yield` added to `exclude`: `YieldManagerStorageLayout.sol` uses `@return $`, which solidity-docgen rejects, so `hardhat docgen` already failed at `HEAD` (hidden by the old script's `\|\| echo`); yield pages were never published. The 40 committed pages whose sources did not change are regenerated byte-identical; the others differ only by NatSpec added since the last regeneration. `docs/api/` was left at `HEAD`. |
| `clean` | removes `.openzeppelin build cache node_modules coverage` | dropped the Hardhat 2 outputs `typechain-types` and `coverage.json`. `pnpm install --frozen-lockfile` restores, and the suite passes again afterwards. |

---

### Post-migration restructure: deploy and signer-UI plugins

Done after Phase 6, uncommitted. Goal: less deployment code, a module tree that mirrors `src/`, and OpenZeppelin Upgrades guarding every upgradeable deployment while Ignition stays the deployment engine.

- **`hardhat-plugins/deploy/`** overrides `ignition deploy` (`overrideTask`, dependencies: signer-ui, hardhat-viem, hardhat-ignition, hardhat-upgrades). It resolves a bare module name, builds parameters from the module's `<Name>.deploy.ts` into `ignition/parameters/<network>.json` (unless `--parameters`), walks the module graph for proxies, beacons and upgrade implementations recorded by `ignition/modules/lib/proxy.ts` and validates them (`validateImplementation`; `validateUpgrade` when the proxy has code and its implementation is in `.openzeppelin/`), runs Ignition inside `withSignerUi`, captures Ignition's connection with a dynamic `newConnection` handler, then `forceImport`s, prints the Security Council calldata of upgrade modules and runs `afterDeploy`. Upgrade modules default to deployment id `<Name>-<chainId>-<bytecode hash>`.
- **`hardhat-plugins/signer-ui/`**: `--signer-ui` global flag (Hardhat reads it from `HARDHAT_SIGNER_UI`), `withSignerUi(hre, label, fn)` registering the `network.onRequest` handler for the call only (replaces `runWithSignerUiSession` / `runSignerUiBatch` and the always-on hook), `deploy:recover`. The bridge's on/off state is the scope, not `process.env`.
- **Modules**: `ignition/modules/{rollup,verifiers,messaging,bridging,yield,operational,governance,predeploy,recovery,proxies,testing,upgrades,stacks,lib}`; file name = module id (validated). `scripts/hardhat/deploy/` (registry, parameter builders, operations, deploy tasks, implementation tasks), `ignition/artifacts/abis/` (ABIs now read from the pinned JSON) and the global signer hook are gone. The five implementation tasks are upgrade modules (`LinethRollupUpgrade`, `LinethRollupReinitializeV9`, …); `CallForwardingProxy` has none (its constructor argument never worked with the Hardhat 2 `ImplementationForProxy` script). `GI_*` constants moved to `common/constants/yield.ts`.
- **Known Hardhat behaviour**: Ignition's production (isolated) build writes build infos without `contracts` for type-only files, which the in-process network cannot load; a full default build removes them, so the plugin builds before connecting and the e2e test rebuilds afterwards.
- **Tests**: `test/hardhat/ignition/plugin.ts` (19, replaces `tasks.ts`), `scripts/signerUiNetworkHook.ts` (9, on an HRE created with `signerUi: true`).

---

## 7. Follow-ups (do not block the migration PR)

| Item | Sketch |
| --- | --- |
| `local-deployments-artifacts/*.ts` to viem, drop `ethers` entirely | `createWalletClient` + `deployContract` from `viem`; e2e already has the ABI consts. |
| Regenerate `docs/api/` | `pnpm run solidity:docgen` (now `scripts/docgen.ts`, Phase 6); the committed pages predate recent NatSpec. Yield docs need `@return $` renamed or a docgen fix before `yield` leaves `exclude`. |
| Storage layout export | Document `forge inspect <Contract> storage-layout` in `AGENTS.md`. |
| Solidity tests in CI | `forge test` or `hardhat test solidity` in `run-smc-tests.yml` (depends on §4.4). |
| Keystore / `configVariable` for secrets | `hardhat keystore set DEPLOYER_PRIVATE_KEY ETHERSCAN_API_KEY *_RPC_URL`; update `docs/deployment/README.md`; `--default-sender` for Ignition. |
| Ledger for mainnet | `hardhat-ledger` + `ledgerAccounts` in network config; Ignition supports it natively. |
| Docs touch-up | `contracts/README.md`, `AGENTS.md`, `test/README.md`, `docs/deployment/*` (new Ignition CLI, parameters files, chained modules), `.cursor/rules`, CI comment about `hardhat-foundry`, `signer-ui/README.md`. |
| Chai → `node:assert` | Optional, mechanical. |

---

## 8. Risk register

| Risk | Likelihood | Mitigation |
| --- | --- | --- |
| Workspace `zod`/`undici` overrides break HH3 at load | Resolved | Range-scoped overrides applied in Phase 0 (uncommitted); no resolution change. |
| UI-signed Ignition deploys fail on nonce/gas renumbering by the browser wallet | Low (spike) | Spike: Ignition works with a wallet that picks its own nonce or edits fees. |
| Wallet rejection / Ctrl-C leaves Ignition stuck on an unsent `TRANSACTION_PREPARE_SEND` (HHE10410) | Certain without mitigation | Bridge logs broadcasts; `deploy:recover` drops unsent entries or runs `ignition track-tx` (Phase 0 results). |
| `trustPolicy` rejects OZ Upgrades v4's Defender SDK 2.7.1 dependencies (no provenance) | Resolved | Exact-version `trustPolicyExclude` entries added in Phase 0; revisit when Defender publishes a release with provenance. |
| HH3 `node:test` reporter crashes on OZ validation errors | Certain | Rethrow wrapper in test helpers; report upstream. |
| Parameters layer drifts from `docs/deployment` env-var contract | Medium | `deploy:prepare-params` reuses the existing `getRequiredEnvVar`/registry helpers and env names; `validateDeployScripts.ts` rewritten to check modules against parameters. |
| Proxy admin model drift | Low with Ignition + pinned artifacts | Post-deploy bytecode check vs `deployments/bytecode/mainnet-proxy`. |
| Storage-layout regressions now that the deploy path bypasses OZ `deployProxy` | Medium | `deploy:validate-upgrade` is mandatory in the wrapper task and in docs; `forceImport` after each deploy keeps `.openzeppelin` manifests current. |
| Test rewrite volume (931 `.connect`, 614 `.getAddress`, 251 `ethers.provider`) | High effort, low ambiguity | Fixtures return per-role contract handles; wrappers absorb ~660 assertion sites; sub-agent fan-out. |
| Blob/KZG tests under viem | Medium | viem has first-class 4844 support (`setupKzg`, `toBlobs`); `c-kzg` already present; validate early in Phase 5 batch 1. |
| Coverage number changes | Medium | `codecov.yml` ignores; compare first run. |
| Local/e2e stack regression via `ts-node` → `tsx` | Medium | Phase 6 `make deploy-contracts`; e2e change detection already triggers on `local-deployments-artifacts/**`. |
| Network validation rejects empty URLs/accounts at config load | High without `configVariable` | §5.1; confirm empty `accounts: []`. |
| Two deployment state stores (Ignition journal vs address registry) | Medium | §4.6 policy + registry cross-check. |
| Compiler-cache CI path differs | Low | One slower run, then fix. |

---

## 9. Reference: upstream facts used by this plan

Checked 2026-11-09 via npm and the Hardhat / OpenZeppelin / rocketh docs.

| Package | Latest | Peer / requirement |
| --- | --- | --- |
| `hardhat` | 3.21.2 | Node ≥22.13; deps `zod ^3.23.8`, `tsx ^4.23`, `chokidar ^5`, `ws ^8.21.1` |
| `@nomicfoundation/hardhat-toolbox-viem` | 5.0.7 | peers only: `hardhat ^3.8`, `viem ^2.47.6`, `hardhat-viem ^3.0.4`, `hardhat-viem-assertions ^3.0.5`, `hardhat-node-test-runner ^3`, `hardhat-network-helpers ^3`, `hardhat-verify ^3`, `hardhat-keystore ^3`, `hardhat-ignition ^3.0.7`, `hardhat-ignition-viem ^3.0.7`, `ignition-core ^3.0.7` |
| `@nomicfoundation/hardhat-viem` | 3.1.0 | `connection.viem`: `getPublicClient`, `getWalletClients`, `getWalletClient(address)`, `getTestClient`, `deployContract(name, args, { client, libraries, confirmations, value, gas… })`, `sendDeploymentTransaction`, `getContractAt`; `chainDescriptors.<id>.viemChain` |
| `@nomicfoundation/hardhat-viem-assertions` | 3.1.5 | `viem.assertions.{revert, revertWith, revertWithCustomError, revertWithCustomErrorWithArgs, emit, emitWithArgs, balancesHaveChanged}`; predicates via `@nomicfoundation/hardhat-toolbox-viem/predicates` |
| `@nomicfoundation/hardhat-node-test-runner` | 3.1.2 | `hardhat ^3.12` |
| `@nomicfoundation/hardhat-mocha` | 4.0.1 | `mocha ^12`, `hardhat ^3.12` (not chosen) |
| `@nomicfoundation/hardhat-verify` | 3.1.4 | `verifyContract({ address, constructorArgs, provider }, hre)` from `@nomicfoundation/hardhat-verify/verify`; defaults to the `production` build profile |
| Hardhat Ignition (HH3) | via toolbox 3.0.x | `m.contract(name, artifact?, args, { libraries, value, from, after })`, `m.library`, `m.contractAt(name, artifact?, address)`, `m.call`, `m.staticCall`, `m.readEventArgument`, `m.encodeFunctionCall`, `m.getParameter`, `m.getAccount`, `m.useModule`; `connection.ignition.deploy(module)` returns typed viem instances; journal in `ignition/deployments/<id>/{journal.jsonl, deployed_addresses.json, artifacts/}`; `ignition verify`; Ledger via `hardhat-ledger` |
| `@openzeppelin/hardhat-upgrades` | 4.1.0 | `hardhat ^3.6`, `viem ^2.30`, `hardhat-viem ^3`, `hardhat-verify ^3.0.10`; ESM-only; `import { upgrades } from "@openzeppelin/hardhat-upgrades/viem"`, `await upgrades(hre, connection)`; contracts by name; `deployProxy`, `upgradeProxy`, `deployImplementation`, `deployBeacon*`, `forceImport`, `validateImplementation`, `validateUpgrade`, `prepareUpgrade`, `admin.changeProxyAdmin(proxy, admin, walletClient?)` (4.x proxies only), `admin.transferProxyAdminOwnership(proxy, owner, walletClient?)`, `erc1967.*`, `silenceWarnings`; deploys OZ Contracts 5.0 proxies since 3.0.0 |
| `@openzeppelin/upgrades-core` | 1.48.0 | — |
| `hardhat-deploy` | 2.3.0 | `hardhat ^3.6`, `rocketh ^0.26`, `@rocketh/node`; viem-based; `--tags` kept, `skip` ignored, own proxy contracts; not chosen |
| `hardhat-ignore-warnings` | 0.3.0 | `hardhat ^3.1` |
| `hardhat-storage-layout` / `solidity-docgen` | 0.1.7 / 0.6.0-beta.36 | `hardhat ^2` only; no HH3 release |
| `viem` | 2.57.17 (catalog pins 2.57.11) | `setupKzg`, `toBlobs`, blob transactions, `signAuthorization` (7702), `viem/linea` fee estimation, `viem/chains` `linea`/`lineaSepolia`/`hoodi` |
| HH3 coverage | built-in | `hardhat test --coverage` → `coverage/lcov.info`, `coverage/html/`; no documented exclude config |
