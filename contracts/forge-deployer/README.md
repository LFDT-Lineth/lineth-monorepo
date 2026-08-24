# Forge contract deployer

One-shot, dev/test-only deployer for the Lineth Forge chart. It packages the
four boot-critical deployment steps used by the Lineth Stack quickstart:

1. L1 `IntegrationTestTrueVerifier` and `LinethRollupV8`
2. L2 `L2MessageService`
3. L1 `BridgedToken` and `TokenBridge`
4. L2 `BridgedToken` and `TokenBridge`

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
guards. Its schema-version-2 `checkpoint.json` contains the exact deployment
image digest, chain IDs, signer addresses, starting nonces, deterministic
expected addresses, the public deployment-configuration hash, transaction
hashes, and block numbers. It never contains private keys or RPC URLs.
`addresses.json` exposes the same versioned, secret-free deployment record,
while four top-level keys expose the completed primary contract addresses.

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
