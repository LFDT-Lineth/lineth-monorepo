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

For **fleet** tests, use the fleet-specific target instead:

```bash
make start-env-with-tracing-v2-ci-fleet
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
| `pnpm -F e2e run test:local`                                 | All tests (excludes fleet and liveness, then runs liveness)     |
| `pnpm -F e2e run test:local:run "<file.spec.ts>"`            | Run one test suite                                               |
| `pnpm -F e2e run test:local:run "<file.spec.ts>" -t "<test name>"` | Run one test                                              |
| `pnpm -F e2e run test:fleet:local`                           | Fleet leader/follower consistency tests                          |
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
execution-proof handoff: a new transaction's payload and witness reach the prover request, the dummy
responder replies, and the coordinator persists the batch as proven. It does not validate a real ZK proof
or L1 submission/finalization against the V9 stub.

CI runs zkEVM and RISC-V as a matrix through the same E2E action; both must pass the existing required
check. Failed runs upload separate logs, with RISC-V proof requests/responses included.
