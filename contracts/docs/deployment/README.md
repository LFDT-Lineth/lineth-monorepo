# Lineth Deployment Scripts

## Address Registry

For stable, named networks (`mainnet`, `sepolia`, `hoodi`, `linea_mainnet`, `linea_sepolia`) the deployment parameter builders (`ignition/modules/**/<ModuleName>.deploy.ts`) read contract addresses from a **PR-reviewed registry** committed to the repository:

```
contracts/deployments/addresses/
  mainnet.json
  sepolia.json
  hoodi.json
  linea_mainnet.json
  linea_sepolia.json
```

See [contracts/deployments/addresses/README.md](../../deployments/addresses/README.md) for the full guide — how addresses are resolved, how to update the registry, and the registry-key-to-env-var mapping.

### Resolution order

| Registry entry | Env var set | Outcome |
|---|---|---|
| Present (non-zero) | Not set | Registry address used — env var is **optional** |
| Present (non-zero) | **Matches** registry | Registry address used |
| Present (non-zero) | **Conflicts** with registry | **Hard fail before any transaction is sent** |
| Absent or zero | Set | Env var used (format-validated) |
| Absent or zero | Not set | Hard fail — no source available |

Networks without a registry (`custom`, `zkevm_dev`, `l2`) fall back to env var only — same behavior as before.

### Address validation

Registry-backed address env vars are validated with viem `isAddress` before use and normalized to EIP-55 checksum form. The registry validator is stricter: committed JSON addresses must already be EIP-55 checksummed. An invalid address (wrong length, bad checksum, etc.) throws immediately with a clear message before any RPC call is made.

<br />

## Environment Variables Naming Convention

Environment variables follow a consistent naming pattern:

| Prefix | Usage |
|--------|-------|
| `L1_` | Ethereum L1 (e.g. `L1_SECURITY_COUNCIL`, `L1_RPC_URL`) |
| `L2_` | Lineth L2 (e.g. `L2_SECURITY_COUNCIL`, `L2_RPC_URL`, `L2_MESSAGE_SERVICE_ADDRESS`) |
| `LINETH_ROLLUP_*` | LinethRollup contract (L1), using the Lineth env namespace |

**Shared per layer:**

- `L1_SECURITY_COUNCIL` — shared across all L1 contracts (Lineth Rollup, Validium, Token Bridge L1, RecoverFunds, Yield Manager)
- `L2_SECURITY_COUNCIL` — shared across all L2 contracts (L2 Message Service, Rollup Revenue Vault, Token Bridge L2)

**Shared L1 base (Lineth Rollup & Validium):** `INITIAL_L2_STATE_ROOT_HASH`, `INITIAL_L2_BLOCK_NUMBER`, `L2_GENESIS_TIMESTAMP` — common to both products via shared contract base

**Product-specific:** `LINETH_ROLLUP_OPERATORS`, `LINETH_ROLLUP_RATE_LIMIT_*`, `VALIDIUM_OPERATORS`, `VALIDIUM_RATE_LIMIT_*`

**RPC endpoints:** `L1_RPC_URL`, `L2_RPC_URL`, `CUSTOM_RPC_URL` (replaces legacy `BLOCKCHAIN_NODE`, `L2_BLOCKCHAIN_NODE`, `CUSTOM_BLOCKCHAIN_URL`)

<br />

This document explains how to deploy the Lineth contracts with [Hardhat Ignition](https://hardhat.org/ignition). Every former `deploy/*.ts` hardhat-deploy script is now an Ignition module under `contracts/ignition/modules/`, in folders that mirror `src/`: chained `--tags` runs are composite modules in `stacks/`, and the scripts that deployed an implementation and printed Security Council calldata are upgrade modules in `upgrades/`. Only the two presigned system-contract broadcasts remain `deploy:*` Hardhat tasks. The operator inputs did not change: the same environment variables (from `.env` or the command line) and the same address-registry rules feed every deployment.

How the scripts can be executed depends on:
- If you're storing deployment variables in an environment file (.env)
- If you plan to deploy an individual module which will deploy a single contract (or a contract + its proxy).
- If you plan to deploy a chained (composite) module that will include multiple contracts.

<br />
Running the script with an .env file set, you will need to make sure that the correct variables are set in the .env file, considering the network that you're deploying on. In this way when the script is being run, it will take the variables it needs to execute the script from that .env file. <br />
<br />

Running the script without an .env file will require you to place the variables as command-line arguments.
The command-line arguments will create or replace existing .env (only in memory) environment variables. If the variables are provided in the terminal as command-line arguments, they will have priority over the same variables if they are defined in the .env file. These need not exist in the .env file.

Furthermore, you can also specify a general set of variables in the .env file (VERIFY_CONTRACT, DEPLOYER_PRIVATE_KEY, ETHERSCAN_API_KEY, INFURA_API_KEY) and provide only the script-specific variables as command-line arguments, when you run each script.

Setting `VERIFY_CONTRACT=true` will start the verifying stage after the contract is deployed, provided that there is a `ETHERSCAN_API_KEY` available in the .env or provided as CLI argument. It is equivalent to `hardhat ignition deploy --verify`; verification is skipped on the in-process network. To verify an existing deployment, run `hardhat ignition verify <deploymentId>`.

<br />

## Network Specific Variables

Dependent on which network you are using, the corresponding API Key or RPC URL must be set.  The block explorer parameter name may also differ. The following table highlights which deployer key and API config is used per network. For all networks, use `DEPLOYER_PRIVATE_KEY` (e.g. `DEPLOYER_PRIVATE_KEY=<key> INFURA_API_KEY=<key>`).

| Network       | Private key parameter name   | API Key / RPC URL | Block explorer parameter name |
| ------------- | ----------------- | ---- | ----------------- | 
| sepolia    | DEPLOYER_PRIVATE_KEY    | INFURA_API_KEY  | ETHERSCAN_API_KEY |
| linea_sepolia | DEPLOYER_PRIVATE_KEY   | INFURA_API_KEY  | ETHERSCAN_API_KEY |
| mainnet   | DEPLOYER_PRIVATE_KEY | INFURA_API_KEY | ETHERSCAN_API_KEY |
| linea_mainnet | DEPLOYER_PRIVATE_KEY |  INFURA_API_KEY  | ETHERSCAN_API_KEY |
| custom    | DEPLOYER_PRIVATE_KEY | CUSTOM_RPC_URL | ETHERSCAN_API_KEY |
| zkevm_dev | DEPLOYER_PRIVATE_KEY | L1_RPC_URL or L2_RPC_URL | n/a |

<br />

## Generalized Command Format

Every deployment is one Ignition module, run by name:

```shell
<possible CLI environment arguments> pnpm exec hardhat ignition deploy <ModuleName> --network sepolia [--verify] [--deployment-id <id>] [--signer-ui]
```

`<ModuleName>` is the module file name under `ignition/modules/` (table below); a path to the module file also works. The in-repo deploy plugin (`hardhat-plugins/deploy/`) wraps Ignition's `ignition deploy`:

1. **Parameters.** The module's `<ModuleName>.deploy.ts` builds its parameters from env vars and the address registry and writes them to `ignition/parameters/<network>.json` (the file is rewritten on every run, so Ignition deploys exactly what was checked). With `--parameters <file.json>`, the file is used as is.
2. **Upgrade safety.** Every upgradeable implementation in the module is validated with OpenZeppelin Upgrades before anything is sent (`validateImplementation` with the module's `unsafeAllow`; `validateUpgrade` against the existing proxy for upgrade modules).
3. **Deployment.** Ignition deploys (journaled, resumable), signing with `DEPLOYER_PRIVATE_KEY` or, with `--signer-ui`, the browser wallet.
4. **After deploying.** New proxies, beacons and implementations are registered in `.openzeppelin/<network>.json` (`forceImport`), upgrade modules print the Security Council calldata, and the module's `afterDeploy` runs (for example the AddressFilter configuration or the `setVerifierAddress` calldata).

| Option | Meaning |
|--------|---------|
| `--deployment-id` | Ignition deployment id. Defaults to `chain-<chainId>` (Ignition's default); upgrade modules default to `<ModuleName>-<chainId>-<bytecode hash>`, so re-running with the same bytecode resumes and new bytecode deploys again. |
| `--parameters` | Use this parameters file instead of building one from env vars (e.g. after reviewing or editing `ignition/parameters/<network>.json`). |
| `--verify` | Verify on Etherscan after deploying. `VERIFY_CONTRACT=true` has the same effect. |
| `--signer-ui` | Sign in the browser wallet (same as `HARDHAT_SIGNER_UI=true`). |

### Ignition specifics

- **Deployment ids and resume.** Ignition journals every transaction under `contracts/ignition/deployments/<deploymentId>/` (`journal.jsonl`, `deployed_addresses.json`). Re-running the same command resumes an interrupted deployment instead of starting over, and a module already deployed in that id is not deployed again. **To deploy a second instance** of a contract on the same network (for example a new verifier), pass a new `--deployment-id`; changing the inputs of an already-deployed module under the same id fails with a reconciliation error. `hardhat ignition status <deploymentId>` shows what was deployed.
- **Proxies.** Upgradeable contracts are deployed behind the pinned OpenZeppelin 4.x (v4.8.3) `TransparentUpgradeableProxy` and `ProxyAdmin` bytecode from `deployments/bytecode/mainnet-proxy` (byte-identical to the mainnet fleet). All proxies deployed into one deployment id share one `ProxyAdmin` (owned by the deployer), like the shared admin the Hardhat 2 OZ plugin kept per network; a new deployment id deploys a new `ProxyAdmin`. `BridgedToken` uses the pinned OpenZeppelin 4.x (v4.4.1) `UpgradeableBeacon` from the same directory, the beacon the Hardhat 2 OZ plugin deployed.
- **Upgrade safety.** The `unsafeAllow` of each implementation is declared where the module deploys it (`deployPinnedTransparentProxy` / `deployUpgradeImplementation` in `ignition/modules/lib/proxy.ts`). Upgrade modules (`ignition/modules/upgrades/`) compare storage layouts with `validateUpgrade` when the proxy's current implementation is in `.openzeppelin/<network>.json`: proxies deployed here are `forceImport`ed after deploying; for older proxies, `forceImport` them first with the previous implementation's validations (`deploy/V1/*` for the V1 contracts). Otherwise the implementation is validated on its own and a warning is printed.
- **Pinned audited bytecode.** `L2MessageServiceV1` and `YieldManagerArtifacts` deploy the audited bytecode in `deploy/V1/` and `deployments/bytecode/2026-01-14/`; OZ validation does not apply to them.
- **Verification.** Contracts compiled from `src/` verify through `--verify` / `hardhat ignition verify`. The pinned proxy, `ProxyAdmin` and audited artifacts are not built from this repository, so verification reports an error for them; they are verified on Etherscan as matches of the already-verified mainnet bytecode (audited artifacts: verify from their audit tag).
- **Signer.** Ignition signs with account 0 of the network (`DEPLOYER_PRIVATE_KEY`) or, with `--signer-ui` / `HARDHAT_SIGNER_UI=true`, through the browser wallet. If a browser-signed deployment is interrupted (wallet rejection, Ctrl-C), run `hardhat deploy:recover` before resuming.
- **Parameters file.** `ignition/parameters/<network>.json` is generated (git-ignored); review it before deploying to a live network. Big integers are written as `"<value>n"`, as Ignition expects.
- **Validations cache.** OpenZeppelin Upgrades stores the upgrade-safety analysis of every compilation in `cache/validations.json` and never prunes it. Deploys build with the isolated `production` profile and tests with the `default` one, so switching between them recompiles and the file keeps growing. If tests or deploys slow down, run `pnpm -F contracts run clean:validations` (deletes the file and rebuilds with `--force`; deleting it without a forced build makes the upgrade checks fail).

### Hardhat 2 command mapping

The per-contract pages under `l1/`, `l2/`, `generic/` and `dual-chain/` list the environment variables of each deployment (unchanged) and the new commands. For scripts or runbooks that still use Hardhat 2 tags:

| Hardhat 2 (`hardhat deploy --tags …`) | Now |
|---|---|
| `PlonkVerifier` | `hardhat ignition deploy PlonkVerifier` (`VERIFIER_CONTRACT_NAME` and `VERIFIER_MIMC_ADDRESS` pick the contract and whether `Mimc` is deployed when the module loads; the `setVerifierAddress` calldata is printed after deploying) |
| `Timelock` | `hardhat ignition deploy Timelock` |
| `LinethRollup` | `hardhat ignition deploy LinethRollup` |
| `Validium` | `hardhat ignition deploy Validium` |
| `L2MessageService` | `hardhat ignition deploy L2MessageService` |
| `L2MessageServiceLineaMainnet` | `hardhat ignition deploy L2MessageServiceV1` |
| `BridgedToken` | `hardhat ignition deploy BridgedToken` |
| `TokenBridge` | `hardhat ignition deploy TokenBridge` |
| `CustomBridgedToken` | `hardhat ignition deploy CustomBridgedToken` |
| `RecoverFunds` | `hardhat ignition deploy RecoverFunds` |
| `TestERC20` | `hardhat ignition deploy TestERC20` |
| `CallForwardingProxy` | `hardhat ignition deploy CallForwardingProxy` |
| `LineaSequencerUptimeFeed` | `hardhat ignition deploy LineaSequencerUptimeFeed` (`LINETH_SEQUENCER_UPTIME_FEED_INITIAL_STATUS` must be `true`, `false`, `1` or `0`) |
| `UpgradeableWithdrawalQueuePredeploy` | `hardhat ignition deploy UpgradeableWithdrawalQueuePredeploy` |
| `UpgradeableConsolidationQueuePredeploy` | `hardhat ignition deploy UpgradeableConsolidationQueuePredeploy` |
| `UpgradeableBeaconChainDepositPredeploy` | `hardhat ignition deploy UpgradeableBeaconChainDepositPredeploy` |
| `RollupRevenueVault` | `hardhat ignition deploy RollupRevenueVault` |
| `L1LineaTokenBurner` | `hardhat ignition deploy L1LineaTokenBurner` |
| `V3DexSwapAdapter` | `hardhat ignition deploy V3DexSwapAdapter` |
| `AddressFilter` | `hardhat ignition deploy AddressFilter` (deploys, then `setFilteredStatus` batches from `ADDRESS_FILTER_FILE_PATH` and the `DEFAULT_ADMIN_ROLE` handover to `L1_SECURITY_COUNCIL`; re-run the same command to retry a failed configuration) |
| `YieldManager` | `hardhat ignition deploy YieldManager` |
| `YieldManagerArtifacts` | `hardhat ignition deploy YieldManagerArtifacts` |
| `YieldManagerImplementation` | `hardhat ignition deploy YieldManagerUpgrade` (prints the `ProxyAdmin.upgrade` calldata) |
| `ForcedTransactionGateway` | `hardhat ignition deploy ForcedTransactionGateway` |
| `PaymentSplitterWrapper` | `hardhat ignition deploy PaymentSplitterWrapper` |
| `SafeExecutionConditions` | `hardhat ignition deploy SafeExecutionConditions` |
| `PlonkVerifier,LinethRollup,Timelock` | `hardhat ignition deploy L1Protocol` ([chained-deployments.md](chained-deployments.md)) |
| `L2MessageService,Timelock` | `hardhat ignition deploy L2Protocol` |
| `BridgedToken,TokenBridge` | `hardhat ignition deploy TokenBridgeWithBeacon` |
| `ImplementationForProxy` (`CONTRACT_NAME=X`) | `hardhat ignition deploy <X>Upgrade` (`LinethRollupUpgrade`, `ValidiumUpgrade`, `L2MessageServiceUpgrade`, `TokenBridgeUpgrade`, `YieldManagerUpgrade`, `RollupRevenueVaultUpgrade`; same proxy env vars) |
| `LinethRollupV8WithReinitialization` | `hardhat ignition deploy LinethRollupReinitializeV9` |
| `L2MessageServiceWithReinitialization` | `hardhat ignition deploy L2MessageServiceReinitializeV3` |
| `TokenBridgeWithReinitialization` | `hardhat ignition deploy TokenBridgeReinitializeV3` |
| `RollupRevenueVaultWithReinitialization` | `hardhat ignition deploy RollupRevenueVaultReinitialize` |
| `EIP2935SystemContract` | `hardhat deploy:eip2935-system-contract` |
| `EIP4788SystemContract` | `hardhat deploy:eip4788-system-contract` |

All commands take `--network <name>` and are run as `pnpm exec hardhat …` from `contracts/`. Upgrade modules deploy the new implementation through Ignition and print the Security Council calldata, as the Hardhat 2 scripts did. `CallForwardingProxy` was accepted by the Hardhat 2 `ImplementationForProxy` script but has a constructor argument, so it never worked there; it has no upgrade module.

<br />

## Browser wallet signing (`--signer-ui`)

The default flow continues to use **`DEPLOYER_PRIVATE_KEY`** (or the configured named account). No UI is involved unless you opt in.

`--signer-ui` (or `HARDHAT_SIGNER_UI=true`) and `DEPLOYER_PRIVATE_KEY` are **mutually exclusive**. If both are set, Hardhat errors before signing instead of guessing which signer to use.

The signer UI under `contracts/signer-ui/` is local operator tooling launched by Hardhat. It is not a published package or an importable library.

To sign and send transactions from a **browser wallet** (MetaMask, Rabby, etc.) instead of putting a private key in the environment:

```shell
pnpm exec hardhat ignition deploy <ModuleName> --network sepolia --signer-ui
```

The signer-UI plugin (`hardhat-plugins/signer-ui/`) adds the `--signer-ui` flag. Tasks that sign wrap their work in `withSignerUi(hre, label, fn)`, which routes the task's JSON-RPC signing (Ignition, OpenZeppelin Upgrades, viem wallet clients) to the browser for the duration of `fn` and closes the session afterwards. `ignition deploy` and the operational Hardhat tasks (under `scripts/operational/`) do this; a standalone script run with `tsx` does not.

### Behaviour summary

| Topic | Details |
|-------|---------|
| **Opt-in** | Only `--signer-ui` or `HARDHAT_SIGNER_UI=true` enables the UI. Otherwise → normal private-key flow. |
| **Exclusive signer mode** | `--signer-ui` and `DEPLOYER_PRIVATE_KEY` cannot be used together. Hardhat fails fast when both are present. |
| **One session per run** | `hardhat ignition deploy <ModuleName>` (including composite modules) uses **one** browser session for the whole Ignition deployment and the post-deploy steps; the session ends when the run finishes. |
| **Accounts in config** | Without `DEPLOYER_PRIVATE_KEY`, `hardhat.config.ts` passes an empty `accounts` array for the network; the wallet you connect in the browser is the signer. |
| **Security** | The bridge requires a per-run session token (embedded in the UI URL). Transactions are checked against the pending request before Hardhat continues. |
| **After the run** | After **`hardhat ignition deploy`**, Hardhat **stops Next.js by default** with the bridge; the tab remains open with history in session storage. Set `HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true` to keep Next running. For **non-deploy** UI sessions, use `HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true` to stop Next when the bridge closes. Optional: `HARDHAT_SIGNER_UI_SHUTDOWN_DRAIN_MS`, `HARDHAT_SIGNER_UI_SHUTDOWN_GRACE_MS`. |
| **Verification** | Unchanged: `VERIFY_CONTRACT` and explorer keys behave as for private-key deploys. |
| **Limitations** | Tasks that broadcast **fixed pre-signed** transactions (`deploy:eip2935-system-contract`, `deploy:eip4788-system-contract`) do not use the browser signer. Ignition skips the futures already deployed in the deployment id (e.g. the shared `ProxyAdmin`), so you may see fewer wallet prompts than “implementation + admin + proxy”. A rejected or interrupted wallet prompt leaves the deployment waiting for `hardhat deploy:recover`. |

### Environment variables (UI)

| Variable | Effect |
|----------|--------|
| `HARDHAT_SIGNER_UI=true` | Same as `--signer-ui`: enable browser signing. Must not be combined with `DEPLOYER_PRIVATE_KEY`. |
| `HARDHAT_SIGNER_UI_OPEN_BROWSER=false` | Print the UI URL only; do not open a tab automatically. |
| `HARDHAT_SIGNER_UI_DEBUG=true` | Forward `next dev` logs to the terminal. |
| `HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true` | After `hardhat ignition deploy`, keep the Next.js dev server when the bridge closes (default is to stop it). |
| `HARDHAT_SIGNER_UI_SHUTDOWN_NEXT_DEV=true` | For non-deploy signer UI sessions, stop Next.js when the HTTP bridge closes. |
| `HARDHAT_SIGNER_UI_SHUTDOWN_DRAIN_MS` | Optional. Milliseconds for the UI to read terminal session state before the bridge closes after deploy (default `1500`). |
| `HARDHAT_SIGNER_UI_SHUTDOWN_GRACE_MS` | Optional. Milliseconds after the bridge closes before stopping Next (default `2000`). |
| `EXPECTED_SIGNER_ADDRESS=0x...` | Optional safety guard. If set, deploy/sign flows fail fast unless the resolved signer address exactly matches this address. |

### Further reading

- **Operator guide** (ports, `zkevm_dev`, `TestERC20` example): [signer-ui/README.md](../../signer-ui/README.md)
- **Chained (composite) deployments**: [chained-deployments.md](chained-deployments.md)
- **Removing this feature** (repo maintainers): [signer-ui-removal.md](signer-ui-removal.md)

<br />

## Order of Precedence

When deploying, the address resolution priority for deployment parameters (`<ModuleName>.deploy.ts`) is:

1. **Address registry** (`contracts/deployments/addresses/<network>.json`) — for stable, named networks. If an entry is present and a conflicting env var is also set, the deploy **hard fails before sending any transaction**.
2. **CLI arguments / env vars** — used when no registry entry exists (e.g. ephemeral addresses, unregistered contracts, or local networks like `custom` / `zkevm_dev`).

The hardhat-deploy artefacts (`deployments/<network_name>/ContractName.json`) are no longer read or written. Addresses produced by a deployment are in `ignition/deployments/<deploymentId>/deployed_addresses.json` (keyed `<ModuleId>#<Name>`); propagate stable addresses to the registry by PR.

<br />

## Deployments

### L1 Contracts (Ethereum)

Verifier deployments require chain-configuration inputs in addition to the contract name and proof type. See [verifier.md](l1/verifier.md) for the full list, including `VERIFIER_IS_ALLOWED_CIRCUIT_ID`.

| Contract | Doc | Hardhat 2 tags (see the mapping above) |
|----------|-----|------|
| PlonkVerifier | [verifier.md](l1/verifier.md) | `PlonkVerifier` |
| LinethRollup | [lineth-rollup.md](l1/lineth-rollup.md) | `LinethRollup`, `LinethRollupV8WithReinitialization` |
| Validium | [validium.md](l1/validium.md) | `Validium` |
| Timelock | [timelock.md](l1/timelock.md) | `Timelock` |
| YieldManager | [yield-manager.md](l1/yield-manager.md) | `YieldManager`, `YieldManagerArtifacts`, `YieldManagerImplementation` |
| RecoverFunds | [recover-funds.md](l1/recover-funds.md) | `RecoverFunds` |
| CallForwardingProxy | [call-forwarding-proxy.md](l1/call-forwarding-proxy.md) | `CallForwardingProxy` |
| L1LineaTokenBurner | [l1-linea-token-burner.md](l1/l1-linea-token-burner.md) | `L1LineaTokenBurner` |
| AddressFilter | [address-filter.md](l1/address-filter.md) | `AddressFilter` |
| ForcedTransactionGateway | [forced-transaction-gateway.md](l1/forced-transaction-gateway.md) | `ForcedTransactionGateway` |

### L2 Contracts (Lineth)

| Contract | Doc | Hardhat 2 tags (see the mapping above) |
|----------|-----|------|
| L2MessageService | [l2-message-service.md](l2/l2-message-service.md) | `L2MessageService`, `L2MessageServiceLineaMainnet`, `L2MessageServiceWithReinitialization` |
| L2 Predeploys | [predeploys.md](l2/predeploys.md) | `EIP2935SystemContract`, `EIP4788SystemContract`, `UpgradeableWithdrawalQueuePredeploy`, `UpgradeableConsolidationQueuePredeploy`, `UpgradeableBeaconChainDepositPredeploy` |
| RollupRevenueVault | [rollup-revenue-vault.md](l2/rollup-revenue-vault.md) | `RollupRevenueVault`, `RollupRevenueVaultWithReinitialization` |
| V3DexSwapAdapter | [v3-dex-swap-adapter.md](l2/v3-dex-swap-adapter.md) | `V3DexSwapAdapter` |
| LineaSequencerUptimeFeed | [sequencer-uptime-feed.md](l2/sequencer-uptime-feed.md) | `LineaSequencerUptimeFeed` |

### Dual-Chain Contracts (L1 or L2)

| Contract | Doc | Hardhat 2 tags (see the mapping above) |
|----------|-----|------|
| TokenBridge | [token-bridge.md](dual-chain/token-bridge.md) | `BridgedToken`, `CustomBridgedToken`, `TokenBridge`, `TokenBridgeWithReinitialization` |

### Generic / Testing

| Contract | Doc | Hardhat 2 tags (see the mapping above) |
|----------|-----|------|
| ImplementationForProxy | [implementation-for-proxy.md](generic/implementation-for-proxy.md) | `ImplementationForProxy` |
| PaymentSplitterWrapper | [payment-splitter.md](generic/payment-splitter.md) | `PaymentSplitterWrapper` |
| TestERC20 | [test-erc20.md](generic/test-erc20.md) | `TestERC20` |

### Chained Deployments

Multi-contract deployment sequences: [chained-deployments.md](chained-deployments.md)

### Optional deploy UI maintenance

How to fully remove the browser-signing stack from the repo: [signer-ui-removal.md](signer-ui-removal.md)
