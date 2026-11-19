//! Tests for the l2-execution guest's input codec: the schema-id framing, the round trip of the
//! input with its optional parts present and absent, and the exit codes of the decode errors. The
//! SSZ rules are the generic codec's and are tested with it.

const std = @import("std");

const l2_execution_ssz = @import("l2_execution").l2_execution_ssz;
const guest_errors = @import("l2_execution").guest_errors;

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// Distinct bytes per field, so a field swap in the codec cannot round-trip.
fn sampleSystemContracts() l2_execution_ssz.SystemContracts {
    return .{
        .beacon_roots = repeat20(0xa1),
        .history_storage = repeat20(0xa2),
        .deposit = repeat20(0xa3),
        .withdrawal_request = repeat20(0xa4),
        .consolidation_request = repeat20(0xa5),
        .builder_deposit_request = repeat20(0xa6),
        .builder_exit_request = repeat20(0xa7),
    };
}

// Two payloads, the first with one forced transaction and the second with two, one of them with an
// empty signed transaction.
fn sampleInput() l2_execution_ssz.L2ExecutionProofPrivateInput {
    const payload0_ftx = [_]l2_execution_ssz.ForcedTransactionWitness{
        .{ .number = 16, .deadline = 1000599, .acceptance = 0, .signed_tx_rlp = .{ .bytes = &[_]u8{ 0x02, 0xf8, 0x6b } } },
    };
    const payload1_ftx = [_]l2_execution_ssz.ForcedTransactionWitness{
        .{ .number = 17, .deadline = 1000600, .acceptance = 0, .signed_tx_rlp = .{ .bytes = &[_]u8{} } },
        .{ .number = 18, .deadline = 1000601, .acceptance = 4, .signed_tx_rlp = .{ .bytes = &[_]u8{} } },
    };
    const payloads = [_]l2_execution_ssz.LineaPayloadInput{
        .{ .stateless_input_ssz = .{ .bytes = &[_]u8{ 0x00, 0x01, 0xaa, 0xbb, 0xcc } }, .forced_transactions = .{ .items = &payload0_ftx } },
        .{ .stateless_input_ssz = .{ .bytes = &[_]u8{ 0x00, 0x01, 0xdd, 0xee } }, .forced_transactions = .{ .items = &payload1_ftx } },
    };
    return .{
        .parent_ftx_rolling_hash = repeat32(0x04),
        .parent_last_processed_ftx_number = 100,
        .chain_config = .{
            .l2_message_service_address = repeat20(0x11),
            .coinbase = repeat20(0x00),
            .chain_id = 59144,
            .system_contracts = sampleSystemContracts(),
        },
        .payloads = .{ .items = &payloads },
    };
}

test "input: encode then decode round-trips every field" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const original = sampleInput();
    const encoded = try l2_execution_ssz.encodeInput(alloc, original);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x00, 0x02 }, encoded[0..2]);
    try std.testing.expectEqualDeep(original, try l2_execution_ssz.decodeInput(alloc, encoded));
}

test "input: an input without payloads, and payloads without forced transactions, round-trip" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    var original = sampleInput();
    const payloads = [_]l2_execution_ssz.LineaPayloadInput{
        .{ .stateless_input_ssz = .{ .bytes = &[_]u8{ 0x00, 0x01 } }, .forced_transactions = .{ .items = &.{} } },
    };
    original.payloads = .{ .items = &payloads };
    try std.testing.expectEqualDeep(original, try l2_execution_ssz.decodeInput(alloc, try l2_execution_ssz.encodeInput(alloc, original)));

    original.payloads = .{ .items = &.{} };
    try std.testing.expectEqualDeep(original, try l2_execution_ssz.decodeInput(alloc, try l2_execution_ssz.encodeInput(alloc, original)));
}

test "input: rejects a frame under another schema id or without one" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const wrong_schema = try alloc.dupe(u8, encoded);
    wrong_schema[1] = 0x01;
    try std.testing.expectError(error.MalformedFrame, l2_execution_ssz.decodeInput(alloc, wrong_schema));
    try std.testing.expectError(error.MalformedFrame, l2_execution_ssz.decodeInput(alloc, &.{}));
    try std.testing.expectError(error.MalformedFrame, l2_execution_ssz.decodeInput(alloc, encoded[0..1]));
}

test "decoding failures exit with the invalid-envelope category" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const encoded = try l2_execution_ssz.encodeInput(alloc, sampleInput());
    const wrong_schema = try alloc.dupe(u8, encoded);
    wrong_schema[1] = 0x01;
    for ([_][]const u8{ wrong_schema, encoded[0 .. encoded.len - 1], encoded[0..40] }) |bytes| {
        const err = if (l2_execution_ssz.decodeInput(alloc, bytes)) |_| return error.DecodedMalformedInput else |e| e;
        try std.testing.expectEqual(guest_errors.ExitCode.invalid_ssz_envelope, guest_errors.exitCode(err));
    }
}
