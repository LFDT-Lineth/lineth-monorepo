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

Networks without a registry (`custom`, `zkevm_dev`, `l2`) fall back to env var only.

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

1. **Parameters.** The module's `<ModuleName>.deploy.ts` builds its parameters from env vars and the address registry and writes them to `ignition/parameters/<network>.json` (the file is rewritten on every run, so Ignition deploys exactly what was checked). With `--parameters <file.json>` (JSON or JSON5), the file is used as is.
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
- **Proxies.** Upgradeable contracts are deployed behind the pinned OpenZeppelin 4.x (v4.8.3) `TransparentUpgradeableProxy` and `ProxyAdmin` bytecode from `deployments/bytecode/mainnet-proxy` (byte-identical to the mainnet fleet). All proxies deployed into one deployment id share one `ProxyAdmin` (owned by the deployer); a new deployment id deploys a new `ProxyAdmin`. `BridgedToken` uses the pinned OpenZeppelin 4.x (v4.4.1) `UpgradeableBeacon` from the same directory.
- **Upgrade safety.** The `unsafeAllow` of each implementation is declared where the module deploys it (`deployPinnedTransparentProxy` / `deployUpgradeImplementation` in `ignition/modules/lib/proxy.ts`). Upgrade modules (`ignition/modules/upgrades/`) compare storage layouts with `validateUpgrade` when the proxy's current implementation is in `.openzeppelin/<network>.json`; otherwise the implementation is validated on its own and a warning is printed. Proxies, beacons and implementations deployed here are `forceImport`ed after deploying, and the public-network manifests are committed (`contracts/.openzeppelin/.gitignore` lists them), so later upgrades are compared against them. A proxy deployed before this workflow is compared only once its current implementation is registered: compile that implementation's source as a contract of this repository and run OpenZeppelin's `forceImport` for the proxy with it.
- **Pinned audited bytecode.** `L2MessageServiceV1` and `YieldManagerArtifacts` deploy the audited bytecode in `deployments/bytecode/V1/` and `deployments/bytecode/2026-01-14/`; OZ validation does not apply to them.
- **Verification.** Contracts compiled from `src/` verify through `--verify` / `hardhat ignition verify`. The pinned proxy, `ProxyAdmin` and audited artifacts are not built from this repository, so verification reports an error for them; they are verified on Etherscan as matches of the already-verified mainnet bytecode (audited artifacts: verify from their audit tag).
- **Signer.** Ignition signs with account 0 of the network (`DEPLOYER_PRIVATE_KEY`) or, with `--signer-ui` / `HARDHAT_SIGNER_UI=true`, through the browser wallet. If a browser-signed deployment is interrupted (wallet rejection, Ctrl-C), run `hardhat deploy:recover` before resuming.
- **Parameters file.** `ignition/parameters/<network>.json` is generated (git-ignored); review it before deploying to a live network. Big integers are written as `"<value>n"`, as Ignition expects.
- **Validations cache.** OpenZeppelin Upgrades stores the upgrade-safety analysis of every compilation in `cache/validations.json` and never prunes it. Deploys build with the isolated `production` profile and tests with the `default` one, so switching between them recompiles and the file keeps growing. If tests or deploys slow down, run `pnpm -F contracts run clean:validations`: it deletes the file and builds, and OpenZeppelin's `build` override recompiles everything when the file is missing.

<br />

## Browser wallet signing (`--signer-ui`)

Deployments sign with `DEPLOYER_PRIVATE_KEY` by default. `--signer-ui` (or `HARDHAT_SIGNER_UI=true`, mutually exclusive with `DEPLOYER_PRIVATE_KEY`) signs every transaction of the run in a browser wallet instead:

```shell
pnpm exec hardhat ignition deploy <ModuleName> --network sepolia --signer-ui
```

See [signer-ui/README.md](../../signer-ui/README.md) for how it works, its environment variables and its limitations.

<br />

## Order of Precedence

When deploying, the address resolution priority for deployment parameters (`<ModuleName>.deploy.ts`) is:

1. **Address registry** (`contracts/deployments/addresses/<network>.json`) — for stable, named networks. If an entry is present and a conflicting env var is also set, the deploy **hard fails before sending any transaction**.
2. **CLI arguments / env vars** — used when no registry entry exists (e.g. ephemeral addresses, unregistered contracts, or local networks like `custom` / `zkevm_dev`).

Addresses produced by a deployment are in `ignition/deployments/<deploymentId>/deployed_addresses.json` (keyed `<ModuleId>#<Name>`); propagate stable addresses to the registry by PR.

<br />

## Deployments

### L1 Contracts (Ethereum)

Verifier deployments require chain-configuration inputs in addition to the contract name and proof type. See [verifier.md](l1/verifier.md) for the full list, including `VERIFIER_IS_ALLOWED_CIRCUIT_ID`.

| Contract | Doc |
|----------|-----|
| PlonkVerifier | [verifier.md](l1/verifier.md) |
| LinethRollup | [lineth-rollup.md](l1/lineth-rollup.md) |
| Validium | [validium.md](l1/validium.md) |
| Timelock | [timelock.md](l1/timelock.md) |
| YieldManager | [yield-manager.md](l1/yield-manager.md) |
| RecoverFunds | [recover-funds.md](l1/recover-funds.md) |
| CallForwardingProxy | [call-forwarding-proxy.md](l1/call-forwarding-proxy.md) |
| L1LineaTokenBurner | [l1-linea-token-burner.md](l1/l1-linea-token-burner.md) |
| AddressFilter | [address-filter.md](l1/address-filter.md) |
| ForcedTransactionGateway | [forced-transaction-gateway.md](l1/forced-transaction-gateway.md) |

### L2 Contracts (Lineth)

| Contract | Doc |
|----------|-----|
| L2MessageService | [l2-message-service.md](l2/l2-message-service.md) |
| L2 Predeploys | [predeploys.md](l2/predeploys.md) |
| RollupRevenueVault | [rollup-revenue-vault.md](l2/rollup-revenue-vault.md) |
| V3DexSwapAdapter | [v3-dex-swap-adapter.md](l2/v3-dex-swap-adapter.md) |
| LineaSequencerUptimeFeed | [sequencer-uptime-feed.md](l2/sequencer-uptime-feed.md) |

### Dual-Chain Contracts (L1 or L2)

| Contract | Doc |
|----------|-----|
| TokenBridge | [token-bridge.md](dual-chain/token-bridge.md) |

### Generic / Testing

| Contract | Doc |
|----------|-----|
| ImplementationForProxy | [implementation-for-proxy.md](generic/implementation-for-proxy.md) |
| PaymentSplitterWrapper | [payment-splitter.md](generic/payment-splitter.md) |
| TestERC20 | [test-erc20.md](generic/test-erc20.md) |

### Chained Deployments

Multi-contract deployment sequences: [chained-deployments.md](chained-deployments.md)
