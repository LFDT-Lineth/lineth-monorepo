const std = @import("std");

const l2_execution_ssz = @import("l2_execution_ssz");

// Golden vectors, embedded straight from the sibling rollup_spec package (the Python reference
// codec's committed output for `getZkL2ExecutionProofV1`) — see build.zig's anonymous imports.
const input_bytes = @embedFile("l2_execution_input.ssz");
const output_bytes = @embedFile("l2_execution_output.ssz");

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// Cross-language byte-exact gate (input): decode the golden vector, then re-encode the decoded
// value and assert the bytes match the vector exactly. Proves this Zig codec's offset/field layout
// agrees with the Python reference codec's `encode_bytes` bit for bit.
test "input: decode then re-encode reproduces the golden vector exactly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = try l2_execution_ssz.decodeInput(alloc, input_bytes);
    const reencoded = try l2_execution_ssz.encodeInput(alloc, value);

    try std.testing.expectEqualSlices(u8, input_bytes, reencoded);
}

// Sanity-checks the decoded input's fields against `getZkL2ExecutionProofV1.request.json`
// (mirrored in rollup_spec's test_rollup_io.py), so the byte-exact test above isn't the only thing
// standing between this codec and a silently-wrong field mapping.
test "input: decoded fields match the request fixture" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = try l2_execution_ssz.decodeInput(alloc, input_bytes);

    try std.testing.expectEqual(@as(u64, 100), value.parent_last_processed_ftx_number);
    try std.testing.expectEqual(@as(u64, 59144), value.chain_config.chain_id);
    try std.testing.expectEqualSlices(u8, &repeat20(0x11), &value.chain_config.l2_message_service_address);
    try std.testing.expectEqualSlices(u8, &repeat20(0x00), &value.chain_config.coinbase);
    try std.testing.expectEqual(@as(usize, 2), value.payloads.len);

    // Each payload's stateless_input_ssz is carried opaquely, but must still be the vanilla
    // stateless-input's own 0x0001-framed bytes — the hand-off boundary this codec preserves.
    for (value.payloads) |payload| {
        try std.testing.expect(payload.stateless_input_ssz.len >= 2);
        try std.testing.expectEqual(@as(u8, 0x00), payload.stateless_input_ssz[0]);
        try std.testing.expectEqual(@as(u8, 0x01), payload.stateless_input_ssz[1]);
    }

    try std.testing.expectEqual(@as(usize, 1), value.payloads[0].forced_transactions.len);
    const ftx0 = value.payloads[0].forced_transactions[0];
    try std.testing.expectEqual(@as(u64, 16), ftx0.number);
    try std.testing.expectEqual(@as(u64, 1000599), ftx0.deadline);
    try std.testing.expectEqual(@as(u8, 0), ftx0.acceptance); // INCLUDED
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x02, 0xf8, 0x6b }, ftx0.signed_tx_rlp);

    try std.testing.expectEqual(@as(usize, 2), value.payloads[1].forced_transactions.len);
    const ftx1 = value.payloads[1].forced_transactions[1];
    try std.testing.expectEqual(@as(u64, 18), ftx1.number);
    try std.testing.expectEqual(@as(u64, 1000601), ftx1.deadline);
    try std.testing.expectEqual(@as(u8, 4), ftx1.acceptance); // FILTERED_ADDRESS_TO
    try std.testing.expectEqual(@as(usize, 0), ftx1.signed_tx_rlp.len);
}

// Cross-language byte-exact gate (output): build the same logical value the Python side built
// from `getZkL2ExecutionProofV1.response.json` and assert `encodeOutput` reproduces the committed
// golden vector exactly.
test "output: encode reproduces the golden vector exactly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = l2_execution_ssz.L2ExecutionProofOutput{
        .public_inputs = .{
            .parent_block_hash = repeat32(0x0a),
            .end_block_hash = repeat32(0x0b),
            .end_block_number = 1000503,
            .end_block_timestamp = 1763000123,
            .l2_l1_messages_hash = repeat32(0x01),
            .parent_l1_l2_bridge_rolling_hash = repeat32(0x02),
            .parent_l1_l2_bridge_rolling_hash_message_number = 0,
            .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
            .end_l1_l2_bridge_rolling_hash_message_number = 5,
            .dynamic_chain_config_hash = repeat32(0xc0),
            .parent_ftx_rolling_hash = repeat32(0x04),
            .parent_processed_ftx_number = 16,
            .end_ftx_rolling_hash = repeat32(0x05),
            .end_processed_ftx_number = 18,
            .filtered_addresses_hash = repeat32(0x06),
            .tx_froms_hash = repeat32(0x07),
        },
        .start_block_number = 1000501,
        .l2_l1_messages = &[_][32]u8{repeat32(0x08)},
        .tx_froms = &[_][20]u8{ repeat20(0x01), repeat20(0x02) },
        .filtered_addresses = &[_][20]u8{repeat20(0x09)},
    };

    const encoded = try l2_execution_ssz.encodeOutput(alloc, value);

    try std.testing.expectEqualSlices(u8, output_bytes, encoded);
}

test "input: rejects a body shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const truncated = input_bytes[0 .. 2 + 10];
    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, truncated));
}

test "input: rejects the wrong schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var corrupted = try alloc.dupe(u8, input_bytes);
    corrupted[0] = 0x00;
    corrupted[1] = 0x03; // the output schema id, on input bytes
    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, corrupted));
}
