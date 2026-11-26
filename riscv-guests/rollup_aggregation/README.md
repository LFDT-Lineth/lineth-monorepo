# Rollup-aggregation Guest

The RISC-V guest program of the rollup-aggregation proof (`rollup_spec/Readme.md` §2.3). It decodes the canonical `RollupAggregationInput` SSZ envelope (schema id `0x0006`, a container holding the list of verifiable rollup proofs), runs the aggregation statement over it and emits the finalization public inputs (schema id `0x0007`: `keccak256(frame) || frame`, where `frame` is the schema id followed by the SSZ public inputs). The codecs are `src/rollup_aggregation_ssz.zig`; the rollup proofs the input embeds are the `core_serde.rollup_proof` wire contract of `guest_common`.

## Statement

`src/rollup_aggregation.zig` runs §2.3 over the M rollup proofs of the input, in this order:

- **Recursion.** Every rollup proof is verified under the program VK it carries against the hash of its public input, by `lineth_recursion`.
- **Tiling and continuity.** Every proof starts right after the end of the proof before it, and its parent data rolling hash, shared-blob tail, parent block hash, forced-transaction state and chain configuration are the ones that proof ends with.
- **Public inputs.** The messaging-block offsets of every proof are rebased onto the first block of the range (`guest_common/src/messaging_offsets.zig`); the bridge pair is the last non-zero one; the filtered addresses are distinct and ascending; the L2-to-L1 messages concatenate in proof order into the roots of `src/l2_l1_roots.zig`; the program IDs are the distinct, ascending program VKs the proofs bubble up together with the VKs they were verified under, a VK being its own ID.

"first"/"last" below are the first/last elements of the input's `rollup_proofs` list.

| `FinalizationPublicInput` field | Source |
| --- | --- |
| `end_block_number`, `end_block_timestamp` | last proof's `public_inputs` |
| `end_l1_l2_bridge_rolling_hash`, `end_l1_l2_bridge_rolling_hash_message_number` | the pair of the last proof whose message number is non-zero, else zeros |
| `dynamic_chain_config_hash` | first proof's `public_inputs` |
| `parent_ftx_rolling_hash`, `parent_ftx_number` | first proof's `public_inputs` |
| `end_ftx_rolling_hash`, `end_processed_ftx_number` | last proof's `public_inputs` |
| `parent_data_rolling_hash`, `parent_shared_blob_carry_over_tail_bytes`, `parent_block_hash` | first proof's `public_inputs` |
| `end_data_rolling_hash`, `final_shared_blob_carry_over_tail_bytes`, `end_block_hash` | last proof's `public_inputs` |
| `l2_l1_tree_depth` | 5 |
| `l2_l1_roots` | the concatenated messages of every proof in groups of 32, the last group zero-padded, each group folded by `keccak256(left \|\| right)` over 5 levels; no messages, no roots |
| `filtered_addresses` | every proof's addresses, distinct and sorted ascending bytewise |
| `program_ids` | every proof's `program_vks` and its own `program_vk`, distinct and sorted ascending bytewise |
| `l2_messaging_blocks_offsets` | every proof's offsets, each plus the proof's distance from the first block of the first proof |

## Exit codes

A rejected request exits with the category of the step that rejects it (`src/guest_errors.zig`); success exits 0.

| Code | Category |
| --- | --- |
| 1 | unknown |
| 2 | the schema id of the frame is missing or wrong |
| 3 | the SSZ body is not the canonical encoding within the bounds of the wire schema |
| 4 | the request has no rollup proof |
| 5 | the output encoder rejected the public inputs: a list exceeds the bound of the output's wire schema |
| 11 | recursion: a proof is rejected by the recursive verifier, or its public input is beyond a bound of its wire schema |
| 13 | tiling and continuity between consecutive proofs |
| 14 | messaging offsets: a proof's `block_count` differs from its block range, or an offset is out of order, out of range or exceeds 16 bits once rebased |
| 15 | an allocation exceeds the guest heap |

## Development

```bash
make -C rollup_aggregation test             # native host tests; the proofs are real devmode proofs (`go` has to be on PATH)
make -C rollup_aggregation compile          # statically-linked rv64im ELF at zig-out/bin/rollup_aggregation_guest
make -C rollup_aggregation gen-fixture      # rewrites test/testdata/sample.request.ssz and sample.output.bin
make -C rollup_aggregation exec             # compile, then run under the ZkC interpreter on the committed sample request
make -C rollup_aggregation zkc-smoke-exec   # the same run, whose output has to equal the committed output byte for byte
```

`-Dkeccak-accel` (`KECCAK_ACCEL=` on `make`) and `-Dsecp256k1-backend` (`SECP256K1_BACKEND=`) select the keccak and secp256k1 providers of `zkvm_provide`, as in the rollup guest.

### Tests

`test/support/scenario.zig` writes chains of adjacent rollup proofs as literal public inputs and proves each with the devmode prover of `lineth_recursion`.

- `test/rollup_aggregation_test.zig`: the public inputs of one proof and of a chain of three.
- `test/rejection_test.zig`: one test per rule of the statement (the shape of the request, recursion, tiling, each continuity rule, the messaging offsets, the bounds of the output), each asserting the error and its exit-code category.
- `test/sample_fixture_test.zig`: the statement over the committed sample request yields the committed output.
- `test/reference/linea_aggregation_test.zig`: three adjacent rollup requests built from the Linea mainnet window of the rollup package (`rollup`'s `test/testdata/`; no EF corpus is needed), each run through the real rollup statement and proved under a rollup program VK, aggregated; the finalization public inputs are asserted against the rollup outputs.

`zkc-smoke` runs the guest ELF under the ZkC interpreter on the committed sample request and compares the output it prints with the committed output; the comparison tool is shared with the rollup guest through `build_common`.

## Releases

The **RISC-V Rollup Aggregation Guest Release** workflow releases the ELF under `releases/riscv-rollup-aggregation/v<semver>` tags, as described in the [guests README](../README.md#guest-releases); `make release-asset GUEST=rollup-aggregation RELEASE_VERSION=<version>` stages it locally.
