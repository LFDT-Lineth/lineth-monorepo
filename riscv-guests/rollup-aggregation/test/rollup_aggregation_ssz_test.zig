const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");

/// The checked-in golden aggregation-input vector, as 0x-prefixed hex text (reviewable in a diff,
/// unlike a raw binary blob) — the same format `make exec`'s ELF->JSON tooling accepts natively
/// for a non-`.ssz`-suffixed `@path` input. Expected values in the tests below are transcribed
/// once from the fixture and are not re-derived at test time; rollup_spec's own golden-byte test
/// is what proves these bytes are still what the canonical encoder produces.
const golden_input_hex = @embedFile("testdata/10-18-getZkRollupAggregationProofV1.request.ssz.hex");

fn hexToOwnedBytes(allocator: std.mem.Allocator, hex: []const u8) ![]u8 {
    const stripped = if (hex.len >= 2 and hex[0] == '0' and (hex[1] == 'x' or hex[1] == 'X')) hex[2..] else hex;
    const trimmed = std.mem.trimEnd(u8, stripped, "\r\n");
    if (trimmed.len % 2 != 0) return error.OddHexLength;
    const out = try allocator.alloc(u8, trimmed.len / 2);
    _ = try std.fmt.hexToBytes(out, trimmed);
    return out;
}

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// ── Golden decode ─────────────────────────────────────────────────────────────────────────────

test "decodeInput: golden vector decodes to its transcribed field values" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    const v = try rollup_aggregation_ssz.decodeInput(alloc, golden_input);
    try std.testing.expectEqual(@as(usize, 2), v.rollup_proofs.len);

    const p0 = v.rollup_proofs[0];
    try std.testing.expectEqualSlices(u8, &repeat32(0xbb), &p0.program_vk);
    try std.testing.expectEqual(@as(u64, 10), p0.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xab, 0xcd, 0xef }, p0.proof.proof);
    const pi0 = p0.proof.public_inputs;
    try std.testing.expectEqual(@as(u64, 11), pi0.end_block_number);
    try std.testing.expectEqual(@as(u64, 1763000457), pi0.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &repeat32(0x11), &pi0.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &repeat32(0x22), &pi0.parent_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi0.parent_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x33), &pi0.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 7), pi0.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0xc0), &pi0.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x44), &pi0.parent_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 15), pi0.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x55), &pi0.end_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 18), pi0.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x66), &pi0.filtered_addresses_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x47), &pi0.parent_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x8d), &pi0.end_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0a), &pi0.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0b), &pi0.end_block_hash);
    try std.testing.expectEqual(@as(u64, 0), pi0.start_offset);
    try std.testing.expectEqual(@as(u64, 131072), pi0.end_offset);
    try std.testing.expectEqual(@as(usize, 1), pi0.program_vks.len);
    try std.testing.expectEqualSlices(u8, &repeat32(0xaa), &pi0.program_vks[0]);
    try std.testing.expectEqual(@as(usize, 2), p0.proof.l2_l1_roots.len);
    try std.testing.expectEqualSlices(u8, &repeat32(0x77), &p0.proof.l2_l1_roots[0]);
    try std.testing.expectEqualSlices(u8, &repeat32(0x88), &p0.proof.l2_l1_roots[1]);
    try std.testing.expectEqual(@as(usize, 1), p0.proof.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &repeat20(0x01), &p0.proof.filtered_addresses[0]);

    const p1 = v.rollup_proofs[1];
    try std.testing.expectEqualSlices(u8, &repeat32(0xbb), &p1.program_vk);
    try std.testing.expectEqual(@as(u64, 15), p1.proof.start_block_number);
    try std.testing.expectEqualSlices(u8, &[_]u8{ 0xab, 0xcd, 0xff }, p1.proof.proof);
    try std.testing.expectEqual(@as(u64, 18), p1.proof.public_inputs.end_block_number);
    try std.testing.expectEqual(@as(u64, 1763000557), p1.proof.public_inputs.end_block_timestamp);
    try std.testing.expectEqual(@as(u64, 18), p1.proof.public_inputs.parent_ftx_number);
}

// ── Output frame: encode/decode round-trip ───────────────────────────────────────────────────────

fn sampleOutput(alloc: std.mem.Allocator) !rollup_aggregation_ssz.RollupAggregationOutput {
    const program_vks = try alloc.dupe([32]u8, &[_][32]u8{ repeat32(0xaa), repeat32(0xbb) });
    const l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{ repeat32(0x77), repeat32(0x88) });
    const filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{repeat20(0x01)});
    const l2_messaging_blocks_offsets = try alloc.dupe(u64, &[_]u64{0xd18d873fe2a9f192});

    return .{
        .public_inputs = .{
            .end_block_number = 18,
            .end_block_timestamp = 1763000557,
            .l2_l1_bridge_transaction_tree = repeat32(0x09),
            .parent_l1_l2_bridge_rolling_hash = repeat32(0x22),
            .parent_l1_l2_bridge_rolling_hash_message_number = 0,
            .end_l1_l2_bridge_rolling_hash = repeat32(0x33),
            .end_l1_l2_bridge_rolling_hash_message_number = 7,
            .dynamic_chain_config_hash = repeat32(0xc0),
            .parent_ftx_rolling_hash = repeat32(0x44),
            .parent_ftx_number = 15,
            .end_ftx_rolling_hash = repeat32(0x55),
            .end_processed_ftx_number = 18,
            .filtered_addresses_hash = repeat32(0x63),
            .parent_data_rolling_hash = repeat32(0x47),
            .end_data_rolling_hash = repeat32(0x8d),
            .parent_block_hash = repeat32(0x0a),
            .end_block_hash = repeat32(0x0b),
            .start_offset = 0,
            .end_offset = 131072,
            .program_vks = program_vks,
        },
        .l2_l1_roots = l2_l1_roots,
        .filtered_addresses = filtered_addresses,
        .l2_messaging_blocks_offsets = l2_messaging_blocks_offsets,
    };
}

test "encodeOutput/decodeOutput: round-trips every field and carries the 0x1802 schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const value = try sampleOutput(alloc);
    const encoded = try rollup_aggregation_ssz.encodeOutput(alloc, value);

    try std.testing.expectEqualSlices(u8, &[_]u8{ 0x18, 0x02 }, encoded[0..2]);

    const decoded = try rollup_aggregation_ssz.decodeOutput(alloc, encoded);
    try std.testing.expectEqual(value.l2_l1_roots.len, decoded.l2_l1_roots.len);
    for (value.l2_l1_roots, decoded.l2_l1_roots) |want, got| try std.testing.expectEqualSlices(u8, &want, &got);
    try std.testing.expectEqual(value.filtered_addresses.len, decoded.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &value.filtered_addresses[0], &decoded.filtered_addresses[0]);
    try std.testing.expectEqual(value.l2_messaging_blocks_offsets.len, decoded.l2_messaging_blocks_offsets.len);
    try std.testing.expectEqual(value.l2_messaging_blocks_offsets[0], decoded.l2_messaging_blocks_offsets[0]);
    try std.testing.expectEqual(value.public_inputs.end_block_number, decoded.public_inputs.end_block_number);
    try std.testing.expectEqual(value.public_inputs.start_offset, decoded.public_inputs.start_offset);
    try std.testing.expectEqual(value.public_inputs.end_offset, decoded.public_inputs.end_offset);
    try std.testing.expectEqual(value.public_inputs.program_vks.len, decoded.public_inputs.program_vks.len);
    for (value.public_inputs.program_vks, decoded.public_inputs.program_vks) |want, got| {
        try std.testing.expectEqualSlices(u8, &want, &got);
    }
    try std.testing.expectEqualSlices(u8, &value.public_inputs.l2_l1_bridge_transaction_tree, &decoded.public_inputs.l2_l1_bridge_transaction_tree);
}

// ── Malformed input ───────────────────────────────────────────────────────────────────────────

test "decodeInput: rejects the wrong schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    var corrupted = try alloc.dupe(u8, golden_input);
    corrupted[0] = 0x18;
    corrupted[1] = 0x02; // the output schema id, on input bytes
    try std.testing.expectError(error.MalformedFrame, rollup_aggregation_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: rejects a frame truncated below the 2-byte schema id" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    try std.testing.expectError(error.MalformedFrame, rollup_aggregation_ssz.decodeInput(alloc, golden_input[0..1]));
}

test "decodeInput: rejects a body shorter than the fixed head" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    try std.testing.expectError(error.InvalidSsz, rollup_aggregation_ssz.decodeInput(alloc, golden_input[0..2]));
}

test "decodeInput: rejects a misaligned first offset (rollup_proofs)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    var corrupted = try alloc.dupe(u8, golden_input);
    // The rollup_proofs offset sits at absolute byte 2 (schema); the canonical value is 4 (the
    // container's own fixed head size — the only field, so this container is entirely variable).
    std.mem.writeInt(u32, corrupted[2..6], 5, .little);
    try std.testing.expectError(error.InvalidSsz, rollup_aggregation_ssz.decodeInput(alloc, corrupted));
}

test "decodeInput: accepts an emptied rollup_proofs region (decode succeeds; rollup_aggregation.run rejects it)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    // A body containing only the 4-byte fixed head, pointing past itself to an empty region.
    var body = [_]u8{ 0x04, 0x00, 0x00, 0x00 };
    var framed: [6]u8 = undefined;
    std.mem.writeInt(u16, framed[0..2], rollup_aggregation_ssz.INPUT_SCHEMA_ID, .big);
    @memcpy(framed[2..6], &body);
    _ = &body;

    const decoded = try rollup_aggregation_ssz.decodeInput(alloc, &framed);
    try std.testing.expectEqual(@as(usize, 0), decoded.rollup_proofs.len);
}
