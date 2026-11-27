//! Tests for the rollup-aggregation guest's wire contract: the schema-id framing of the input and
//! the commitment and schema id of the output, each with a round trip and the rejection of a frame
//! that is not canonical. The SSZ rules are the generic codec's and are tested with it.

const std = @import("std");
const rollup_aggregation = @import("rollup_aggregation");
const scenario = @import("scenario");

const rollup_aggregation_ssz = rollup_aggregation.rollup_aggregation_ssz;
const guest_errors = rollup_aggregation.guest_errors;

fn sampleInput(alloc: std.mem.Allocator) !rollup_aggregation_ssz.RollupAggregationInput {
    const messages = [_][32]u8{ @splat(0x71), @splat(0x72) };
    const proofs = try scenario.build(alloc, &.{
        .{ .messages = &messages, .offsets = &.{ 1, 4 }, .filtered_addresses = &.{@as([20]u8, @splat(0xa1))} },
        .{ .program_vks = &.{@as([32]u8, @splat(0x1f))} },
    });
    for (proofs) |*verifiable| verifiable.proof.proof = .{ .bytes = "proof image" };
    return scenario.input(proofs);
}

fn samplePublicInput() rollup_aggregation_ssz.FinalizationPublicInput {
    const roots = [_][32]u8{ @splat(0x01), @splat(0x02) };
    const addresses = [_][20]u8{@splat(0xa1)};
    const ids = [_][32]u8{ @splat(0x1f), @splat(0x2a) };
    return .{
        .end_block_number = 1_000_520,
        .end_block_timestamp = 1_763_000_123,
        .end_l1_l2_bridge_rolling_hash = @splat(0xb1),
        .end_l1_l2_bridge_rolling_hash_message_number = 4,
        .dynamic_chain_config_hash = @splat(0xc0),
        .parent_ftx_rolling_hash = @splat(0x50),
        .parent_ftx_number = 10,
        .end_ftx_rolling_hash = @splat(0x52),
        .end_processed_ftx_number = 14,
        .parent_data_rolling_hash = @splat(0x30),
        .end_data_rolling_hash = @splat(0x32),
        .parent_block_hash = @splat(0x10),
        .end_block_hash = @splat(0x12),
        .parent_shared_blob_carry_over_tail_bytes = 0,
        .final_shared_blob_carry_over_tail_bytes = 200,
        .l2_l1_tree_depth = 5,
        .l2_l1_roots = .{ .items = &roots },
        .filtered_addresses = .{ .items = &addresses },
        .program_ids = .{ .items = &ids },
        .l2_messaging_blocks_offsets = .{ .items = &.{ 1, 4, 12 } },
    };
}

fn exitCodeOfDecoding(bytes: []const u8) !guest_errors.ExitCode {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    if (rollup_aggregation_ssz.decodeInput(arena.allocator(), bytes)) |_| return error.DecodedMalformedInput else |err| return guest_errors.exitCode(err);
}

test "encodeInput/decodeInput: round-trips the rollup proofs under the 0x0006 schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const original = try sampleInput(alloc);
    const encoded = try rollup_aggregation_ssz.encodeInput(alloc, original);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x00, 0x06 }, encoded[0..2]);
    try std.testing.expectEqualDeep(original, try rollup_aggregation_ssz.decodeInput(alloc, encoded));
}

test "encodeInput/decodeInput: an input without proofs round-trips" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const empty: rollup_aggregation_ssz.RollupAggregationInput = .{ .rollup_proofs = .{ .items = &.{} } };
    const decoded = try rollup_aggregation_ssz.decodeInput(alloc, try rollup_aggregation_ssz.encodeInput(alloc, empty));
    try std.testing.expectEqual(@as(usize, 0), decoded.rollup_proofs.items.len);
}

test "decoding failures exit with the code of their category" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const frame = try rollup_aggregation_ssz.encodeInput(alloc, try sampleInput(alloc));
    const wrong_schema = try alloc.dupe(u8, frame);
    wrong_schema[1] = 0x07; // the aggregation output's schema id
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_FRAME, try exitCodeOfDecoding(wrong_schema));
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_FRAME, try exitCodeOfDecoding(&.{}));
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_SSZ, try exitCodeOfDecoding(frame[0 .. frame.len / 2]));
    try std.testing.expectEqual(guest_errors.CODE_MALFORMED_SSZ, try exitCodeOfDecoding(frame[0..3]));
}

test "encodeOutput: the commitment to the frame, then the 0x0007 schema id and the public inputs" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const pi = samplePublicInput();
    const out = try rollup_aggregation_ssz.encodeOutput(alloc, pi);
    var digest: [32]u8 = undefined;
    std.crypto.hash.sha3.Keccak256.hash(out[32..], &digest, .{});
    try std.testing.expectEqualSlices(u8, &digest, out[0..32]);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x00, 0x07 }, out[32..34]);
    try std.testing.expectEqualDeep(pi, try rollup_aggregation_ssz.decodeOutput(alloc, out));
}

test "decodeOutput: rejects a tampered commitment, a tampered frame and an input frame" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const out = try rollup_aggregation_ssz.encodeOutput(alloc, samplePublicInput());
    const input_frame = try rollup_aggregation_ssz.encodeInput(alloc, try sampleInput(alloc));
    for ([_][]const u8{ &.{}, out[0..33], input_frame }) |bytes| {
        try std.testing.expectError(error.MalformedFrame, rollup_aggregation_ssz.decodeOutput(alloc, bytes));
    }
    const tampered = try alloc.dupe(u8, out);
    tampered[0] ^= 1;
    try std.testing.expectError(error.MalformedFrame, rollup_aggregation_ssz.decodeOutput(alloc, tampered));
    tampered[0] ^= 1;
    tampered[tampered.len - 1] ^= 1;
    try std.testing.expectError(error.MalformedFrame, rollup_aggregation_ssz.decodeOutput(alloc, tampered));
}
