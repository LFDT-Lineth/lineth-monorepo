# Chained Deployments

[← Back to index](README.md)

<br />

This section describes the composite Ignition modules (`contracts/ignition/modules/stacks/`) that deploy multiple contracts in one deployment. They replace the chained `hardhat deploy --tags A,B,C` runs: the contracts a later step needs (the new verifier for LinethRollup, the new BridgedToken beacon for TokenBridge) are wired inside the module, so no address is handed over between scripts and a stale registry/env value for them is never used.

| Hardhat 2 | Now | Module file |
|---|---|---|
| `--tags PlonkVerifier,LinethRollup,Timelock` | `hardhat ignition deploy L1Protocol` | `ignition/modules/stacks/L1Protocol.ts` |
| `--tags L2MessageService,Timelock` | `hardhat ignition deploy L2Protocol` | `ignition/modules/stacks/L2Protocol.ts` |
| `--tags BridgedToken,TokenBridge` | `hardhat ignition deploy TokenBridgeWithBeacon` | `ignition/modules/stacks/TokenBridgeWithBeacon.ts` |

`hardhat ignition deploy <Name>` builds the parameters (env vars + address registry → `ignition/parameters/<network>.json`), runs the OpenZeppelin Upgrades checks, deploys with Ignition and runs the post-deploy steps. See [README.md](README.md#generalized-command-format) for the details, deployment ids and resume.

<br />

## UI-backed chained deployments

Composite deployments support the browser-wallet flow with **one session for the entire run**:

```shell
pnpm exec hardhat ignition deploy L1Protocol --network sepolia --signer-ui
```

With `--signer-ui` (or `HARDHAT_SIGNER_UI=true`):

- **Single session per `hardhat ignition deploy` run** — with `--signer-ui`, the whole Ignition deployment and the post-deploy steps run in one signer session, so the local HTTP bridge and Next.js UI stay up until the whole module is deployed.
- **Multiple transactions** share that same session (sequential wallet approvals). Ignition skips what is already deployed in the deployment id, so a resumed run only asks for the missing transactions.
- The UI can show **batch context** (module, step) and keep a **submitted-transaction history** in the browser (session storage) after the bridge closes.
- By default the **Next.js process is stopped** when the deploy run ends (success or failure), alongside the HTTP bridge; the tab stays open with history in session storage. Set `HARDHAT_SIGNER_UI_LEAVE_NEXT_DEV_AFTER_DEPLOY=true` to keep Next running. The bridge reports a terminal outcome so the UI stops polling cleanly.
- If a wallet prompt is rejected or the run is interrupted, run `pnpm exec hardhat deploy:recover --network <n>` before re-running the same command.
- `--signer-ui` and `DEPLOYER_PRIVATE_KEY` cannot be used together. Hardhat errors if both are present.

If `HARDHAT_SIGNER_UI` is not set (or not `true`), deployments sign with **`DEPLOYER_PRIVATE_KEY`** (the network's account 0).

See the **Browser wallet signing (`HARDHAT_SIGNER_UI`)** section in [README.md](README.md) and [signer-ui/README.md](../../signer-ui/README.md) for env vars and local stack notes.

<br />

## L1MessageService Chained Deployments

Module `L1Protocol`: deploys PlonkVerifier (+ `Mimc`, unless `VERIFIER_MIMC_ADDRESS` is set), LinethRollup (behind the pinned OpenZeppelin 4.x proxy, `defaultVerifier` = the new verifier) and Timelock. The `setVerifierAddress` calldata for `VERIFIER_PROOF_TYPE` is printed after deploying.

Parameters that should be filled either in .env or passed as CLI arguments:

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contracts (same as `--verify`) |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| INITIAL_L2_STATE_ROOT_HASH   | true      | bytes | Initial State Root Hash (shared base) |
| INITIAL_L2_BLOCK_NUMBER   | true      | uint256 | Initial L2 Block Number (shared base) |
| L2_GENESIS_TIMESTAMP | true | uint256 | Genesis timestamp (shared base) |
| L1_SECURITY_COUNCIL  | registry\|env | address | Security Council Address. Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_OPERATORS     | registry\|env | address | Operators Addresses (comma-delimited if multiple). Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_RATE_LIMIT_PERIOD     | true  | uint256   | L1 Rate Limit Period |
| LINETH_ROLLUP_RATE_LIMIT_AMOUNT     | true  | uint256   | L1 Rate Limit Amount |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |
| VERIFIER_CONTRACT_NAME | true | string | PlonkVerifier contract name that should be deployed |
| VERIFIER_PROOF_TYPE | true | string | The proof type that the verifier should be mapped to |
| VERIFIER_CHAIN_ID | true | uint256 | Chain ID passed to the verifier constructor |
| VERIFIER_BASE_FEE | true | uint256 | Base fee passed to the verifier constructor |
| VERIFIER_COINBASE | true | address | Coinbase address passed to the verifier constructor |
| L2_MESSAGE_SERVICE_ADDRESS | registry\|env | address | L2 Message Service address passed to the verifier constructor. Read from registry on stable networks; env var used as fallback. |
| VERIFIER_IS_ALLOWED_CIRCUIT_ID | true | uint256 | Allowed-circuit bit mask passed to the verifier constructor and folded into the verifier chain configuration hash. This value must match the prover-side `isAllowedCircuitID` used to generate aggregation proofs. |
| VERIFIER_MIMC_ADDRESS | false | address | Optional. Reuse an existing deployed `Mimc` library for PlonkVerifier instead of deploying a new one (see [verifier.md](l1/verifier.md)). |
| YIELD_MANAGER_ADDRESS | registry\|env | address | Yield Manager contract address. Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_ADDRESS_FILTER | registry\|env | address | AddressFilter contract address. Read from registry if present; env var used as fallback. |

<br />

Base command:
```shell
pnpm exec hardhat ignition deploy L1Protocol --network sepolia
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> INITIAL_L2_STATE_ROOT_HASH=<bytes> INITIAL_L2_BLOCK_NUMBER=<value> L2_GENESIS_TIMESTAMP=<value> L1_SECURITY_COUNCIL=<address> LINETH_ROLLUP_OPERATORS=<address> LINETH_ROLLUP_RATE_LIMIT_PERIOD=<value> LINETH_ROLLUP_RATE_LIMIT_AMOUNT=<value> YIELD_MANAGER_ADDRESS=<address> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> VERIFIER_CONTRACT_NAME=PlonkVerifierForMultiTypeDataAggregation VERIFIER_IS_ALLOWED_CIRCUIT_ID=<value> pnpm exec hardhat ignition deploy L1Protocol --network sepolia
```

With a reviewed or edited parameters file (no rebuild from env vars):
```shell
pnpm exec hardhat ignition deploy L1Protocol --network sepolia --parameters ignition/parameters/sepolia.json --verify
```

(make sure to replace `<value>` `<bytes>` `<key>` `<address>` with actual values)

<br />

## L2MessageService Chained Deployments

Module `L2Protocol`: deploys L2MessageService (behind the pinned OpenZeppelin 4.x proxy) and Timelock.

| Parameter name        | Required | Input Value | Description |
| ------------------ | -------- | ---------- | ----------- |
| VERIFY_CONTRACT    | false    |true\|false| Verifies the deployed contracts (same as `--verify`) |
| \**DEPLOYER_PRIVATE_KEY* | true     | key | Network-specific private key used when deploying the contract |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY     | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_SECURITY_COUNCIL | registry\|env | address | L2 Security council address. Read from registry on stable networks; env var used as fallback. |
| L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER  | registry\|env  |  address | L1L2 Message Setter address on L2. Read from registry on stable networks; env var used as fallback. |
| L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD    | true  |  uint256 | L2 Rate Limit Period |
| L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT    | true  |  uint256 | L2 Rate Limit Amount |
| TIMELOCK_PROPOSERS | true     | address | Timelock Proposers address |
| TIMELOCK_EXECUTORS | true     | address | Timelock Executors address |
| TIMELOCK_ADMIN_ADDRESS | true     | address | Timelock Admin address |
| MIN_DELAY | true      | uint256 | Timelock Minimum Delay |

<br />

Base command:
```shell
pnpm exec hardhat ignition deploy L2Protocol --network linea_sepolia
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true DEPLOYER_PRIVATE_KEY=<key> ETHERSCAN_API_KEY=<key> INFURA_API_KEY=<key> L2_SECURITY_COUNCIL=<address> L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER=<address> L2_MESSAGE_SERVICE_RATE_LIMIT_PERIOD=<value> L2_MESSAGE_SERVICE_RATE_LIMIT_AMOUNT=<value> TIMELOCK_PROPOSERS=<address> TIMELOCK_EXECUTORS=<address> TIMELOCK_ADMIN_ADDRESS=<address> MIN_DELAY=<value> pnpm exec hardhat ignition deploy L2Protocol --network linea_sepolia
```

(make sure to replace `<value>` `<key>` `<address>` with actual values)

<br />

## TokenBridge & BridgedToken Chained Deployments

Module `TokenBridgeWithBeacon`: deploys the BridgedToken implementation behind the pinned OpenZeppelin 4.x (v4.4.1) `UpgradeableBeacon`, then the TokenBridge (behind the pinned OpenZeppelin 4.x proxy) initialized with that beacon. `BRIDGED_TOKEN_ADDRESS` is not read (it is only used by the standalone `TokenBridge` module).

| Parameter name        | Required | Input Value | Description |
| --------------------- | -------- | ---------- | ----------- |
| VERIFY_CONTRACT       | false    |true\|false| Verifies the deployed contracts (same as `--verify`). |
| \**DEPLOYER_PRIVATE_KEY*       | true     | key | Network-specific private key used when deploying the contract. |
| \**BLOCK_EXPLORER_API_KEY*  | false     | key | Network-specific Block Explorer API Key used for verifying deployed contracts. |
| INFURA_API_KEY         | true     | key | Infura API Key. This is required only when deploying contracts to a live network, not required when deploying on a local dev network. |
| L2_MESSAGE_SERVICE_ADDRESS    | registry\|env  | address   | L2 Message Service address. Read from registry on stable networks; env var used as fallback. |
| LINETH_ROLLUP_ADDRESS         | registry\|env    | address       | L1 Rollup address. Read from registry on stable networks; env var used as fallback. |
| REMOTE_CHAIN_ID       | true      |   uint256     | ChainID of the remote (target) network |
| REMOTE_SENDER_ADDRESS | registry\|env | address | Remote sender address (the TokenBridge on the other chain). Read from registry if present; env var used as fallback. |
| DEPLOY_TOKEN_BRIDGE_ON_L1       | false     |true\|false| If Token Bridge is deployed on L1, DEPLOY_TOKEN_BRIDGE_ON_L1 should be set to `true`. Otherwise it should be `false`|
| L1_SECURITY_COUNCIL | registry\|env | address | L1 Security Council address. Required when `DEPLOY_TOKEN_BRIDGE_ON_L1=true`. Read from registry on stable networks; env var used as fallback. |
| L2_SECURITY_COUNCIL | registry\|env | address | L2 Security Council address. Required when `DEPLOY_TOKEN_BRIDGE_ON_L1=false`. Read from registry on stable networks; env var used as fallback. |
| L1_RESERVED_TOKEN_ADDRESSES | false   | address   | If DEPLOY_TOKEN_BRIDGE_ON_L1=true, then L1_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|
| L2_RESERVED_TOKEN_ADDRESSES | false   | address   | If DEPLOY_TOKEN_BRIDGE_ON_L1=false, then L2_RESERVED_TOKEN_ADDRESSES should be defined. If multiple addresses, provide them in a comma-delimited array.|


Base command:
```shell
pnpm exec hardhat ignition deploy TokenBridgeWithBeacon --network linea_sepolia
```

Base command with cli arguments:
```shell
VERIFY_CONTRACT=true ETHERSCAN_API_KEY=<key> DEPLOYER_PRIVATE_KEY=<key> INFURA_API_KEY=<key> REMOTE_CHAIN_ID=<uint256> DEPLOY_TOKEN_BRIDGE_ON_L1=true L1_SECURITY_COUNCIL=<address> L1_RESERVED_TOKEN_ADDRESSES=<address> L2_MESSAGE_SERVICE_ADDRESS=<address> LINETH_ROLLUP_ADDRESS=<address> REMOTE_SENDER_ADDRESS=<address> pnpm exec hardhat ignition deploy TokenBridgeWithBeacon --network linea_sepolia
```
(make sure to replace `<value>` `<key>` `<address>` with actual values)
