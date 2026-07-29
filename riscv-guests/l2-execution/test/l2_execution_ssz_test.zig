const std = @import("std");

const l2_execution_ssz = @import("l2_execution_ssz");

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// A hand-built input value — readable, diffable Zig source, not an opaque SSZ blob a human can't
// review or hand-edit when the schema changes. Two payloads, with forced transactions covering a
// non-empty signed_tx_rlp and a different acceptance outcome, exercise every variable-length branch
// this codec has.
fn sampleInput() l2_execution_ssz.L2ExecutionProofPrivateInput {
    const payload0_ftx = [_]l2_execution_ssz.ForcedTransactionWitness{
        .{ .number = 16, .deadline = 1000599, .acceptance = 0, .signed_tx_rlp = &[_]u8{ 0x02, 0xf8, 0x6b } },
    };
    const payload1_ftx = [_]l2_execution_ssz.ForcedTransactionWitness{
        .{ .number = 17, .deadline = 1000600, .acceptance = 0, .signed_tx_rlp = &[_]u8{} },
        .{ .number = 18, .deadline = 1000601, .acceptance = 4, .signed_tx_rlp = &[_]u8{} },
    };
    const payloads = [_]l2_execution_ssz.LineaPayloadInput{
        // The vanilla stateless-input bytes are carried opaquely (zero-copy) by this codec — never
        // decoded further here — so a minimal 0x0001-framed placeholder stands in for a real vector.
        .{ .stateless_input_ssz = &[_]u8{ 0x00, 0x01, 0xaa, 0xbb, 0xcc }, .forced_transactions = &payload0_ftx },
        .{ .stateless_input_ssz = &[_]u8{ 0x00, 0x01, 0xdd, 0xee }, .forced_transactions = &payload1_ftx },
    };
    return .{
        .parent_ftx_rolling_hash = repeat32(0x04),
        .parent_last_processed_ftx_number = 100,
        .chain_config = .{
            .l2_message_service_address = repeat20(0x11),
            .coinbase = repeat20(0x00),
            .chain_id = 59144,
        },
        .payloads = &payloads,
    };
}

test "input: encode then decode round-trips every field" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = sampleInput();
    const encoded = try l2_execution_ssz.encodeInput(alloc, value);
    const decoded = try l2_execution_ssz.decodeInput(alloc, encoded);

    try std.testing.expectEqual(value.parent_last_processed_ftx_number, decoded.parent_last_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &value.parent_ftx_rolling_hash, &decoded.parent_ftx_rolling_hash);
    try std.testing.expectEqual(value.chain_config.chain_id, decoded.chain_config.chain_id);
    try std.testing.expectEqualSlices(u8, &value.chain_config.l2_message_service_address, &decoded.chain_config.l2_message_service_address);
    try std.testing.expectEqualSlices(u8, &value.chain_config.coinbase, &decoded.chain_config.coinbase);
    try std.testing.expectEqual(value.payloads.len, decoded.payloads.len);

    for (value.payloads, decoded.payloads) |want, got| {
        try std.testing.expectEqualSlices(u8, want.stateless_input_ssz, got.stateless_input_ssz);
        try std.testing.expectEqual(want.forced_transactions.len, got.forced_transactions.len);
        for (want.forced_transactions, got.forced_transactions) |want_ftx, got_ftx| {
            try std.testing.expectEqual(want_ftx.number, got_ftx.number);
            try std.testing.expectEqual(want_ftx.deadline, got_ftx.deadline);
            try std.testing.expectEqual(want_ftx.acceptance, got_ftx.acceptance);
            try std.testing.expectEqualSlices(u8, want_ftx.signed_tx_rlp, got_ftx.signed_tx_rlp);
        }
    }
}

// `encodeOutput` commits ONLY `keccak256(public_inputs)` — 32 bytes, nothing else — so this asserts
// internal self-consistency against `hashPublicInputs` directly rather than a byte-exact fixture.
test "output: encode commits ONLY hashPublicInputs(public_inputs)" {
    const public_inputs = l2_execution_ssz.L2ExecutionProofPublicInput{
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
    };

    const out = l2_execution_ssz.encodeOutput(public_inputs);
    const encoded = &out;

    // 2(schema) + 32(hash) = 34 bytes total — ONLY the hash, nothing else.
    try std.testing.expectEqual(@as(usize, 34), encoded.len);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x00, 0x03 }, encoded[0..2]); // OUTPUT_SCHEMA_ID

    const pi_hash = l2_execution_ssz.hashPublicInputs(public_inputs);
    try std.testing.expectEqualSlices(u8, &pi_hash, encoded[2..34]);
}

test "input: rejects a body shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const truncated = encoded[0 .. 2 + 10];
    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, truncated));
}

test "input: rejects the wrong schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    var corrupted = try alloc.dupe(u8, encoded);
    corrupted[0] = 0x00;
    corrupted[1] = 0x03; // the output schema id, on input bytes
    try std.testing.expectError(error.InvalidSsz, l2_execution_ssz.decodeInput(alloc, corrupted));
}
