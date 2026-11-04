//! Wire contract of an l2-execution proof: the public-input tuple, the proof that carries it with
//! its range start and proof bytes, and the verifiable proof that adds the program VK. The layouts
//! are SSZ and decoders accept only the canonical encoding; byte-list fields alias the decoded buffer.
//!
//! The l2-execution guest's own output frames the public inputs with the 0x0003 schema id and leads
//! with their keccak256 commitment: `keccak256(frame) || frame`, `frame = schema_id || SSZ(public inputs)`.
//!
//! Fixed-head sizes (variable-size fields sit behind 4-byte offsets):
//!   L2ExecutionProofPublicInput:  284 bytes
//!   L2ExecutionProof:              16 bytes
//!   VerifiableL2ExecutionProof:    36 bytes

const std = @import("std");
const ssz = @import("guest_common").ssz;

pub const OUTPUT_SCHEMA_ID: u16 = 0x0003;
const SCHEMA_ID_SIZE: usize = 2;
const HASH_SIZE: usize = 32;

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
    l2_l1_messages: []const [32]u8,
    end_l1_l2_bridge_rolling_hash: [32]u8,
    end_l1_l2_bridge_rolling_hash_message_number: u64,
    dynamic_chain_config_hash: [32]u8,
    parent_ftx_rolling_hash: [32]u8,
    parent_ftx_number: u64,
    end_ftx_rolling_hash: [32]u8,
    end_processed_ftx_number: u64,
    filtered_addresses: []const [20]u8,
    tx_froms_hash: [32]u8,
    block_count: u64,
    l2_messaging_blocks_offsets: []const u16,
};

pub const L2ExecutionProof = struct {
    public_inputs: L2ExecutionProofPublicInput,
    start_block_number: u64,
    proof: []const u8,
};

pub const VerifiableL2ExecutionProof = struct {
    proof: L2ExecutionProof,
    program_vk: [32]u8,
};

const readU32 = ssz.readU32;
const readU64 = ssz.readU64;
const writeU32 = ssz.writeU32;
const writeU64 = ssz.writeU64;

// ── Cursor reads/writes over the fixed head ───────────────────────────────────

/// Write a 32-byte hash at the cursor and advance it.
inline fn putHash(out: []u8, pos: *usize, value: [32]u8) void {
    @memcpy(out[pos.*..][0..32], &value);
    pos.* += 32;
}

/// Write a little-endian u64 at the cursor and advance it.
inline fn putU64(out: []u8, pos: *usize, value: u64) void {
    writeU64(out, pos.*, value);
    pos.* += 8;
}

/// Write the offset of a variable-size field's tail at the cursor and advance it.
inline fn putOffset(out: []u8, pos: *usize, tail_start: usize) void {
    writeU32(out, pos.*, @intCast(tail_start));
    pos.* += 4;
}

/// Read a 32-byte hash at the cursor and advance it.
inline fn getHash(bytes: []const u8, pos: *usize) [32]u8 {
    defer pos.* += 32;
    return bytes[pos.*..][0..32].*;
}

/// Read a little-endian u64 at the cursor and advance it.
inline fn getU64(bytes: []const u8, pos: *usize) u64 {
    defer pos.* += 8;
    return readU64(bytes, pos.*);
}

/// Read the offset of a variable-size field's tail at the cursor and advance it.
inline fn getOffset(bytes: []const u8, pos: *usize) usize {
    defer pos.* += 4;
    return readU32(bytes, pos.*);
}

/// Alias a region of fixed-size byte-array elements after checking its length against the element size and the bound.
fn decodeFixedList(comptime T: type, region: []const u8, max_items: usize) ![]const T {
    if (region.len % @sizeOf(T) != 0) return error.InvalidSsz;
    if (region.len / @sizeOf(T) > max_items) return error.BoundsViolation;
    return std.mem.bytesAsSlice(T, region);
}

// ── L2ExecutionProofPublicInput ───────────────────────────────────────────────
// Fixed head: the fixed-size fields inline plus a 4-byte offset per list = 284.
const PI_FIXED_SIZE: usize = 284;

/// SSZ-encode the public-input tuple: the 284-byte fixed head followed by the three list tails.
pub fn encodePublicInputs(alloc: std.mem.Allocator, pi: L2ExecutionProofPublicInput) ![]u8 {
    if (pi.l2_l1_messages.len > MAX_MESSAGES or
        pi.filtered_addresses.len > MAX_FILTERED or
        pi.l2_messaging_blocks_offsets.len > MAX_MESSAGING_BLOCKS) return error.BoundsViolation;

    const messages_len = pi.l2_l1_messages.len * 32;
    const filtered_len = pi.filtered_addresses.len * 20;
    const out = try alloc.alloc(u8, PI_FIXED_SIZE + messages_len + filtered_len + pi.l2_messaging_blocks_offsets.len * 2);

    const messages_start = PI_FIXED_SIZE;
    const filtered_start = messages_start + messages_len;
    const blocks_start = filtered_start + filtered_len;

    var pos: usize = 0;
    putHash(out, &pos, pi.parent_block_hash);
    putHash(out, &pos, pi.end_block_hash);
    putU64(out, &pos, pi.end_block_number);
    putU64(out, &pos, pi.end_block_timestamp);
    putOffset(out, &pos, messages_start);
    putHash(out, &pos, pi.end_l1_l2_bridge_rolling_hash);
    putU64(out, &pos, pi.end_l1_l2_bridge_rolling_hash_message_number);
    putHash(out, &pos, pi.dynamic_chain_config_hash);
    putHash(out, &pos, pi.parent_ftx_rolling_hash);
    putU64(out, &pos, pi.parent_ftx_number);
    putHash(out, &pos, pi.end_ftx_rolling_hash);
    putU64(out, &pos, pi.end_processed_ftx_number);
    putOffset(out, &pos, filtered_start);
    putHash(out, &pos, pi.tx_froms_hash);
    putU64(out, &pos, pi.block_count);
    putOffset(out, &pos, blocks_start);
    std.debug.assert(pos == PI_FIXED_SIZE);

    for (pi.l2_l1_messages, 0..) |m, i| @memcpy(out[messages_start + i * 32 ..][0..32], &m);
    for (pi.filtered_addresses, 0..) |a, i| @memcpy(out[filtered_start + i * 20 ..][0..20], &a);
    for (pi.l2_messaging_blocks_offsets, 0..) |o, i| std.mem.writeInt(u16, out[blocks_start + i * 2 ..][0..2], o, .little);
    return out;
}

/// Decode the SSZ public-input tuple. Inverse of `encodePublicInputs`; the message and address lists alias `bytes`.
pub fn decodePublicInputs(alloc: std.mem.Allocator, bytes: []const u8) !L2ExecutionProofPublicInput {
    if (bytes.len < PI_FIXED_SIZE) return error.InvalidSsz;

    var pos: usize = 0;
    const parent_block_hash = getHash(bytes, &pos);
    const end_block_hash = getHash(bytes, &pos);
    const end_block_number = getU64(bytes, &pos);
    const end_block_timestamp = getU64(bytes, &pos);
    const messages_start = getOffset(bytes, &pos);
    const end_l1_l2_bridge_rolling_hash = getHash(bytes, &pos);
    const end_l1_l2_bridge_rolling_hash_message_number = getU64(bytes, &pos);
    const dynamic_chain_config_hash = getHash(bytes, &pos);
    const parent_ftx_rolling_hash = getHash(bytes, &pos);
    const parent_ftx_number = getU64(bytes, &pos);
    const end_ftx_rolling_hash = getHash(bytes, &pos);
    const end_processed_ftx_number = getU64(bytes, &pos);
    const filtered_start = getOffset(bytes, &pos);
    const tx_froms_hash = getHash(bytes, &pos);
    const block_count = getU64(bytes, &pos);
    const blocks_start = getOffset(bytes, &pos);
    std.debug.assert(pos == PI_FIXED_SIZE);

    // The first tail starts right behind the fixed head and every tail ends where the next begins.
    if (messages_start != PI_FIXED_SIZE or filtered_start < messages_start or
        blocks_start < filtered_start or blocks_start > bytes.len) return error.InvalidSsz;

    const l2_l1_messages = try decodeFixedList([32]u8, bytes[messages_start..filtered_start], MAX_MESSAGES);
    const filtered_addresses = try decodeFixedList([20]u8, bytes[filtered_start..blocks_start], MAX_FILTERED);
    const blocks_region = bytes[blocks_start..];
    if (blocks_region.len % 2 != 0) return error.InvalidSsz;
    if (blocks_region.len / 2 > MAX_MESSAGING_BLOCKS) return error.BoundsViolation;
    const l2_messaging_blocks_offsets = try alloc.alloc(u16, blocks_region.len / 2);
    for (l2_messaging_blocks_offsets, 0..) |*o, i| o.* = std.mem.readInt(u16, blocks_region[i * 2 ..][0..2], .little);

    return .{
        .parent_block_hash = parent_block_hash,
        .end_block_hash = end_block_hash,
        .end_block_number = end_block_number,
        .end_block_timestamp = end_block_timestamp,
        .l2_l1_messages = l2_l1_messages,
        .end_l1_l2_bridge_rolling_hash = end_l1_l2_bridge_rolling_hash,
        .end_l1_l2_bridge_rolling_hash_message_number = end_l1_l2_bridge_rolling_hash_message_number,
        .dynamic_chain_config_hash = dynamic_chain_config_hash,
        .parent_ftx_rolling_hash = parent_ftx_rolling_hash,
        .parent_ftx_number = parent_ftx_number,
        .end_ftx_rolling_hash = end_ftx_rolling_hash,
        .end_processed_ftx_number = end_processed_ftx_number,
        .filtered_addresses = filtered_addresses,
        .tx_froms_hash = tx_froms_hash,
        .block_count = block_count,
        .l2_messaging_blocks_offsets = l2_messaging_blocks_offsets,
    };
}

// ── L2ExecutionProof ──────────────────────────────────────────────────────────
// Fixed head: public_inputs offset(4) + start_block_number(8) + proof offset(4) = 16.
const PROOF_FIXED_SIZE: usize = 16;

pub fn encodeProof(alloc: std.mem.Allocator, v: L2ExecutionProof) ![]u8 {
    if (v.proof.len > MAX_PROOF_BYTES) return error.BoundsViolation;
    const public_inputs = try encodePublicInputs(alloc, v.public_inputs);
    defer alloc.free(public_inputs);

    const out = try alloc.alloc(u8, PROOF_FIXED_SIZE + public_inputs.len + v.proof.len);
    writeU32(out, 0, @intCast(PROOF_FIXED_SIZE));
    writeU64(out, 4, v.start_block_number);
    writeU32(out, 12, @intCast(PROOF_FIXED_SIZE + public_inputs.len));
    @memcpy(out[PROOF_FIXED_SIZE..][0..public_inputs.len], public_inputs);
    @memcpy(out[PROOF_FIXED_SIZE + public_inputs.len ..], v.proof);
    return out;
}

/// Decode an `L2ExecutionProof`. Inverse of `encodeProof`; `proof` aliases `bytes`.
pub fn decodeProof(alloc: std.mem.Allocator, bytes: []const u8) !L2ExecutionProof {
    if (bytes.len < PROOF_FIXED_SIZE) return error.InvalidSsz;
    const off_public_inputs = readU32(bytes, 0);
    const start_block_number = readU64(bytes, 4);
    const off_proof = readU32(bytes, 12);
    if (off_public_inputs != PROOF_FIXED_SIZE or off_proof < off_public_inputs or off_proof > bytes.len) return error.InvalidSsz;

    const proof = bytes[off_proof..];
    if (proof.len > MAX_PROOF_BYTES) return error.BoundsViolation;
    return .{
        .public_inputs = try decodePublicInputs(alloc, bytes[off_public_inputs..off_proof]),
        .start_block_number = start_block_number,
        .proof = proof,
    };
}

// ── VerifiableL2ExecutionProof ────────────────────────────────────────────────
// Fixed head: proof offset(4) + program_vk(32) = 36.
const VERIFIABLE_FIXED_SIZE: usize = 36;

pub fn encodeVerifiableProof(alloc: std.mem.Allocator, v: VerifiableL2ExecutionProof) ![]u8 {
    const proof = try encodeProof(alloc, v.proof);
    defer alloc.free(proof);

    const out = try alloc.alloc(u8, VERIFIABLE_FIXED_SIZE + proof.len);
    writeU32(out, 0, @intCast(VERIFIABLE_FIXED_SIZE));
    @memcpy(out[4..VERIFIABLE_FIXED_SIZE], &v.program_vk);
    @memcpy(out[VERIFIABLE_FIXED_SIZE..], proof);
    return out;
}

/// Decode a `VerifiableL2ExecutionProof`. Inverse of `encodeVerifiableProof`; `proof.proof` aliases `bytes`.
pub fn decodeVerifiableProof(alloc: std.mem.Allocator, bytes: []const u8) !VerifiableL2ExecutionProof {
    if (bytes.len < VERIFIABLE_FIXED_SIZE) return error.InvalidSsz;
    if (readU32(bytes, 0) != VERIFIABLE_FIXED_SIZE) return error.InvalidSsz;
    return .{
        .proof = try decodeProof(alloc, bytes[VERIFIABLE_FIXED_SIZE..]),
        .program_vk = bytes[4..VERIFIABLE_FIXED_SIZE][0..32].*,
    };
}

// ── Guest output ──────────────────────────────────────────────────────────────

/// Encode the 0x0003 output: `keccak256(schema_id || ssz) || schema_id || ssz`.
pub fn encodeOutput(alloc: std.mem.Allocator, pi: L2ExecutionProofPublicInput) ![]u8 {
    const body = try encodePublicInputs(alloc, pi);
    defer alloc.free(body);

    const out = try alloc.alloc(u8, HASH_SIZE + SCHEMA_ID_SIZE + body.len);
    const frame = out[HASH_SIZE..];
    std.mem.writeInt(u16, frame[0..2], OUTPUT_SCHEMA_ID, .big);
    @memcpy(frame[SCHEMA_ID_SIZE..], body);
    std.crypto.hash.sha3.Keccak256.hash(frame, out[0..HASH_SIZE], .{});
    return out;
}

/// keccak256 of the schema-framed SSZ public-input tuple: the commitment at the head of the output.
pub fn hashPublicInputs(alloc: std.mem.Allocator, pi: L2ExecutionProofPublicInput) ![32]u8 {
    const out = try encodeOutput(alloc, pi);
    defer alloc.free(out);
    return out[0..HASH_SIZE].*;
}
