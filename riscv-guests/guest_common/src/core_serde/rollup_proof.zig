//! Wire contract of a rollup proof's public inputs: the tuple the rollup guest commits to. The
//! layout is SSZ and the decoders accept only the canonical encoding; the lists alias the decoded buffer.
//!
//! The rollup guest's own output frames the public inputs with the 0x0005 schema id and leads with
//! their keccak256 commitment: `keccak256(frame) || frame`, `frame = schema_id || SSZ(public inputs)`.

const std = @import("std");
const ssz = @import("guest_common").ssz;
const l2_execution_proof = @import("l2_execution_proof.zig");
const output_frame = @import("output_frame.zig");

pub const OUTPUT_SCHEMA_ID: u16 = 0x0005;

// List bounds of the wire schema: encoders and decoders reject longer lists.
/// L2-execution proofs, one per conflation, in one rollup proof.
pub const MAX_L2_EXECUTION_PROOFS_PER_ROLLUP: usize = 1 << 10;
/// The messages of every execution proof of one rollup proof.
pub const MAX_L2_L1_MESSAGES: usize = l2_execution_proof.MAX_MESSAGES * MAX_L2_EXECUTION_PROOFS_PER_ROLLUP;
pub const MAX_FILTERED_ADDRESSES: usize = 1 << 16;
pub const MAX_PROGRAM_VKS: usize = 1 << 10;
pub const MAX_MESSAGING_BLOCKS_OFFSETS: usize = 1 << 16;

/// The rollup public input tuple, in wire order.
pub const RollupPublicInput = struct {
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
    parent_data_tail_take_bytes: u64,
    final_data_tail_discard_bytes: u64,
    l2_l1_messages: ssz.FixedList([32]u8, MAX_L2_L1_MESSAGES),
    filtered_addresses: ssz.FixedList([20]u8, MAX_FILTERED_ADDRESSES),
    program_vks: ssz.FixedList([32]u8, MAX_PROGRAM_VKS),
    block_count: u64,
    l2_messaging_blocks_offsets: ssz.FixedList(u16, MAX_MESSAGING_BLOCKS_OFFSETS),
};

pub fn encodePublicInputs(alloc: std.mem.Allocator, pi: RollupPublicInput) ssz.EncodeError![]u8 {
    return ssz.encode(RollupPublicInput, alloc, pi);
}

pub fn decodePublicInputs(alloc: std.mem.Allocator, bytes: []const u8) ssz.DecodeError!RollupPublicInput {
    return ssz.decode(RollupPublicInput, alloc, bytes);
}

// ── Guest output ──────────────────────────────────────────────────────────────

/// Encode the 0x0005 output: `keccak256(schema_id || ssz) || schema_id || ssz`.
pub fn encodeOutput(alloc: std.mem.Allocator, pi: RollupPublicInput) ssz.EncodeError![]u8 {
    return output_frame.encode(RollupPublicInput, alloc, OUTPUT_SCHEMA_ID, pi);
}

/// Decode a 0x0005 output into its public inputs. The leading commitment is the keccak256 of the
/// frame and the frame leads with the schema id; a failing check is `error.MalformedFrame`.
pub fn decodeOutput(alloc: std.mem.Allocator, bytes: []const u8) (error{MalformedFrame} || ssz.DecodeError)!RollupPublicInput {
    return decodePublicInputs(alloc, try output_frame.decode(bytes, OUTPUT_SCHEMA_ID));
}

/// keccak256 of the schema-framed SSZ public-input tuple: the commitment at the head of the output.
pub fn hashPublicInputs(alloc: std.mem.Allocator, pi: RollupPublicInput) ssz.EncodeError![32]u8 {
    return output_frame.commitment(RollupPublicInput, alloc, OUTPUT_SCHEMA_ID, pi);
}
