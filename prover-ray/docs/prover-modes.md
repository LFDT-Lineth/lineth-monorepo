# Prover modes

Each proof layer has its own mode, set with `prover_mode` in its config section (`[execution]`, `[rollup]`, `[aggregation]`). Every mode except `full` is a dev mode: it makes no real proof, and the response's `proverVersion` gets the mode as a suffix (for example `0.0.0-riscv-dev-partial`).

The guest is the RISC-V program whose run is proven: the `l2_execution` guest for execution, the rollup guest for rollup and the aggregation guest for aggregation.

| Mode | Runs the guest | Traces the guest | Constraints | Conglomeration | Public inputs | Proof |
|---|---|---|---|---|---|---|
| `dev-mock` | no | no | not checked | none | placeholder | marker bytes |
| `dev-zkvm` | yes | no | not checked | none | real | marker bytes |
| `dev-partial` | yes | yes, shard by shard | checked | mock (checks the shards fit together) | real | devmode proof |
| `dev-full-dummy-wrap` (aggregation only) | yes | yes, shard by shard | proved | real recursion | real | real proof, but a dummy BN254 proof for L1 |
| `full` | yes | yes, shard by shard | proved | real recursion | real | real proof |

### dev-mock

- Runs nothing and returns a placeholder response at once.
- Only the end block number is set.
- Use it to test the request and response flow.

### dev-zkvm

- Runs the guest on the real input, without a trace.
- The public inputs are real.
- Nothing is checked against the constraints.
- The guest output must be equal to the native runner's output (temporary, see below).

### dev-partial

- Runs the guest and traces it, shard by shard.
- Checks every shard against every constraint.
- The mock conglomeration checks that:
  - the shards agree on the program VK and the guest output;
  - every shard is there;
  - the message bus balances across the shards.
- The proof is a devmode proof:
  - a real wiop proof that the next layer can verify;
  - but anyone can make one, so it proves nothing.
- If a native runner is set, the guest output must also be equal to its output (temporary, see below).

### dev-full-dummy-wrap (aggregation only)

- The same as `full`.
- But the final gnark BN254 proof sent to L1 is a dummy one.

### full

- The production path.
- Real proofs, merged by real recursion.

### Native runner check

- The native runner runs the same guest code outside the VM.
- The check will be dropped once the VM passes the RISC-V compliance tests, or before a second guest version goes live.

## Status per layer

| Mode | Execution | Rollup | Aggregation |
|---|---|---|---|
| `dev-mock` | ready | ready | ready |
| `dev-zkvm` | ready | not yet | not yet |
| `dev-partial` | ready | not yet | not yet |
| `dev-full-dummy-wrap` | n/a | n/a | not yet |
| `full` | not yet | not yet | not yet |

For rollup and aggregation, the modes other than `dev-mock` need the layer's guest input format (not decided yet) and a guest that verifies the proof type of the layer below: a devmode proof below `dev-partial`, a real proof below `full`. In every mode except `full`, aggregation sends a dummy BN254 proof to L1.

Sample configs: `prover-ray/config/config-dev-mock.toml`, `config-dev-zkvm.toml`, `config-dev-partial.toml`.
