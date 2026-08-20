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
- `INITIAL_L2_STATE_ROOT_HASH` (32-byte hex)

The L1 endpoint is operator-defined. For an external fee-paying L1, fund the L1
deployer before starting the Job. Forge L2 networks are gas-free by default;
set `L2_DEPLOY_GAS_PRICE_WEI=0` explicitly in the chart.

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
guards. Its `checkpoint.json` contains chain IDs, signer addresses, starting
nonces, deterministic expected addresses, the public deployment-configuration
hash, transaction hashes, and block numbers. It never contains private keys or
RPC URLs. `addresses.json` exposes the same versioned, secret-free deployment
record, while four top-level keys expose the completed primary contract
addresses.

Every deployment receipt is persisted and acknowledged before the child deploy
script can submit its next transaction. Reruns validate checkpoint identity and on-chain bytecode, skip fully
verified steps, and send zero deployment transactions for a complete stack.
Partial deployments or unexplained signer nonce drift fail closed.

For local runs, set `CHECKPOINT_FILE=/path/to/checkpoint.json` instead of using
the Kubernetes API.

## Development

```bash
pnpm --dir contracts/forge-deployer test
pnpm --dir contracts/forge-deployer typecheck
pnpm --dir contracts/forge-deployer build
make docker-build-contract-deployer
```
