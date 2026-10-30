//! Tests for the shared l2-execution proof codec: byte-exact agreement with the reference codec,
//! round-trips through the public decoders, and rejection of every non-canonical encoding.

const std = @import("std");

const l2_execution_proof = @import("core_serde").l2_execution_proof;

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

const sample_messages = [_][32]u8{repeat32(0x08)};
const sample_filtered = [_][20]u8{repeat20(0x09)};
const sample_offsets = [_]u16{ 1, 3 };

fn samplePublicInputs() l2_execution_proof.L2ExecutionProofPublicInput {
    return .{
        .parent_block_hash = repeat32(0x0a),
        .end_block_hash = repeat32(0x0b),
        .end_block_number = 1000503,
        .end_block_timestamp = 1763000123,
        .l2_l1_messages = &sample_messages,
        .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
        .end_l1_l2_bridge_rolling_hash_message_number = 5,
        .dynamic_chain_config_hash = repeat32(0xc0),
        .parent_ftx_rolling_hash = repeat32(0x04),
        .parent_ftx_number = 16,
        .end_ftx_rolling_hash = repeat32(0x05),
        .end_processed_ftx_number = 18,
        .filtered_addresses = &sample_filtered,
        .tx_froms_hash = repeat32(0x07),
        .block_count = 3,
        .l2_messaging_blocks_offsets = &sample_offsets,
    };
}

const sample_proof_bytes = [_]u8{ 0x50, 0x52, 0x4f, 0x4f, 0x46 };

fn sampleVerifiableProof() l2_execution_proof.VerifiableL2ExecutionProof {
    return .{
        .proof = .{
            .public_inputs = samplePublicInputs(),
            .start_block_number = 1000501,
            .proof = &sample_proof_bytes,
        },
        .program_vk = repeat32(0x77),
    };
}

// Produced by the Python reference codec's `encode_l2_execution_output` for `samplePublicInputs`.
const sample_output_hex =
    "9fba64a1911307d769659b8ecb64c72d0df2d90d8158241267c55d37bb480030" ++
    "00030a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a" ++
    "0a0a0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b" ++
    "0b0b37440f00000000003b3f1569000000001c01000003030303030303030303" ++
    "030303030303030303030303030303030303030303030500000000000000c0c0" ++
    "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c00404" ++
    "0404040404040404040404040404040404040404040404040404040404041000" ++
    "0000000000000505050505050505050505050505050505050505050505050505" ++
    "05050505050512000000000000003c0100000707070707070707070707070707" ++
    "0707070707070707070707070707070707070300000000000000500100000808" ++
    "0808080808080808080808080808080808080808080808080808080808080909" ++
    "09090909090909090909090909090909090901000300";

// Produced by the Python reference codec's `SszVerifiableL2ExecutionProof.encode_bytes` for
// `sampleVerifiableProof`: 36-byte head (proof offset, program_vk), then the 16-byte proof head, the
// 340-byte public inputs and the 5 proof bytes.
const sample_verifiable_hex =
    "2400000077777777777777777777777777777777777777777777777777777777" ++
    "777777771000000035440f0000000000640100000a0a0a0a0a0a0a0a0a0a0a0a" ++
    "0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0a0b0b0b0b0b0b0b0b0b0b0b0b" ++
    "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b37440f00000000003b3f1569" ++
    "000000001c010000030303030303030303030303030303030303030303030303" ++
    "03030303030303030500000000000000c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0" ++
    "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c004040404040404040404040404040404" ++
    "0404040404040404040404040404040410000000000000000505050505050505" ++
    "0505050505050505050505050505050505050505050505051200000000000000" ++
    "3c01000007070707070707070707070707070707070707070707070707070707" ++
    "0707070703000000000000005001000008080808080808080808080808080808" ++
    "0808080808080808080808080808080809090909090909090909090909090909" ++
    "090909090100030050524f4f46";

const verifiable_head_size = 36;
const proof_head_size = 16;
const public_inputs_head_size = 284;

fn hexBytes(comptime hex: []const u8) [hex.len / 2]u8 {
    @setEvalBranchQuota(hex.len * 20);
    var out: [hex.len / 2]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

const sample_verifiable_bytes = hexBytes(sample_verifiable_hex);
const sample_proof_encoding = sample_verifiable_bytes[verifiable_head_size..];
const sample_public_inputs_encoding = sample_proof_encoding[proof_head_size .. sample_proof_encoding.len - sample_proof_bytes.len];

fn expectPublicInputsEqual(want: l2_execution_proof.L2ExecutionProofPublicInput, got: l2_execution_proof.L2ExecutionProofPublicInput) !void {
    try std.testing.expectEqualSlices(u8, &want.parent_block_hash, &got.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &want.end_block_hash, &got.end_block_hash);
    try std.testing.expectEqual(want.end_block_number, got.end_block_number);
    try std.testing.expectEqual(want.end_block_timestamp, got.end_block_timestamp);
    try std.testing.expectEqualSlices([32]u8, want.l2_l1_messages, got.l2_l1_messages);
    try std.testing.expectEqualSlices(u8, &want.end_l1_l2_bridge_rolling_hash, &got.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(want.end_l1_l2_bridge_rolling_hash_message_number, got.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &want.dynamic_chain_config_hash, &got.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &want.parent_ftx_rolling_hash, &got.parent_ftx_rolling_hash);
    try std.testing.expectEqual(want.parent_ftx_number, got.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &want.end_ftx_rolling_hash, &got.end_ftx_rolling_hash);
    try std.testing.expectEqual(want.end_processed_ftx_number, got.end_processed_ftx_number);
    try std.testing.expectEqualSlices([20]u8, want.filtered_addresses, got.filtered_addresses);
    try std.testing.expectEqualSlices(u8, &want.tx_froms_hash, &got.tx_froms_hash);
    try std.testing.expectEqual(want.block_count, got.block_count);
    try std.testing.expectEqualSlices(u16, want.l2_messaging_blocks_offsets, got.l2_messaging_blocks_offsets);
}

fn expectVerifiableProofEqual(want: l2_execution_proof.VerifiableL2ExecutionProof, got: l2_execution_proof.VerifiableL2ExecutionProof) !void {
    try expectPublicInputsEqual(want.proof.public_inputs, got.proof.public_inputs);
    try std.testing.expectEqual(want.proof.start_block_number, got.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, want.proof.proof, got.proof.proof);
    try std.testing.expectEqualSlices(u8, &want.program_vk, &got.program_vk);
}

// ── Guest output ──────────────────────────────────────────────────────────────

test "output is the schema-framed SSZ public inputs preceded by their hash, matching the Python reference" {
    const alloc = std.testing.allocator;
    const out = try l2_execution_proof.encodeOutput(alloc, samplePublicInputs());
    defer alloc.free(out);

    const expected = hexBytes(sample_output_hex);
    try std.testing.expectEqualSlices(u8, &expected, out);

    var frame_hash: [32]u8 = undefined;
    std.crypto.hash.sha3.Keccak256.hash(out[32..], &frame_hash, .{});
    try std.testing.expectEqualSlices(u8, &frame_hash, out[0..32]);
    try std.testing.expectEqual(l2_execution_proof.OUTPUT_SCHEMA_ID, std.mem.readInt(u16, out[32..34], .big));
}

test "output with empty lists is the 284-byte fixed head plus framing" {
    const alloc = std.testing.allocator;
    var pi = samplePublicInputs();
    pi.l2_l1_messages = &.{};
    pi.filtered_addresses = &.{};
    pi.l2_messaging_blocks_offsets = &.{};
    const out = try l2_execution_proof.encodeOutput(alloc, pi);
    defer alloc.free(out);
    try std.testing.expectEqual(@as(usize, 32 + 2 + public_inputs_head_size), out.len);
}

test "hashPublicInputs is the commitment at the head of the output" {
    const alloc = std.testing.allocator;
    const pi = samplePublicInputs();
    const out = try l2_execution_proof.encodeOutput(alloc, pi);
    defer alloc.free(out);
    const h = try l2_execution_proof.hashPublicInputs(alloc, pi);
    try std.testing.expectEqualSlices(u8, out[0..32], &h);
}

// ── Round-trips and agreement with the reference codec ───────────────────────

test "public inputs: encoding matches the reference bytes and decodes back to every field" {
    const alloc = std.testing.allocator;
    const encoded = try l2_execution_proof.encodePublicInputs(alloc, samplePublicInputs());
    defer alloc.free(encoded);
    try std.testing.expectEqualSlices(u8, sample_public_inputs_encoding, encoded);

    var arena = std.heap.ArenaAllocator.init(alloc);
    defer arena.deinit();
    const decoded = try l2_execution_proof.decodePublicInputs(arena.allocator(), encoded);
    try expectPublicInputsEqual(samplePublicInputs(), decoded);
}

test "public inputs with empty lists round-trip" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var pi = samplePublicInputs();
    pi.l2_l1_messages = &.{};
    pi.filtered_addresses = &.{};
    pi.l2_messaging_blocks_offsets = &.{};
    const encoded = try l2_execution_proof.encodePublicInputs(alloc, pi);
    try std.testing.expectEqual(@as(usize, public_inputs_head_size), encoded.len);
    try expectPublicInputsEqual(pi, try l2_execution_proof.decodePublicInputs(alloc, encoded));
}

test "verifiable proof: encoding matches the reference bytes and decodes back to every field" {
    const alloc = std.testing.allocator;
    const encoded = try l2_execution_proof.encodeVerifiableProof(alloc, sampleVerifiableProof());
    defer alloc.free(encoded);
    try std.testing.expectEqualSlices(u8, &sample_verifiable_bytes, encoded);

    var arena = std.heap.ArenaAllocator.init(alloc);
    defer arena.deinit();
    const decoded = try l2_execution_proof.decodeVerifiableProof(arena.allocator(), encoded);
    try expectVerifiableProofEqual(sampleVerifiableProof(), decoded);
}

test "proof: encoding is the verifiable proof's tail and decodes back to every field" {
    const alloc = std.testing.allocator;
    const encoded = try l2_execution_proof.encodeProof(alloc, sampleVerifiableProof().proof);
    defer alloc.free(encoded);
    try std.testing.expectEqualSlices(u8, sample_proof_encoding, encoded);

    var arena = std.heap.ArenaAllocator.init(alloc);
    defer arena.deinit();
    const decoded = try l2_execution_proof.decodeProof(arena.allocator(), encoded);
    try expectVerifiableProofEqual(sampleVerifiableProof(), .{ .proof = decoded, .program_vk = repeat32(0x77) });
}

test "proof with no proof bytes round-trips" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var v = sampleVerifiableProof();
    v.proof.proof = &.{};
    const decoded = try l2_execution_proof.decodeVerifiableProof(alloc, try l2_execution_proof.encodeVerifiableProof(alloc, v));
    try expectVerifiableProofEqual(v, decoded);
}

// ── Public inputs: malformed encodings ───────────────────────────────────────

fn decodePublicInputsOf(alloc: std.mem.Allocator, bytes: []const u8) !l2_execution_proof.L2ExecutionProofPublicInput {
    return l2_execution_proof.decodePublicInputs(alloc, bytes);
}

// Fixed-head positions of the three list offsets.
const messages_offset_pos = 80;
const filtered_offset_pos = 236;
const blocks_offset_pos = 280;

fn sampleCorrupted(alloc: std.mem.Allocator, pos: usize, value: u32) ![]u8 {
    const bytes = try alloc.dupe(u8, sample_public_inputs_encoding);
    std.mem.writeInt(u32, bytes[pos..][0..4], value, .little);
    return bytes;
}

test "public inputs: rejects a buffer shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), sample_public_inputs_encoding[0 .. public_inputs_head_size - 1]));
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), &.{}));
}

test "public inputs: rejects a first list offset that is not the fixed-head size" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const bytes = try sampleCorrupted(alloc, messages_offset_pos, public_inputs_head_size + 4);
    defer alloc.free(bytes);
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), bytes));
}

test "public inputs: rejects list offsets that run backwards" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const filtered_before_messages = try sampleCorrupted(alloc, filtered_offset_pos, public_inputs_head_size - 4);
    defer alloc.free(filtered_before_messages);
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), filtered_before_messages));

    const filtered_start = std.mem.readInt(u32, sample_public_inputs_encoding[filtered_offset_pos..][0..4], .little);
    const blocks_before_filtered = try sampleCorrupted(alloc, blocks_offset_pos, filtered_start - 1);
    defer alloc.free(blocks_before_filtered);
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), blocks_before_filtered));
}

test "public inputs: rejects a list offset past the end of the buffer" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const bytes = try sampleCorrupted(alloc, blocks_offset_pos, @intCast(sample_public_inputs_encoding.len + 1));
    defer alloc.free(bytes);
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), bytes));

    const huge = try sampleCorrupted(alloc, blocks_offset_pos, 0xFFFFFFFF);
    defer alloc.free(huge);
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), huge));
}

test "public inputs: rejects list regions that are not whole elements" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;

    // One byte short of a whole message.
    const messages_start = std.mem.readInt(u32, sample_public_inputs_encoding[messages_offset_pos..][0..4], .little);
    const short_message = try sampleCorrupted(alloc, filtered_offset_pos, messages_start + 31);
    defer alloc.free(short_message);
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), short_message));

    // A stray byte appended to the u16 list.
    const trailing = try alloc.alloc(u8, sample_public_inputs_encoding.len + 1);
    defer alloc.free(trailing);
    @memcpy(trailing[0..sample_public_inputs_encoding.len], sample_public_inputs_encoding);
    trailing[trailing.len - 1] = 0xEE;
    try std.testing.expectError(error.InvalidSsz, decodePublicInputsOf(arena.allocator(), trailing));
}

// Public inputs with empty lists whose tails are replaced by zero-filled regions of the given sizes.
fn publicInputsWithTails(alloc: std.mem.Allocator, messages_len: usize, filtered_len: usize, blocks_len: usize) ![]u8 {
    var empty = samplePublicInputs();
    empty.l2_l1_messages = &.{};
    empty.filtered_addresses = &.{};
    empty.l2_messaging_blocks_offsets = &.{};
    const head = try l2_execution_proof.encodePublicInputs(alloc, empty);
    defer alloc.free(head);

    const bytes = try alloc.alloc(u8, head.len + messages_len + filtered_len + blocks_len);
    @memcpy(bytes[0..head.len], head);
    @memset(bytes[head.len..], 0);
    std.mem.writeInt(u32, bytes[messages_offset_pos..][0..4], @intCast(head.len), .little);
    std.mem.writeInt(u32, bytes[filtered_offset_pos..][0..4], @intCast(head.len + messages_len), .little);
    std.mem.writeInt(u32, bytes[blocks_offset_pos..][0..4], @intCast(head.len + messages_len + filtered_len), .little);
    return bytes;
}

test "public inputs: accepts lists at their bound and rejects one element more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const max_messages = l2_execution_proof.MAX_MESSAGES;
    const max_filtered = l2_execution_proof.MAX_FILTERED;
    const max_blocks = l2_execution_proof.MAX_MESSAGING_BLOCKS;

    const full = try publicInputsWithTails(alloc, max_messages * 32, max_filtered * 20, max_blocks * 2);
    defer alloc.free(full);
    const decoded = try decodePublicInputsOf(arena.allocator(), full);
    try std.testing.expectEqual(max_messages, decoded.l2_l1_messages.len);
    try std.testing.expectEqual(max_filtered, decoded.filtered_addresses.len);
    try std.testing.expectEqual(max_blocks, decoded.l2_messaging_blocks_offsets.len);
    for (decoded.l2_messaging_blocks_offsets) |offset| try std.testing.expectEqual(@as(u16, 0), offset);

    const too_many_messages = try publicInputsWithTails(alloc, (max_messages + 1) * 32, 0, 0);
    defer alloc.free(too_many_messages);
    try std.testing.expectError(error.BoundsViolation, decodePublicInputsOf(arena.allocator(), too_many_messages));

    const too_many_filtered = try publicInputsWithTails(alloc, 0, (max_filtered + 1) * 20, 0);
    defer alloc.free(too_many_filtered);
    try std.testing.expectError(error.BoundsViolation, decodePublicInputsOf(arena.allocator(), too_many_filtered));

    const too_many_blocks = try publicInputsWithTails(alloc, 0, 0, (max_blocks + 1) * 2);
    defer alloc.free(too_many_blocks);
    try std.testing.expectError(error.BoundsViolation, decodePublicInputsOf(arena.allocator(), too_many_blocks));
}

test "public inputs: encoder rejects lists beyond their bound" {
    const alloc = std.testing.allocator;

    const messages = try alloc.alloc([32]u8, l2_execution_proof.MAX_MESSAGES + 1);
    defer alloc.free(messages);
    @memset(messages, repeat32(0));
    var too_many_messages = samplePublicInputs();
    too_many_messages.l2_l1_messages = messages;
    try std.testing.expectError(error.BoundsViolation, l2_execution_proof.encodePublicInputs(alloc, too_many_messages));

    const blocks = try alloc.alloc(u16, l2_execution_proof.MAX_MESSAGING_BLOCKS + 1);
    defer alloc.free(blocks);
    @memset(blocks, 0);
    var too_many_blocks = samplePublicInputs();
    too_many_blocks.l2_messaging_blocks_offsets = blocks;
    try std.testing.expectError(error.BoundsViolation, l2_execution_proof.encodePublicInputs(alloc, too_many_blocks));
}

// ── Proof and verifiable proof: malformed encodings ──────────────────────────

fn decodeProofOf(alloc: std.mem.Allocator, bytes: []const u8) !l2_execution_proof.L2ExecutionProof {
    return l2_execution_proof.decodeProof(alloc, bytes);
}

fn decodeVerifiableProofOf(alloc: std.mem.Allocator, bytes: []const u8) !l2_execution_proof.VerifiableL2ExecutionProof {
    return l2_execution_proof.decodeVerifiableProof(alloc, bytes);
}

test "proof: rejects a buffer shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    try std.testing.expectError(error.InvalidSsz, decodeProofOf(arena.allocator(), sample_proof_encoding[0 .. proof_head_size - 1]));
}

test "proof: rejects a public-inputs offset that is not the fixed-head size" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const bytes = try alloc.dupe(u8, sample_proof_encoding);
    defer alloc.free(bytes);
    std.mem.writeInt(u32, bytes[0..4], proof_head_size + 4, .little);
    try std.testing.expectError(error.InvalidSsz, decodeProofOf(arena.allocator(), bytes));
}

test "proof: rejects a proof offset before the public inputs or past the end" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const bytes = try alloc.dupe(u8, sample_proof_encoding);
    defer alloc.free(bytes);

    std.mem.writeInt(u32, bytes[12..16], proof_head_size - 4, .little);
    try std.testing.expectError(error.InvalidSsz, decodeProofOf(arena.allocator(), bytes));

    std.mem.writeInt(u32, bytes[12..16], @intCast(bytes.len + 1), .little);
    try std.testing.expectError(error.InvalidSsz, decodeProofOf(arena.allocator(), bytes));
}

test "proof: rejects non-canonical public inputs nested inside it" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const bytes = try alloc.dupe(u8, sample_proof_encoding);
    defer alloc.free(bytes);
    std.mem.writeInt(u32, bytes[proof_head_size + messages_offset_pos ..][0..4], public_inputs_head_size + 4, .little);
    try std.testing.expectError(error.InvalidSsz, decodeProofOf(arena.allocator(), bytes));
}

test "proof: accepts proof bytes at their bound and rejects one byte more" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const public_inputs_len = sample_public_inputs_encoding.len;

    const at_bound = try alloc.alloc(u8, proof_head_size + public_inputs_len + l2_execution_proof.MAX_PROOF_BYTES);
    defer alloc.free(at_bound);
    @memcpy(at_bound[0 .. proof_head_size + public_inputs_len], sample_proof_encoding[0 .. proof_head_size + public_inputs_len]);
    @memset(at_bound[proof_head_size + public_inputs_len ..], 0);
    try std.testing.expectEqual(l2_execution_proof.MAX_PROOF_BYTES, (try decodeProofOf(arena.allocator(), at_bound)).proof.len);

    const over_bound = try alloc.alloc(u8, at_bound.len + 1);
    defer alloc.free(over_bound);
    @memcpy(over_bound[0..at_bound.len], at_bound);
    over_bound[at_bound.len] = 0;
    try std.testing.expectError(error.BoundsViolation, decodeProofOf(arena.allocator(), over_bound));

    const proof_bytes = try alloc.alloc(u8, l2_execution_proof.MAX_PROOF_BYTES + 1);
    defer alloc.free(proof_bytes);
    @memset(proof_bytes, 0);
    var oversized = sampleVerifiableProof().proof;
    oversized.proof = proof_bytes;
    try std.testing.expectError(error.BoundsViolation, l2_execution_proof.encodeProof(alloc, oversized));
}

test "verifiable proof: rejects every truncation that cuts into the proof head or public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const without_proof_bytes_len = verifiable_head_size + proof_head_size + sample_public_inputs_encoding.len;
    for (0..without_proof_bytes_len) |len| {
        try std.testing.expectError(error.InvalidSsz, decodeVerifiableProofOf(arena.allocator(), sample_verifiable_bytes[0..len]));
    }
}

test "verifiable proof: rejects a proof offset that is not the fixed-head size" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = std.testing.allocator;
    const bytes = try alloc.dupe(u8, &sample_verifiable_bytes);
    defer alloc.free(bytes);
    std.mem.writeInt(u32, bytes[0..4], verifiable_head_size + 4, .little);
    try std.testing.expectError(error.InvalidSsz, decodeVerifiableProofOf(arena.allocator(), bytes));
}
