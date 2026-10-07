# End to end tests

## Prerequisites

1. Install dependencies from the **repo root**:

```bash
pnpm install
```

2. Build workspace dependencies from the **repo root**:

```bash
pnpm run --filter="e2e..." build
```

3. Spin up the local environment from the **repo root**:

```bash
make start-env-with-tracing-v2-ci
```

4. For remote environments (devnet / sepolia), copy `.env.template` to `.env` and fill in the required values.

### Environment variables

`e2e/.env.template`:

```bash
# Optional: override local genesis paths and L2 RPC
# LOCAL_L1_GENESIS=
# LOCAL_L2_GENESIS=
# LOCAL_L2_RPC_URL=

# Optional: log level (defaults to "info")
# LOG_LEVEL=
```

Variable meanings:

- `LOCAL_L1_GENESIS`: optional absolute/relative path override for local L1 genesis file.
- `LOCAL_L2_GENESIS`: optional absolute/relative path override for local L2 genesis file.
- `LOCAL_L2_RPC_URL`: optional local L2 RPC override (defaults to the zkEVM follower; the RISC-V command selects the sequencer).
- `LOG_LEVEL`: optional logger level (for example `debug`, `info`, `warn`, `error`).

## Run tests

### Local

Run these commands from the monorepo root.

| Command                                                       | Description                                                      |
|---------------------------------------------------------------|------------------------------------------------------------------|
| `pnpm -F e2e run test:local`                                 | All tests (excludes liveness, then runs liveness)     |
| `pnpm -F e2e run test:local:run "<file.spec.ts>"`            | Run one test suite                                               |
| `pnpm -F e2e run test:local:run "<file.spec.ts>" -t "<test name>"` | Run one test                                              |
| `pnpm -F e2e run test:liveness:local`                        | Sequencer liveness tests                                         |
| `pnpm -F e2e run test:sendbundle:local`                      | sendBundle RPC tests                                             |

If you are already inside `e2e/`, remove `-F e2e` and run the same scripts directly with `pnpm run`.

Examples:

```bash
# Run one test suite (all tests in opcodes.spec.ts)
pnpm -F e2e run test:local:run "opcodes.spec.ts"

# Run one test
pnpm -F e2e run test:local:run "opcodes.spec.ts" -t "Should be able to execute all opcodes"
```

### RISC-V local and CI

```bash
make start-env-with-riscv
pnpm -F e2e run test:riscv:local
```

The focused suite reuses the local clients, funding and transaction helpers. It checks ETH transfers,
contract execution with `linea_estimateGas`, sender/recipient denylist rejection and restoration, and the
execution-proof handoff: a new transaction's payload and witness reach the prover request, prover-ray
(`dev-mock`) answers it, and the coordinator persists the batch as proven. It does not validate a real ZK proof
or L1 submission/finalization against the V9 stub.

Known gap: coordinator 2.0.x sends `programId`/`provingSystemVersion` in prover requests, while the pinned
prover-ray image still requires `programVk`, so the execution-proof handoff fails until both images agree on
the `rollup_spec` request fields. The guest program ID is asserted only as an opaque 32-byte value.

Each run selects one stack through the `stack` input. Dispatching "Reusable: Run E2E Tests" directly
defaults to RISC-V; `main` (with an optional `e2e_stack` dispatch input) and release callers default to
zkEVM. Both use the same E2E action and preserve the existing required check. Failed runs upload logs,
with RISC-V proof requests/responses included.
