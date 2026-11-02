# Rollup Guest (stub)

Decodes the canonical `RollupProofPrivateInput` SSZ envelope (schema id `0x1001`) and emits the
rollup public inputs (schema id `0x1801`: `keccak256(frame) || frame`, where `frame` is the schema
id followed by the SSZ public inputs). The input codec is `src/rollup_ssz.zig`; the output codec and
the l2-execution proofs the input embeds are the `core_serde` wire contracts of `guest_common`.

The public inputs are built by **echo or sentinel**: every output field is either derived from the
input's l2-execution proofs and request fields the way the specification derives it, or set to a
fixed, precomputed sentinel constant. The guest exercises the wire format and its bounds; proofs,
chunks and conflations pass through unverified.

## Field provenance (`RollupPublicInput`)

"first"/"last" are the first/last elements of the input's `l2_execution_proofs` list (the guest
rejects an empty list).

| `RollupPublicInput` field | Source |
| --- | --- |
| `end_block_number` | last exec proof's `public_inputs.end_block_number` |
| `end_block_timestamp` | last exec proof's `public_inputs.end_block_timestamp` |
| `end_l1_l2_bridge_rolling_hash` | `public_inputs.end_l1_l2_bridge_rolling_hash` of the last exec proof whose message number is non-zero, else zeros |
| `end_l1_l2_bridge_rolling_hash_message_number` | that exec proof's `public_inputs.end_l1_l2_bridge_rolling_hash_message_number`, else zero |
| `dynamic_chain_config_hash` | first exec proof's `public_inputs.dynamic_chain_config_hash` |
| `parent_ftx_rolling_hash` | first exec proof's `public_inputs.parent_ftx_rolling_hash` |
| `parent_ftx_number` | first exec proof's `public_inputs.parent_ftx_number` |
| `end_ftx_rolling_hash` | last exec proof's `public_inputs.end_ftx_rolling_hash` |
| `end_processed_ftx_number` | last exec proof's `public_inputs.end_processed_ftx_number` |
| `parent_data_rolling_hash` | input's `parent_data_rolling_hash` |
| `end_data_rolling_hash` | **sentinel** |
| `parent_block_hash` | first exec proof's `public_inputs.parent_block_hash` |
| `end_block_hash` | last exec proof's `public_inputs.end_block_hash` |
| `parent_data_tail_take_bytes` | input's `parent_data_tail_take_bytes` |
| `final_data_tail_discard_bytes` | **sentinel** |
| `l2_l1_messages` | concatenation of every exec proof's `public_inputs.l2_l1_messages`, proof order |
| `filtered_addresses` | every exec proof's `public_inputs.filtered_addresses`, distinct and sorted ascending bytewise |
| `program_vks` | every `l2_execution_proofs[i].program_vk`, distinct and sorted ascending bytewise |
| `block_count` | sum of every exec proof's `public_inputs.block_count` |
| `l2_messaging_blocks_offsets` | exactly one element: **sentinel** |

Sentinel values and their derivation are defined and commented in `src/rollup.zig`. Exit codes are
defined and commented in `src/guest_errors.zig`.

## Development

```bash
make -C rollup test      # native host unit tests (rollup_ssz + rollup logic)
make -C rollup compile   # statically-linked rv64im ELF at zig-out/bin/rollup_guest
make -C rollup exec      # compile, then run under the ZkC interpreter on a freshly generated sample input
```
