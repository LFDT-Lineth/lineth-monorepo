//! Wire contract of the rollup-aggregation guest: the input container behind the 0x0006 schema id
//! and the `FinalizationPublicInput` the guest commits to behind the 0x0007 schema id. The decoder
//! accepts only the canonical encoding, and the byte lists alias the decoded buffer.
//!
//! The already-proven rollup proofs the input embeds are core_serde's contract. The output is
//! `keccak256(frame) || frame`, `frame = schema_id || SSZ(public inputs)`.

const core_serde = @import("core_serde");
const ssz = @import("guest_common").ssz;

const Allocator = @import("std").mem.Allocator;
const output_frame = core_serde.output_frame;
const rollup_proof = core_serde.rollup_proof;

pub const INPUT_SCHEMA_ID: u16 = 0x0006;
pub const OUTPUT_SCHEMA_ID: u16 = 0x0007;

// List bounds of the wire schema: encoders and decoders reject longer lists.
/// Rollup proofs one aggregation proof verifies.
pub const MAX_ROLLUP_PROOFS_PER_AGGREGATION: usize = 1 << 10;
pub const MAX_L2_L1_ROOTS: usize = 1 << 16;

pub const RollupAggregationInput = struct {
    rollup_proofs: ssz.VariableList(rollup_proof.VerifiableRollupProof, MAX_ROLLUP_PROOFS_PER_AGGREGATION),
};

/// The finalization public input tuple, in wire order.
pub const FinalizationPublicInput = struct {
    end_block_number: u64,
    end_block_timestamp: u64,
    end_l1_l2_bridge_rolling_hash: [32]u8,
    end_l1_l2_bridge_rolling_hash_message_number: u64,
    dynamic_chain_config_hash: [32]u8,
    parent_ftx_rolling_hash: [32]u8,
    parent_ftx_number: u64,
    end_ftx_rolling_hash: [32]u8,
    end_processed_ftx_number: u64,
    parent_data_rolling_hash: [32]u8,
    end_data_rolling_hash: [32]u8,
    parent_block_hash: [32]u8,
    end_block_hash: [32]u8,
    parent_shared_blob_carry_over_tail_bytes: u64,
    final_shared_blob_carry_over_tail_bytes: u64,
    l2_l1_tree_depth: u64,
    l2_l1_roots: ssz.FixedList([32]u8, MAX_L2_L1_ROOTS),
    filtered_addresses: ssz.FixedList([20]u8, rollup_proof.MAX_FILTERED_ADDRESSES),
    program_ids: ssz.FixedList([32]u8, rollup_proof.MAX_PROGRAM_VKS),
    l2_messaging_blocks_offsets: ssz.FixedList(u16, rollup_proof.MAX_MESSAGING_BLOCKS_OFFSETS),
};

/// The input in `data`, the 0x0006 schema id followed by the SSZ container.
pub fn decodeInput(alloc: Allocator, data: []const u8) ssz.FramedDecodeError!RollupAggregationInput {
    return ssz.decodeFramed(RollupAggregationInput, alloc, data, INPUT_SCHEMA_ID);
}

/// The frame `decodeInput` reads.
pub fn encodeInput(alloc: Allocator, input: RollupAggregationInput) ssz.EncodeError![]u8 {
    return ssz.encodeFramed(RollupAggregationInput, alloc, INPUT_SCHEMA_ID, input);
}

/// The 0x0007 output carrying `pi`.
pub fn encodeOutput(alloc: Allocator, pi: FinalizationPublicInput) ssz.EncodeError![]u8 {
    return output_frame.encode(FinalizationPublicInput, alloc, OUTPUT_SCHEMA_ID, pi);
}

/// The public inputs of a 0x0007 output; a failing commitment or schema id check is
/// `error.MalformedFrame`.
pub fn decodeOutput(alloc: Allocator, bytes: []const u8) (error{MalformedFrame} || ssz.DecodeError)!FinalizationPublicInput {
    return ssz.decode(FinalizationPublicInput, alloc, try output_frame.decode(bytes, OUTPUT_SCHEMA_ID));
}
