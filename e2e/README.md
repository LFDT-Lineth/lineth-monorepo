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
make start-env-with-tracing-v2-fleet-ci
```

4. For remote environments (devnet / sepolia), copy `.env.template` to `.env` and fill in the required values.

### Environment variables

`e2e/.env.template`:

```bash
# Optional: override default genesis file paths (local only)
# LOCAL_L1_GENESIS=
# LOCAL_L2_GENESIS=

# Optional: log level (defaults to "info")
# LOG_LEVEL=
```

Variable meanings:

- `LOCAL_L1_GENESIS`: optional absolute/relative path override for local L1 genesis file.
- `LOCAL_L2_GENESIS`: optional absolute/relative path override for local L2 genesis file.
- `LOG_LEVEL`: optional logger level (for example `debug`, `info`, `warn`, `error`).

## Run tests

### Local

Run these commands from the monorepo root.

| Command                                                       | Description                                                      |
|---------------------------------------------------------------|------------------------------------------------------------------|
| `pnpm run -F e2e test:local`                                 | All tests (excludes fleet and liveness, then runs liveness)     |
| `pnpm run -F e2e test:local:run "<file.spec.ts>"`            | Run one test suite                                               |
| `pnpm run -F e2e test:local:run "<file.spec.ts>" -t "<test name>"` | Run one test                                              |
| `pnpm run -F e2e test:fleet:local`                           | Fleet leader/follower consistency tests                          |
| `pnpm run -F e2e test:liveness:local`                        | Sequencer liveness tests                                         |
| `pnpm run -F e2e test:sendbundle:local`                      | sendBundle RPC tests                                             |

If you are already inside `e2e/`, remove `-F e2e` and run the same scripts directly with `pnpm run`.

Examples:

```bash
# Run one test suite (all tests in opcodes.spec.ts)
pnpm run -F e2e test:local:run "opcodes.spec.ts"

# Run one test
pnpm run -F e2e test:local:run "opcodes.spec.ts" -t "Should be able to execute all opcodes"
```


