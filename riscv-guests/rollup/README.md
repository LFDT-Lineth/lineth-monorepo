# Rollup Guest (stub)

Decodes the canonical `RollupProofPrivateInput` SSZ envelope (schema id `0x1001`) and emits a
schema-valid `RollupOutput` SSZ envelope (schema id `0x1801`), entirely by **echo or sentinel**:
every output field is either copied from a defined place in the input, or set to a fixed,
precomputed sentinel constant. No proof verification, no chunk/conflation folding — that logic
lands with the real rollup guest, which will replace this package's contents.

## Field provenance (`RollupOutput`)

"first"/"last" are the first/last elements of the input's `l2_execution_proofs` list (the guest
rejects an empty list — see [Guest termination semantics](#guest-termination-semantics)).

| `RollupOutput` field | Source |
| --- | --- |
| `public_inputs.end_block_number` | last exec proof's `public_inputs.end_block_number` |
| `public_inputs.end_block_timestamp` | last exec proof's `public_inputs.end_block_timestamp` |
| `public_inputs.l2_l1_bridge_transaction_tree` | **sentinel** |
| `public_inputs.parent_l1_l2_bridge_rolling_hash` | first exec proof's `public_inputs.parent_l1_l2_bridge_rolling_hash` |
| `public_inputs.parent_l1_l2_bridge_rolling_hash_message_number` | first exec proof's `public_inputs.parent_l1_l2_bridge_rolling_hash_message_number` |
| `public_inputs.end_l1_l2_bridge_rolling_hash` | last exec proof's `public_inputs.end_l1_l2_bridge_rolling_hash` |
| `public_inputs.end_l1_l2_bridge_rolling_hash_message_number` | last exec proof's `public_inputs.end_l1_l2_bridge_rolling_hash_message_number` |
| `public_inputs.dynamic_chain_config_hash` | first exec proof's `public_inputs.dynamic_chain_config_hash` |
| `public_inputs.parent_ftx_rolling_hash` | first exec proof's `public_inputs.parent_ftx_rolling_hash` |
| `public_inputs.parent_ftx_number` | first exec proof's `public_inputs.parent_ftx_number` |
| `public_inputs.end_ftx_rolling_hash` | last exec proof's `public_inputs.end_ftx_rolling_hash` |
| `public_inputs.end_processed_ftx_number` | last exec proof's `public_inputs.end_processed_ftx_number` |
| `public_inputs.filtered_addresses_hash` | **sentinel** |
| `public_inputs.parent_data_rolling_hash` | input's `parent_data_rolling_hash` |
| `public_inputs.end_data_rolling_hash` | **sentinel** |
| `public_inputs.parent_block_hash` | first exec proof's `public_inputs.parent_block_hash` |
| `public_inputs.end_block_hash` | last exec proof's `public_inputs.end_block_hash` |
| `public_inputs.start_offset` | input's `start_offset` |
| `public_inputs.end_offset` | **sentinel** |
| `public_inputs.program_vks` | every `l2_execution_proofs[i].program_vk`, deduplicated and sorted ascending bytewise |
| `start_block_number` | first exec proof's `proof.start_block_number` |
| `l2_l1_roots` | exactly one element: **sentinel** |
| `filtered_addresses` | concatenation of every exec proof's `filtered_addresses`, input order (no dedup) |

Sentinel values and their derivation are defined and commented in `src/rollup.zig`. Exit codes are
defined and commented in `src/guest_errors.zig`.

## Development

```bash
make -C rollup test      # native host unit tests (rollup_ssz + rollup logic)
make -C rollup compile   # statically-linked rv64im ELF at zig-out/bin/rollup_guest
make -C rollup exec      # compile, then run under the ZkC interpreter on the checked-in golden input
```
