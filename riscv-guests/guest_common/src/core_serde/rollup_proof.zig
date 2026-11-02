//! Wire contract of a rollup proof's public inputs: the tuple the rollup guest commits to. The
//! layout is SSZ and the decoders accept only the canonical encoding; the message, address and
//! verifying-key lists alias the decoded buffer.
//!
//! The rollup guest's own output frames the public inputs with the 0x1801 schema id and leads with
//! their keccak256 commitment: `keccak256(frame) || frame`, `frame = schema_id || SSZ(public inputs)`.
//!
//! Fixed-head size (variable-size fields sit behind 4-byte offsets):
//!   RollupPublicInput:  336 bytes

const std = @import("std");
const ssz = @import("guest_common").ssz;
const l2_execution_proof = @import("l2_execution_proof.zig");
const output_frame = @import("output_frame.zig");

pub const OUTPUT_SCHEMA_ID: u16 = 0x1801;

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
    l2_l1_messages: []const [32]u8,
    filtered_addresses: []const [20]u8,
    program_vks: []const [32]u8,
    block_count: u64,
    l2_messaging_blocks_offsets: []const u16,
};

const putHash = ssz.putHash;
const putU64 = ssz.putU64;
const putOffset = ssz.putOffset;
const getHash = ssz.getHash;
const getU64 = ssz.getU64;
const getOffset = ssz.getOffset;

// ── RollupPublicInput ─────────────────────────────────────────────────────────
// Fixed head: the fixed-size fields inline plus a 4-byte offset per list = 336.
const PI_FIXED_SIZE: usize = 336;

/// SSZ-encode the public-input tuple: the 336-byte fixed head followed by the four list tails.
pub fn encodePublicInputs(alloc: std.mem.Allocator, pi: RollupPublicInput) ![]u8 {
    if (pi.l2_l1_messages.len > MAX_L2_L1_MESSAGES or
        pi.filtered_addresses.len > MAX_FILTERED_ADDRESSES or
        pi.program_vks.len > MAX_PROGRAM_VKS or
        pi.l2_messaging_blocks_offsets.len > MAX_MESSAGING_BLOCKS_OFFSETS) return error.BoundsViolation;

    const messages_len = pi.l2_l1_messages.len * 32;
    const filtered_len = pi.filtered_addresses.len * 20;
    const vks_len = pi.program_vks.len * 32;
    const out = try alloc.alloc(u8, PI_FIXED_SIZE + messages_len + filtered_len + vks_len + pi.l2_messaging_blocks_offsets.len * 2);

    const messages_start = PI_FIXED_SIZE;
    const filtered_start = messages_start + messages_len;
    const vks_start = filtered_start + filtered_len;
    const offsets_start = vks_start + vks_len;

    var pos: usize = 0;
    putU64(out, &pos, pi.end_block_number);
    putU64(out, &pos, pi.end_block_timestamp);
    putHash(out, &pos, pi.end_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, pi.end_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, pi.dynamic_chain_config_hash);
    putHash(out, &pos, pi.parent_ftx_rolling_hash);
    putU64(out, &pos, pi.parent_ftx_number);
    putHash(out, &pos, pi.end_ftx_rolling_hash);
    putU64(out, &pos, pi.end_processed_ftx_number);
    putHash(out, &pos, pi.parent_data_rolling_hash);
    putHash(out, &pos, pi.end_data_rolling_hash);
    putHash(out, &pos, pi.parent_block_hash);
    putHash(out, &pos, pi.end_block_hash);
    putU64(out, &pos, pi.parent_data_tail_take_bytes);
    putU64(out, &pos, pi.final_data_tail_discard_bytes);
    putOffset(out, &pos, messages_start);
    putOffset(out, &pos, filtered_start);
    putOffset(out, &pos, vks_start);
    putU64(out, &pos, pi.block_count);
    putOffset(out, &pos, offsets_start);
    std.debug.assert(pos == PI_FIXED_SIZE);

    for (pi.l2_l1_messages, 0..) |m, i| @memcpy(out[messages_start + i * 32 ..][0..32], &m);
    for (pi.filtered_addresses, 0..) |a, i| @memcpy(out[filtered_start + i * 20 ..][0..20], &a);
    for (pi.program_vks, 0..) |vk, i| @memcpy(out[vks_start + i * 32 ..][0..32], &vk);
    ssz.writeU16List(out[offsets_start..], pi.l2_messaging_blocks_offsets);
    return out;
}

/// Decode the SSZ public-input tuple. Inverse of `encodePublicInputs`; the message, address and
/// verifying-key lists alias `bytes`.
pub fn decodePublicInputs(alloc: std.mem.Allocator, bytes: []const u8) !RollupPublicInput {
    if (bytes.len < PI_FIXED_SIZE) return error.InvalidSsz;

    var pos: usize = 0;
    const end_block_number = getU64(bytes, &pos);
    const end_block_timestamp = getU64(bytes, &pos);
    const end_l1_l2_bridge_rolling_hash = getHash(bytes, &pos);
    const end_l1_l2_bridge_rolling_hash_message_number = getU64(bytes, &pos);
    const dynamic_chain_config_hash = getHash(bytes, &pos);
    const parent_ftx_rolling_hash = getHash(bytes, &pos);
    const parent_ftx_number = getU64(bytes, &pos);
    const end_ftx_rolling_hash = getHash(bytes, &pos);
    const end_processed_ftx_number = getU64(bytes, &pos);
    const parent_data_rolling_hash = getHash(bytes, &pos);
    const end_data_rolling_hash = getHash(bytes, &pos);
    const parent_block_hash = getHash(bytes, &pos);
    const end_block_hash = getHash(bytes, &pos);
    const parent_data_tail_take_bytes = getU64(bytes, &pos);
    const final_data_tail_discard_bytes = getU64(bytes, &pos);
    const messages_start = getOffset(bytes, &pos);
    const filtered_start = getOffset(bytes, &pos);
    const vks_start = getOffset(bytes, &pos);
    const block_count = getU64(bytes, &pos);
    const offsets_start = getOffset(bytes, &pos);
    std.debug.assert(pos == PI_FIXED_SIZE);

    // The first tail starts right behind the fixed head and every tail ends where the next begins.
    if (messages_start != PI_FIXED_SIZE or filtered_start < messages_start or vks_start < filtered_start or
        offsets_start < vks_start or offsets_start > bytes.len) return error.InvalidSsz;

    const l2_l1_messages = try ssz.decodeFixedList([32]u8, bytes[messages_start..filtered_start], MAX_L2_L1_MESSAGES);
    const filtered_addresses = try ssz.decodeFixedList([20]u8, bytes[filtered_start..vks_start], MAX_FILTERED_ADDRESSES);
    const program_vks = try ssz.decodeFixedList([32]u8, bytes[vks_start..offsets_start], MAX_PROGRAM_VKS);
    const l2_messaging_blocks_offsets = try ssz.decodeU16List(alloc, bytes[offsets_start..], MAX_MESSAGING_BLOCKS_OFFSETS);

    return .{
        .end_block_number = end_block_number,
        .end_block_timestamp = end_block_timestamp,
        .end_l1_l2_bridge_rolling_hash = end_l1_l2_bridge_rolling_hash,
        .end_l1_l2_bridge_rolling_hash_message_number = end_l1_l2_bridge_rolling_hash_message_number,
        .dynamic_chain_config_hash = dynamic_chain_config_hash,
        .parent_ftx_rolling_hash = parent_ftx_rolling_hash,
        .parent_ftx_number = parent_ftx_number,
        .end_ftx_rolling_hash = end_ftx_rolling_hash,
        .end_processed_ftx_number = end_processed_ftx_number,
        .parent_data_rolling_hash = parent_data_rolling_hash,
        .end_data_rolling_hash = end_data_rolling_hash,
        .parent_block_hash = parent_block_hash,
        .end_block_hash = end_block_hash,
        .parent_data_tail_take_bytes = parent_data_tail_take_bytes,
        .final_data_tail_discard_bytes = final_data_tail_discard_bytes,
        .l2_l1_messages = l2_l1_messages,
        .filtered_addresses = filtered_addresses,
        .program_vks = program_vks,
        .block_count = block_count,
        .l2_messaging_blocks_offsets = l2_messaging_blocks_offsets,
    };
}

// ── Guest output ──────────────────────────────────────────────────────────────

/// Encode the 0x1801 output: `keccak256(schema_id || ssz) || schema_id || ssz`.
pub fn encodeOutput(alloc: std.mem.Allocator, pi: RollupPublicInput) ![]u8 {
    const body = try encodePublicInputs(alloc, pi);
    defer alloc.free(body);
    return output_frame.encode(alloc, OUTPUT_SCHEMA_ID, body);
}

/// Decode a 0x1801 output into its public inputs. The leading commitment is the keccak256 of the
/// frame and the frame leads with the schema id; a failing check is `error.MalformedFrame`.
pub fn decodeOutput(alloc: std.mem.Allocator, bytes: []const u8) !RollupPublicInput {
    return decodePublicInputs(alloc, try output_frame.decode(bytes, OUTPUT_SCHEMA_ID));
}

/// keccak256 of the schema-framed SSZ public-input tuple: the commitment at the head of the output.
pub fn hashPublicInputs(alloc: std.mem.Allocator, pi: RollupPublicInput) ![32]u8 {
    const body = try encodePublicInputs(alloc, pi);
    defer alloc.free(body);
    return output_frame.commitment(OUTPUT_SCHEMA_ID, body);
}
