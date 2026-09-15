# Forge contract deployer

One-shot, dev/test-only deployer for the Lineth Forge chart. It packages the
five boot-critical deployment steps used by the Lineth Stack quickstart:

1. L1 `IntegrationTestTrueVerifier` and `LinethRollupV8`
2. L2 `L2MessageService`
3. L1 `BridgedToken` and `TokenBridge`
4. L2 `BridgedToken` and `TokenBridge`
5. L2 EIP-7997 / Arachnid deterministic deployment proxy (`Create2Factory`)

It intentionally does not deploy coordinator, prover, postman, demo-token, or
Forced Transaction Gateway contracts.

## Required inputs

- `L1_RPC_URL`
- `L2_RPC_URL`
- `L1_DEPLOYER_PRIVATE_KEY`
- `L2_DEPLOYER_PRIVATE_KEY`
- `L1_STARTING_NONCE`
- `L2_STARTING_NONCE`
- `DEPLOYER_IMAGE_DIGEST` (`sha256:` plus 64 lowercase hexadecimal characters)
- `INITIAL_L2_STATE_ROOT_HASH` (32-byte hex)

The L1 endpoint is operator-defined. For an external fee-paying L1, fund the L1
deployer before starting the Job. Forge L2 networks are gas-free by default;
set `L2_DEPLOY_GAS_PRICE_WEI=0` explicitly in the chart.

Starting nonces are pinned before deployment so losing or recreating the
checkpoint cannot silently create a second contract stack. Use dedicated
deployment signers with nonce `0` where possible. When reusing a signer, set
the corresponding starting nonce to its expected pending nonce before the
first run; changing it after a checkpoint exists fails closed.

`L2_GENESIS_TIMESTAMP` is optional and otherwise comes from L2 block 0.
`BOOTSTRAP_MANIFEST_FILE` (and optionally `BOOTSTRAP_SCRIPTS_DIR`) enable the
custom bootstrap phase; see below.
`L1_DEPLOYER_ADDRESS` and `L2_DEPLOYER_ADDRESS` can be supplied to assert that
the mounted keys derive the bootstrap/operator-published identities.
`EXPECTED_L1_CHAIN_ID` and `EXPECTED_L2_CHAIN_ID` fail before deployment when
an RPC endpoint points at the wrong network.
Dev/test role addresses default to their chain deployer. Override them with
`L1_SECURITY_COUNCIL`, `LINETH_ROLLUP_OPERATORS`,
`L2_SECURITY_COUNCIL`, and
`L2_MESSAGE_SERVICE_L1L2_MESSAGE_SETTER`.

Optional fee inputs are `L1_DEPLOY_GAS_PRICE_WEI` and
`L2_DEPLOY_GAS_PRICE_WEI`. Rate limits default to one day and 1000 ETH and can
be changed with `CONTRACT_RATE_LIMIT_PERIOD` and
`CONTRACT_RATE_LIMIT_AMOUNT`. RPC readiness is bounded by
`RPC_READY_TIMEOUT_MS` (default: five minutes). Kubernetes output defaults to
the service account namespace and can be set with `POD_NAMESPACE` and
`CHECKPOINT_CONFIG_MAP_NAME`.

## Durable output and reruns

In Kubernetes the Lineth chart creates the `lineth-contract-addresses`
placeholder ConfigMap and the deployer updates it using resource-version
guards. Its schema-version-4 `checkpoint.json` contains the exact deployment
image digest, chain IDs, signer addresses, starting nonces, deterministic
expected addresses, the public deployment-configuration hash, the bootstrap
manifest hash, completed custom bootstrap records, transaction hashes, and
block numbers. It never contains private keys or RPC URLs.
`addresses.json` exposes the same versioned, secret-free deployment record,
while five top-level keys expose the completed primary contract addresses
(including `deterministic-deployment-proxy`).

Before every deployment broadcast, the parent persists an intent containing
the planned contract, nonce, and deterministic address, then acknowledges the
child. A mined receipt atomically replaces that intent before the next
transaction may proceed. Reruns validate the checkpoint identity, including
the image digest, and on-chain bytecode; a digest mismatch fails closed. Fully
verified steps are skipped and a complete stack sends zero deployment
transactions.

Any unresolved broadcast intent fails closed unconditionally, even if the
current RPC reports no pending transaction or the deterministic address has no
code. An operator must reconcile the transaction across providers and decide
whether to preserve the stack or recreate it from clean chain state and a
fresh checkpoint; this version never clears ambiguous intent automatically.
Partial deployments, a missing checkpoint after nonce advancement, or
unexplained signer nonce drift also fail closed.

For local runs, set `CHECKPOINT_FILE=/path/to/checkpoint.json` instead of using
the Kubernetes API.

## Deterministic deployment proxy (step 5)

The final L2 step installs the well-known EIP-7997 / Arachnid
`Create2Factory` so downstream tooling can deploy contracts at deterministic
addresses:

- Factory address: `0x4e59b44847b379578588920cA78FbF26c0B4956C` (a fixed,
  well-known constant — not derived from the deployer nonce)
- Keyless signer it funds: `0x3fAB184622Dc19b6109349B94811493BF2a45362`
- Funding amount: exactly `0.01 ETH` (`10000000000000000` wei) — the budget the
  pre-signed raw transaction hardcodes (`gasLimit 100000` x `gasPrice 100 gwei`)

The step runs **after** the four contract-deployment steps. The deployment
transaction is a pre-signed, keyless broadcast, so it consumes **no** deployer
nonce; only the funding transfer consumes one L2 deployer nonce.

The step is idempotent and restart-safe. When factory bytecode already exists
at the well-known address — including dev chains such as anvil that preinstall
it in genesis — the deployer adopts it as a `recovered` checkpoint record and
sends nothing (no funding transfer, no broadcast). Otherwise it funds the
signer and broadcasts the raw transaction, then verifies the factory bytecode
and checkpoints the broadcast transaction hash.

**Gas-free L2 caveat.** On a gas-free L2 (`L2_DEPLOY_GAS_PRICE_WEI=0`) the
funding transfer itself is free, but the funding is still required: the
pre-signed raw transaction hardcodes a 100 gwei gas price, so the signer must
actually hold 0.01 ETH for the network to accept the broadcast regardless of
the chain's configured gas price. The L2 funding preflight therefore includes
`ARACHNID_FUNDING_WEI` in the required balance even when the selected gas price
is zero, so an unfunded deployer fails at preflight — before any broadcast —
rather than mid-step after an in-flight intent has been persisted.

## Custom bootstrap phase (optional)

Operators of new networks can inject extra transactions after the five core
steps — for example a consortium deploying its own contracts on every partner
network for cross-chain interoperability. The phase is opt-in: when
`BOOTSTRAP_MANIFEST_FILE` is unset the deployer behaves exactly as before.

Set `BOOTSTRAP_MANIFEST_FILE` to the path of a JSON manifest mounted into the
Job. The manifest declares an ordered list of items; `id` values must be unique
lowercase kebab-case and stay stable across reruns (they key the checkpoint).

```json
{
  "version": 1,
  "items": [
    { "id": "fund-relayer", "kind": "sign", "chain": "l2",
      "to": "0xConsortiumRelayer", "valueWei": "100000000000000000" },
    { "id": "bridge-factory", "kind": "presigned", "chain": "l2",
      "rawTx": "0x...", "expectAddress": "0x..." },
    { "id": "extra-setup", "kind": "script", "chain": "l2",
      "script": "consortium/extra.js" }
  ]
}
```

Item kinds:

- **`sign`** — the deployer signs and submits. Set `to` for a value transfer,
  `data` (bytecode) for a bare contract create, or **both** for a contract call
  that carries data — which is exactly how a CREATE2 deploy through the
  deterministic proxy works (a signed call to the factory `to` with the encoded
  deploy `data`). `valueWei` defaults to `0`; `gasLimit` is optional. These are
  how you fund the external accounts that pre-signed transactions spend from.
  They hook into the existing nonce management: the runner reads the deployer's
  current pending nonce after the core steps and hands it to the phase as the
  starting nonce, so each `sign` item continues the chain's nonce sequence with
  no gaps.
  - **Constructor args / create data:** `data` is the raw transaction payload,
    so constructor arguments are pre-encoded onto the bytecode by the operator
    (`bytecode ++ abiEncode(args)`); the step submits it verbatim.
  - **Deployed address:** set `expectAddress` to pin and record the resulting
    contract. For a bare create the step asserts `receipt.contractAddress`
    matches it. For a CREATE2-proxy call the receipt has no contract address
    (it's a call to the factory), so the step instead verifies bytecode landed
    at `expectAddress`.
- **`presigned`** — broadcast a keyless, externally-signed raw transaction
  verbatim. Consumes no deployer nonce. With `expectAddress` set, the item is
  skipped when that address already has bytecode (the same idempotency check
  the deterministic-proxy step uses).
- **`script`** — run an operator-supplied Node script (resolved relative to
  `BOOTSTRAP_SCRIPTS_DIR`) that may create/sign/submit anything. The child
  receives `RPC_URL`, `DEPLOYER_PRIVATE_KEY`, `BOOTSTRAP_ITEM_ID`,
  `BOOTSTRAP_CHAIN`, `BOOTSTRAP_CHAIN_ID`, and `BOOTSTRAP_START_NONCE`, and can
  `require("ethers")` (bundled into the image). Scripts run as restricted child
  processes and are trusted operator input. A script performs arbitrary actions
  with no single transaction hash, so on success it is recorded with a sentinel
  hash and skipped on rerun — the script itself must make its own on-chain
  actions safe to have run once.

Within a run, `sign` (funding) items execute first in manifest order, then
`presigned`, then `script` — so funded accounts exist before any pre-signed
transaction that spends from them. Items are grouped by chain and each chain's
pending items run in one pass.

### Restart-safety and fail-closed semantics

Each completed item is durably recorded in the checkpoint under
`bootstrap.<id>` (kind, chain, transaction hash, block number, chain ID, and
the deployed address when one exists). Completed items are skipped on rerun, so
re-running with the same manifest sends no further transactions.

Like the core steps, an item persists a durable intent (under
`inFlightBootstrap`) before it may broadcast, and the completed record
atomically replaces that intent. A crash after broadcast but before the record
lands leaves the item unresolved, and a rerun fails closed with an
`in-flight bootstrap item` error rather than repeating the action — an operator
must reconcile the transaction before continuing. This is the same fail-closed
contract the core deployment steps use.

The manifest content is hashed (`keccak256` over the normalized manifest) into
the checkpoint identity as `bootstrapHash`. Changing the manifest against an
existing checkpoint fails closed with a `bootstrap manifest` mismatch — the same
rule that guards the image digest, chain IDs, and signers. To deploy a changed
manifest, recreate the network from clean chain state and a fresh checkpoint.

## Development

```bash
pnpm --dir contracts/forge-deployer test
pnpm --dir contracts/forge-deployer typecheck
pnpm --dir contracts/forge-deployer build
make docker-build-contract-deployer
```

## Image release order

Release tags are the complete 40-character merged public commit SHA. First
obtain approval to merge the public PR. After that merge, obtain a separate
approval to publish the exact merged commit, verify that the registry tag
resolves to the workflow-reported immutable digest, and only then pin both the
full-SHA tag and `sha256:` digest in the private chart. Never reuse or overwrite
an existing release tag.
