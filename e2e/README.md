# End to end tests

## Prerequisites

1. Install dependencies from this directory (the `postinstall` script generates ABI typings from contract artifacts):

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

4. For remote environments (dev / sepolia), copy `.env.template` to `.env` and fill in the required RPC keys and private keys.

## Run tests

### Local

| Command                                            | Description                                                         |
|----------------------------------------------------|---------------------------------------------------------------------|
| `pnpm run test:local`                          | Full suite (excludes fleet and liveness, then runs liveness)    |
| `pnpm run test:local:run "<file.spec.ts>"`     | Run one test suite                                               |
| `pnpm run test:local:run "<file.spec.ts>" -t "<test name>"` | Run one test                                              |
| `pnpm run test:fleet:local`                    | Fleet leader/follower consistency tests                         |
| `pnpm run test:liveness:local`                 | Sequencer liveness tests                                        |
| `pnpm run test:sendbundle:local`               | sendBundle RPC tests                                            |

Example:

```bash
# Run one test suite (all tests in opcodes.spec.ts)
pnpm run test:local:run "opcodes.spec.ts"

# Run one test
pnpm run test:local:run "opcodes.spec.ts" -t "Should be able to execute all opcodes"
```

### Remote

| Command                        | Description                                                                       |
|--------------------------------|-----------------------------------------------------------------------------------|
| `pnpm run test:dev`       | Uses DEV env, may need to update config in `src/config/schema/dev.ts`              |
| `pnpm run test:sepolia`   | Uses Sepolia env, may need to update config in `src/config/schema/sepolia.ts`      |

