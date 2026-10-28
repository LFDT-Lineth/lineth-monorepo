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
make start-env-with-riscv                             # prover-ray in dev-mock (placeholder proofs)
pnpm -F e2e run test:riscv:local

RISCV_PROVER_MODE=dev-zkvm make start-env-with-riscv  # prover-ray executes the guest
RISCV_PROVER_MODE=dev-zkvm pnpm -F e2e run test:riscv:local
```

The suite reuses the zkEVM `OpcodeTester` contract (deployed by `start-env-with-riscv`) and sends
`executeAllOpcodes` plus the precompile entry points, so the guest executes every EVM opcode and precompile.
For each transaction it checks that the prover request carries the block payload and witness, that
prover-ray wrote a successful response for that batch, and that the coordinator persisted the batch as
`Proven`. It also checks that batches are proven contiguously from genesis with no prover failures. In
`dev-zkvm`, the response public inputs come from guest execution and are compared with the chain. Neither
mode validates a real ZK proof or L1 submission.

CI runs this suite only on pushes to `main`, as a separate non-required job. Changes to prover-ray,
`riscv-guests`, `rollup_spec/src` or `docker/config/prover/riscv` run in `dev-zkvm` with a prover-ray image
built from the commit; other E2E-relevant changes run in `dev-mock` with the pinned image. Failed runs
upload logs, including RISC-V proof requests/responses.
