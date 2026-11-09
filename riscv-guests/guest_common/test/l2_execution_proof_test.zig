//! Tests for the shared l2-execution proof codec: the output frame, round-trips of each type and
//! the encoder's bounds. The SSZ rules themselves are tested once in `ssz_test.zig`.

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
const sample_proof_bytes = [_]u8{ 0x50, 0x52, 0x4f, 0x4f, 0x46 };

fn samplePublicInputs() l2_execution_proof.L2ExecutionProofPublicInput {
    return .{
        .parent_block_hash = repeat32(0x0a),
        .end_block_hash = repeat32(0x0b),
        .end_block_number = 1000503,
        .end_block_timestamp = 1763000123,
        .l2_l1_messages = .{ .items = &sample_messages },
        .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
        .end_l1_l2_bridge_rolling_hash_message_number = 5,
        .dynamic_chain_config_hash = repeat32(0xc0),
        .parent_ftx_rolling_hash = repeat32(0x04),
        .parent_ftx_number = 16,
        .end_ftx_rolling_hash = repeat32(0x05),
        .end_processed_ftx_number = 18,
        .filtered_addresses = .{ .items = &sample_filtered },
        .tx_froms_hash = repeat32(0x07),
        .block_count = 3,
        .l2_messaging_blocks_offsets = .{ .items = &sample_offsets },
    };
}

fn withEmptyLists(pi: l2_execution_proof.L2ExecutionProofPublicInput) l2_execution_proof.L2ExecutionProofPublicInput {
    var empty = pi;
    empty.l2_l1_messages = .{ .items = &.{} };
    empty.filtered_addresses = .{ .items = &.{} };
    empty.l2_messaging_blocks_offsets = .{ .items = &.{} };
    return empty;
}

fn sampleVerifiableProof() l2_execution_proof.VerifiableL2ExecutionProof {
    return .{
        .proof = .{
            .public_inputs = samplePublicInputs(),
            .start_block_number = 1000501,
            .proof = .{ .bytes = &sample_proof_bytes },
        },
        .program_vk = repeat32(0x77),
    };
}

fn expectPublicInputsEqual(want: l2_execution_proof.L2ExecutionProofPublicInput, got: l2_execution_proof.L2ExecutionProofPublicInput) !void {
    inline for (std.meta.fields(l2_execution_proof.L2ExecutionProofPublicInput)) |field| {
        const want_field = @field(want, field.name);
        const got_field = @field(got, field.name);
        switch (@typeInfo(field.type)) {
            .@"struct" => {
                try std.testing.expectEqual(want_field.items.len, got_field.items.len);
                for (want_field.items, got_field.items) |w, g| {
                    if (@typeInfo(@TypeOf(w)) == .array) {
                        try std.testing.expectEqualSlices(u8, &w, &g);
                    } else {
                        try std.testing.expectEqual(w, g);
                    }
                }
            },
            .array => try std.testing.expectEqualSlices(u8, &want_field, &got_field),
            else => try std.testing.expectEqual(want_field, got_field),
        }
    }
}

fn expectProofEqual(want: l2_execution_proof.L2ExecutionProof, got: l2_execution_proof.L2ExecutionProof) !void {
    try expectPublicInputsEqual(want.public_inputs, got.public_inputs);
    try std.testing.expectEqual(want.start_block_number, got.start_block_number);
    try std.testing.expectEqualSlices(u8, want.proof.bytes, got.proof.bytes);
}

// ── Guest output ──────────────────────────────────────────────────────────────

test "output is the commitment to the 0x0003 frame followed by the frame carrying the public inputs" {
    const alloc = std.testing.allocator;
    const pi = samplePublicInputs();
    const out = try l2_execution_proof.encodeOutput(alloc, pi);
    defer alloc.free(out);
    const body = try l2_execution_proof.encodePublicInputs(alloc, pi);
    defer alloc.free(body);

    try std.testing.expectEqual(32 + 2 + body.len, out.len);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x00, 0x03 }, out[32..34]);
    try std.testing.expectEqualSlices(u8, body, out[34..]);
    var frame_hash: [32]u8 = undefined;
    std.crypto.hash.sha3.Keccak256.hash(out[32..], &frame_hash, .{});
    try std.testing.expectEqualSlices(u8, &frame_hash, out[0..32]);
}

test "hashPublicInputs is the commitment at the head of the output" {
    const alloc = std.testing.allocator;
    const pi = samplePublicInputs();
    const out = try l2_execution_proof.encodeOutput(alloc, pi);
    defer alloc.free(out);
    const hash = try l2_execution_proof.hashPublicInputs(alloc, pi);
    try std.testing.expectEqualSlices(u8, out[0..32], &hash);
}

// ── Round-trips ───────────────────────────────────────────────────────────────

test "public inputs decode to what was encoded, including empty lists" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    for ([_]l2_execution_proof.L2ExecutionProofPublicInput{ samplePublicInputs(), withEmptyLists(samplePublicInputs()) }) |pi| {
        const bytes = try l2_execution_proof.encodePublicInputs(alloc, pi);
        try expectPublicInputsEqual(pi, try l2_execution_proof.decodePublicInputs(alloc, bytes));
    }
}

test "a proof decodes to what was encoded, with and without proof bytes" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var empty = sampleVerifiableProof().proof;
    empty.public_inputs = withEmptyLists(empty.public_inputs);
    empty.proof = .{ .bytes = &.{} };
    for ([_]l2_execution_proof.L2ExecutionProof{ sampleVerifiableProof().proof, empty }) |proof| {
        const bytes = try l2_execution_proof.encodeProof(alloc, proof);
        try expectProofEqual(proof, try l2_execution_proof.decodeProof(alloc, bytes));
    }
}

test "a verifiable proof decodes to what was encoded" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const verifiable = sampleVerifiableProof();
    const bytes = try l2_execution_proof.encodeVerifiableProof(alloc, verifiable);
    const decoded = try l2_execution_proof.decodeVerifiableProof(alloc, bytes);
    try expectProofEqual(verifiable.proof, decoded.proof);
    try std.testing.expectEqualSlices(u8, &verifiable.program_vk, &decoded.program_vk);
}

// ── Bounds ────────────────────────────────────────────────────────────────────

test "the encoder rejects a list beyond its bound" {
    const alloc = std.testing.allocator;

    const messages = try alloc.alloc([32]u8, l2_execution_proof.MAX_MESSAGES + 1);
    defer alloc.free(messages);
    @memset(messages, repeat32(0));
    var too_many_messages = samplePublicInputs();
    too_many_messages.l2_l1_messages = .{ .items = messages };
    try std.testing.expectError(error.BoundsViolation, l2_execution_proof.encodePublicInputs(alloc, too_many_messages));

    const proof_bytes = try alloc.alloc(u8, l2_execution_proof.MAX_PROOF_BYTES + 1);
    defer alloc.free(proof_bytes);
    @memset(proof_bytes, 0);
    var oversized = sampleVerifiableProof();
    oversized.proof.proof = .{ .bytes = proof_bytes };
    try std.testing.expectError(error.BoundsViolation, l2_execution_proof.encodeVerifiableProof(alloc, oversized));
}
