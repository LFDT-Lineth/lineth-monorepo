# Rollup Guest (stub)

This package contains the RISC-V guest program stub for the rollup proof. It decodes the
canonical `RollupProofPrivateInput` SSZ envelope and emits a schema-valid `RollupOutput` SSZ
envelope, entirely by **echo or sentinel**: every output field is either copied from a defined
place in the input, or set to a fixed, precomputed sentinel constant. It performs **no proof
verification** and **no chunk/conflation folding** — those are the real rollup guest's concern,
landing in a later work package. This stub exists so the wire format, its decode/encode bounds,
and the ZkC build/exec path are exercised end-to-end before that logic exists.

## Scope

- Decodes `RollupProofPrivateInput` (schema id `0x1001`): the parent data-rolling-hash, the
  conflation/chunk witnesses, the recursively-verified `l2_execution_proofs` list, the opaque
  prefix/suffix blob bytes, and the optional boundary hash.
- Maps it to `RollupOutput` (schema id `0x1801`) via the field-provenance table below and emits it.
- Does not verify any embedded proof, fold chunks into a data-rolling-hash, or aggregate rollups —
  see `../rollup-aggregation` for the sibling stub.

## Wire format

Mirrors `rollup_spec/src/rollup_spec/rollup_ssz.py` byte-for-byte: a 2-byte big-endian schema id
followed by the SSZ container (SSZ itself little-endian). Container layouts, field orders, and
list bounds are documented in `src/rollup_ssz.zig`'s header comment and enforced by its decoder —
see that file for the full byte-layout derivation.

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

## Sentinels

Each sentinel is `keccak256("lineth.stub.rollup.<fieldCamelCaseName>")`, precomputed and hardcoded
below; `endOffset` (a `u64`) takes the first 8 bytes of its 32-byte hash, big-endian.

| Field | Formula input | Value |
| --- | --- | --- |
| `l2L1BridgeTransactionTree` | `lineth.stub.rollup.l2L1BridgeTransactionTree` | `0xbc436fcfbb175835d12e0a12f4534f60cd92dbd4babf87db2339277a72ecd22a` |
| `endDataRollingHash` | `lineth.stub.rollup.endDataRollingHash` | `0x1fe617c10b3bfc97fd6d0090c608df47de544a4b7d4f6379300bd96167da2ada` |
| `filteredAddressesHash` | `lineth.stub.rollup.filteredAddressesHash` | `0x8fa4b00e95cd0784a49f00efe0d12f67715a99a6cf54ac4ca5606ebc4d0a42ba` |
| `endOffset` (u64) | `lineth.stub.rollup.endOffset` | `0x1ab5956f53caf2ea` |
| `l2L1Roots` (single element) | `lineth.stub.rollup.l2L1Roots` | `0x45c25758659787f96843b2171dd2091c964ee7fe11518fabf1a4c944b4f75e0e` |

Defined as `pub const` values in `src/rollup.zig`.

## Guest termination semantics

Exit 0 on success. On failure, the guest exits with a deterministic, category-stable nonzero code
(`src/guest_errors.zig`):

| Code | Category | Trigger |
| --- | --- | --- |
| 1 | Unknown | Any error not mapped below |
| 2 | Malformed frame | Missing/wrong-schema-id 2-byte frame prefix |
| 3 | Malformed SSZ | A structurally invalid container: short buffer, misaligned/out-of-order/out-of-bounds offset, truncated variable region |
| 4 | Bounds violation | A list decoded to more elements (or a `ByteList` to more bytes) than its wire-format bound allows |
| 5 | Empty proofs | `l2_execution_proofs` decoded successfully but is empty |
| 6 | Output encode failed | The computed output's SSZ encoding failed (allocator exhaustion) |

## Development

The Zig version, dependency checkout, build manifest, and ZKC helper commands are shared by all
guests at `riscv-guests/`.

```bash
make -C rollup test      # native host unit tests (rollup_ssz + rollup logic)
make -C rollup compile   # statically-linked rv64im ELF at zig-out/bin/rollup_guest
make -C rollup exec      # compile, then run under the ZkC interpreter on the checked-in golden input
```

`make -C rollup compile` links the guest as a **statically-linked rv64im ELF** via
`build_common`'s shared `installGuestElf` — the same [zkvm-standards](https://github.com/eth-act/zkvm-standards/blob/main/standards/riscv-target/target.md)
artifact every guest in this tree produces. `make -C rollup exec` builds it and hands it to the ZKC
interpreter (see the [parent README](../README.md#zkc-interpreter-integration)) against
`test/testdata/10-14-getZkRollupProofV1.request.ssz` — a copy of the golden rollup-input vector
from `rollup_spec/src/rollup_spec/prover_io/testdata/`.
