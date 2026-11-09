//! Wire contract of an l2-execution proof: the public-input tuple, the proof that carries it with
//! its range start and proof bytes, and the verifiable proof that adds the program VK. The layouts
//! are SSZ and decoders accept only the canonical encoding; the lists alias the decoded buffer.
//!
//! The l2-execution guest's own output frames the public inputs with the 0x0003 schema id and leads
//! with their keccak256 commitment: `keccak256(frame) || frame`, `frame = schema_id || SSZ(public inputs)`.

const std = @import("std");
const ssz = @import("guest_common").ssz;
const output_frame = @import("output_frame.zig");

pub const OUTPUT_SCHEMA_ID: u16 = 0x0003;

// List bounds of the wire schema: encoders and decoders reject longer lists.
/// Blocks (one payload each) in one l2-execution proof range; the guest input's payload list
/// shares this bound.
pub const MAX_PAYLOADS: usize = 1 << 16;
pub const MAX_MESSAGES: usize = 1 << 16;
pub const MAX_FILTERED: usize = 1 << 16;
pub const MAX_MESSAGING_BLOCKS: usize = MAX_PAYLOADS;
pub const MAX_PROOF_BYTES: usize = 1 << 24;

/// The l2-execution public input tuple, in wire order.
pub const L2ExecutionProofPublicInput = struct {
    parent_block_hash: [32]u8,
    end_block_hash: [32]u8,
    end_block_number: u64,
    end_block_timestamp: u64,
    l2_l1_messages: ssz.FixedList([32]u8, MAX_MESSAGES),
    end_l1_l2_bridge_rolling_hash: [32]u8,
    end_l1_l2_bridge_rolling_hash_message_number: u64,
    dynamic_chain_config_hash: [32]u8,
    parent_ftx_rolling_hash: [32]u8,
    parent_ftx_number: u64,
    end_ftx_rolling_hash: [32]u8,
    end_processed_ftx_number: u64,
    filtered_addresses: ssz.FixedList([20]u8, MAX_FILTERED),
    tx_froms_hash: [32]u8,
    block_count: u64,
    l2_messaging_blocks_offsets: ssz.FixedList(u16, MAX_MESSAGING_BLOCKS),
};

pub const L2ExecutionProof = struct {
    public_inputs: L2ExecutionProofPublicInput,
    start_block_number: u64,
    proof: ssz.ByteList(MAX_PROOF_BYTES),
};

pub const VerifiableL2ExecutionProof = struct {
    proof: L2ExecutionProof,
    program_vk: [32]u8,
};

pub fn encodePublicInputs(alloc: std.mem.Allocator, pi: L2ExecutionProofPublicInput) ssz.EncodeError![]u8 {
    return ssz.encode(L2ExecutionProofPublicInput, alloc, pi);
}

pub fn decodePublicInputs(alloc: std.mem.Allocator, bytes: []const u8) ssz.DecodeError!L2ExecutionProofPublicInput {
    return ssz.decode(L2ExecutionProofPublicInput, alloc, bytes);
}

pub fn encodeProof(alloc: std.mem.Allocator, v: L2ExecutionProof) ssz.EncodeError![]u8 {
    return ssz.encode(L2ExecutionProof, alloc, v);
}

pub fn decodeProof(alloc: std.mem.Allocator, bytes: []const u8) ssz.DecodeError!L2ExecutionProof {
    return ssz.decode(L2ExecutionProof, alloc, bytes);
}

pub fn encodeVerifiableProof(alloc: std.mem.Allocator, v: VerifiableL2ExecutionProof) ssz.EncodeError![]u8 {
    return ssz.encode(VerifiableL2ExecutionProof, alloc, v);
}

pub fn decodeVerifiableProof(alloc: std.mem.Allocator, bytes: []const u8) ssz.DecodeError!VerifiableL2ExecutionProof {
    return ssz.decode(VerifiableL2ExecutionProof, alloc, bytes);
}

// ── Guest output ──────────────────────────────────────────────────────────────

/// Encode the 0x0003 output: `keccak256(schema_id || ssz) || schema_id || ssz`.
pub fn encodeOutput(alloc: std.mem.Allocator, pi: L2ExecutionProofPublicInput) ssz.EncodeError![]u8 {
    return output_frame.encode(L2ExecutionProofPublicInput, alloc, OUTPUT_SCHEMA_ID, pi);
}

/// keccak256 of the schema-framed SSZ public-input tuple: the commitment at the head of the output.
pub fn hashPublicInputs(alloc: std.mem.Allocator, pi: L2ExecutionProofPublicInput) ssz.EncodeError![32]u8 {
    return output_frame.commitment(L2ExecutionProofPublicInput, alloc, OUTPUT_SCHEMA_ID, pi);
}
