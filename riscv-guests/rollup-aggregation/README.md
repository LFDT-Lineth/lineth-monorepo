# Rollup-Aggregation Guest (stub)

Decodes the canonical `RollupAggregationProofPrivateInput` SSZ envelope (schema id `0x1002`) and
emits a schema-valid `RollupAggregationOutput` SSZ envelope (schema id `0x1802`), entirely by
**echo or sentinel**: every output field is either copied from a defined place in the input, or set
to a fixed, precomputed sentinel constant. No proof verification, no rollup aggregation — that
logic lands with the real rollup-aggregation guest, which will replace this package's contents.

## Field provenance (`RollupAggregationOutput`)

"first"/"last" are the first/last elements of the input's `rollup_proofs` list (the guest rejects
an empty list — see [Guest termination semantics](#guest-termination-semantics)).

| `RollupAggregationOutput` field | Source |
| --- | --- |
| `public_inputs.end_block_number` | last rollup proof's `public_inputs.end_block_number` |
| `public_inputs.end_block_timestamp` | last rollup proof's `public_inputs.end_block_timestamp` |
| `public_inputs.l2_l1_bridge_transaction_tree` | **sentinel** |
| `public_inputs.parent_l1_l2_bridge_rolling_hash` | first rollup proof's `public_inputs.parent_l1_l2_bridge_rolling_hash` |
| `public_inputs.parent_l1_l2_bridge_rolling_hash_message_number` | first rollup proof's `public_inputs.parent_l1_l2_bridge_rolling_hash_message_number` |
| `public_inputs.end_l1_l2_bridge_rolling_hash` | last rollup proof's `public_inputs.end_l1_l2_bridge_rolling_hash` |
| `public_inputs.end_l1_l2_bridge_rolling_hash_message_number` | last rollup proof's `public_inputs.end_l1_l2_bridge_rolling_hash_message_number` |
| `public_inputs.dynamic_chain_config_hash` | first rollup proof's `public_inputs.dynamic_chain_config_hash` |
| `public_inputs.parent_ftx_rolling_hash` | first rollup proof's `public_inputs.parent_ftx_rolling_hash` |
| `public_inputs.parent_ftx_number` | first rollup proof's `public_inputs.parent_ftx_number` |
| `public_inputs.end_ftx_rolling_hash` | last rollup proof's `public_inputs.end_ftx_rolling_hash` |
| `public_inputs.end_processed_ftx_number` | last rollup proof's `public_inputs.end_processed_ftx_number` |
| `public_inputs.filtered_addresses_hash` | **sentinel** |
| `public_inputs.parent_data_rolling_hash` | first rollup proof's `public_inputs.parent_data_rolling_hash` |
| `public_inputs.end_data_rolling_hash` | last rollup proof's `public_inputs.end_data_rolling_hash` |
| `public_inputs.parent_block_hash` | first rollup proof's `public_inputs.parent_block_hash` |
| `public_inputs.end_block_hash` | last rollup proof's `public_inputs.end_block_hash` |
| `public_inputs.start_offset` | first rollup proof's `public_inputs.start_offset` |
| `public_inputs.end_offset` | last rollup proof's `public_inputs.end_offset` |
| `public_inputs.program_vks` | union of every embedded `public_inputs.program_vks` and every `rollup_proofs[i].program_vk`, deduplicated and sorted ascending bytewise |
| `l2_l1_roots` | concatenation of every rollup proof's `l2_l1_roots`, input order (no dedup) |
| `filtered_addresses` | concatenation of every rollup proof's `filtered_addresses`, input order (no dedup) |
| `l2_messaging_blocks_offsets` | exactly one element: **sentinel** |

Sentinel values and their derivation are defined and commented in `src/rollup_aggregation.zig`.
Exit codes are defined and commented in `src/guest_errors.zig`.

## Development

```bash
make -C rollup-aggregation test      # native host unit tests (rollup_aggregation_ssz + logic)
make -C rollup-aggregation compile   # statically-linked rv64im ELF at zig-out/bin/rollup_aggregation_guest
make -C rollup-aggregation exec      # compile, then run under the ZkC interpreter on a freshly generated sample input
```
