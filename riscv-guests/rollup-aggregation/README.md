# Rollup-Aggregation Guest (stub)

This package contains the RISC-V guest program stub for the rollup-aggregation proof. It decodes
the canonical `RollupAggregationProofPrivateInput` SSZ envelope and emits a schema-valid
`RollupAggregationOutput` SSZ envelope, entirely by **echo or sentinel**: every output field is
either copied from a defined place in the input, or set to a fixed, precomputed sentinel constant.
It performs **no proof verification** and **no rollup aggregation** — those are the real
rollup-aggregation guest's concern, landing in a later work package. This stub exists so the wire
format, its decode/encode bounds, and the ZkC build/exec path are exercised end-to-end before that
logic exists.

## Scope

- Decodes `RollupAggregationProofPrivateInput` (schema id `0x1002`): a list of recursively-verified
  `rollup_proofs`, each carrying its own 20-field public-input tuple, `l2_l1_roots`, and
  `filtered_addresses`.
- Maps it to `RollupAggregationOutput` (schema id `0x1802`) via the field-provenance table below
  and emits it.
- Does not verify any embedded proof or aggregate rollups into a single finalization submission —
  see `../rollup` for the sibling stub.

## Wire format

Mirrors `rollup_spec/src/rollup_spec/rollup_ssz.py` byte-for-byte: a 2-byte big-endian schema id
followed by the SSZ container (SSZ itself little-endian). Container layouts, field orders, and
list bounds are documented in `src/rollup_aggregation_ssz.zig`'s header comment and enforced by
its decoder — see that file for the full byte-layout derivation. `RollupPublicInput`'s 20-field
container is decoded here independently of `../rollup`'s own copy (this package takes no path
dependency on it — see that file's header comment).

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

## Sentinels

Each sentinel is `keccak256("lineth.stub.rollup-aggregation.<fieldCamelCaseName>")`, precomputed
and hardcoded below; `l2MessagingBlocksOffsets` (a `u64`) takes the first 8 bytes of its 32-byte
hash, big-endian.

| Field | Formula input | Value |
| --- | --- | --- |
| `l2L1BridgeTransactionTree` | `lineth.stub.rollup-aggregation.l2L1BridgeTransactionTree` | `0x0918836198239a5edf0936db3e28a64d5c4e195fd728e6ddfc3544fc95008ab3` |
| `filteredAddressesHash` | `lineth.stub.rollup-aggregation.filteredAddressesHash` | `0x63d40d3ea387065027b369a25cc441dd7685e2a9a9e0e56556ec2e82bbe2273c` |
| `l2MessagingBlocksOffsets` (u64, single element) | `lineth.stub.rollup-aggregation.l2MessagingBlocksOffsets` | `0xd18d873fe2a9f192` |

Defined as `pub const` values in `src/rollup_aggregation.zig`.

## Guest termination semantics

Exit 0 on success. On failure, the guest exits with a deterministic, category-stable nonzero code
(`src/guest_errors.zig`):

| Code | Category | Trigger |
| --- | --- | --- |
| 1 | Unknown | Any error not mapped below |
| 2 | Malformed frame | Missing/wrong-schema-id 2-byte frame prefix |
| 3 | Malformed SSZ | A structurally invalid container: short buffer, misaligned/out-of-order/out-of-bounds offset, truncated variable region |
| 4 | Bounds violation | A list decoded to more elements (or a `ByteList` to more bytes) than its wire-format bound allows |
| 5 | Empty proofs | `rollup_proofs` decoded successfully but is empty |
| 6 | Output encode failed | The computed output's SSZ encoding failed (allocator exhaustion) |

## Development

The Zig version, dependency checkout, build manifest, and ZKC helper commands are shared by all
guests at `riscv-guests/`.

```bash
make -C rollup-aggregation test      # native host unit tests (rollup_aggregation_ssz + logic)
make -C rollup-aggregation compile   # statically-linked rv64im ELF at zig-out/bin/rollup_aggregation_guest
make -C rollup-aggregation exec      # compile, then run under the ZkC interpreter on the checked-in golden input
```

`make -C rollup-aggregation compile` links the guest as a **statically-linked rv64im ELF** via
`build_common`'s shared `installGuestElf` — the same [zkvm-standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md)
artifact every guest in this tree produces. `make -C rollup-aggregation exec` builds it and hands
it to the ZKC interpreter (see the [parent README](../README.md#zkc-interpreter-integration))
against `test/testdata/10-18-getZkRollupAggregationProofV1.request.ssz` — a copy of the golden
aggregation-input vector from `rollup_spec/src/rollup_spec/prover_io/testdata/`.
