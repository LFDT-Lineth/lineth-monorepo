const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");
const rollup_aggregation = @import("rollup_aggregation");

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

// ── Echo semantics ────────────────────────────────────────────────────────────────────────────
// The golden vector has 2 rollup_proofs: `parent_ftx_number` differs (15 vs 18), so the
// first/last split is independently observable, not just "any value would pass".

test "run: copied fields echo the mapped source (first/last rollup_proofs, PIs)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    const input = try rollup_aggregation_ssz.decodeInput(alloc, golden_input);
    const out = try rollup_aggregation.run(alloc, input);
    const pi = out.public_inputs;

    // last (rollup_proofs[1], end_block_number 18) sourced fields.
    try std.testing.expectEqual(@as(u64, 18), pi.end_block_number);
    try std.testing.expectEqual(@as(u64, 1763000557), pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &repeat32(0x33), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 7), pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x55), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 18), pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0b), &pi.end_block_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x8d), &pi.end_data_rolling_hash);
    try std.testing.expectEqual(@as(u64, 131072), pi.end_offset);

    // first (rollup_proofs[0], parent_ftx_number 15) sourced fields.
    try std.testing.expectEqualSlices(u8, &repeat32(0x22), &pi.parent_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.parent_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x44), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 15), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0xc0), &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0a), &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x47), &pi.parent_data_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.start_offset);

    // program_vks: union of {0xbb (both proofs' own program_vk), 0xaa (both PIs' program_vks)},
    // deduplicated and sorted ascending -> [0xaa, 0xbb].
    try std.testing.expectEqual(@as(usize, 2), pi.program_vks.len);
    try std.testing.expectEqualSlices(u8, &repeat32(0xaa), &pi.program_vks[0]);
    try std.testing.expectEqualSlices(u8, &repeat32(0xbb), &pi.program_vks[1]);

    // l2_l1_roots/filtered_addresses: concatenation of every proof's own lists, input order.
    try std.testing.expectEqual(@as(usize, 4), out.l2_l1_roots.len);
    try std.testing.expectEqualSlices(u8, &repeat32(0x77), &out.l2_l1_roots[0]);
    try std.testing.expectEqualSlices(u8, &repeat32(0x88), &out.l2_l1_roots[1]);
    try std.testing.expectEqualSlices(u8, &repeat32(0x77), &out.l2_l1_roots[2]);
    try std.testing.expectEqualSlices(u8, &repeat32(0x88), &out.l2_l1_roots[3]);
    try std.testing.expectEqual(@as(usize, 2), out.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &repeat20(0x01), &out.filtered_addresses[0]);
    try std.testing.expectEqualSlices(u8, &repeat20(0x01), &out.filtered_addresses[1]);
}

test "run: sentinel fields equal their pinned constants exactly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    const input = try rollup_aggregation_ssz.decodeInput(alloc, golden_input);
    const out = try rollup_aggregation.run(alloc, input);
    const pi = out.public_inputs;

    try std.testing.expectEqualSlices(u8, &rollup_aggregation.L2_L1_BRIDGE_TRANSACTION_TREE, &pi.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &rollup_aggregation.FILTERED_ADDRESSES_HASH, &pi.filtered_addresses_hash);
    try std.testing.expectEqual(@as(usize, 1), out.l2_messaging_blocks_offsets.len);
    try std.testing.expectEqual(rollup_aggregation.L2_MESSAGING_BLOCKS_OFFSETS_ELEMENT, out.l2_messaging_blocks_offsets[0]);

    // Exact pinned hex, transcribed independently of `rollup_aggregation.zig`'s own constants —
    // catches a wrong constant definition, not just a copy/paste of the same bug into both the
    // constant and this assertion.
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0x09, 0x18, 0x83, 0x61, 0x98, 0x23, 0x9a, 0x5e, 0xdf, 0x09, 0x36, 0xdb, 0x3e, 0x28, 0xa6, 0x4d,
        0x5c, 0x4e, 0x19, 0x5f, 0xd7, 0x28, 0xe6, 0xdd, 0xfc, 0x35, 0x44, 0xfc, 0x95, 0x00, 0x8a, 0xb3,
    }, &pi.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0x63, 0xd4, 0x0d, 0x3e, 0xa3, 0x87, 0x06, 0x50, 0x27, 0xb3, 0x69, 0xa2, 0x5c, 0xc4, 0x41, 0xdd,
        0x76, 0x85, 0xe2, 0xa9, 0xa9, 0xe0, 0xe5, 0x65, 0x56, 0xec, 0x2e, 0x82, 0xbb, 0xe2, 0x27, 0x3c,
    }, &pi.filtered_addresses_hash);
    try std.testing.expectEqual(@as(u64, 0xd18d873fe2a9f192), out.l2_messaging_blocks_offsets[0]);
}

// ── Malformed input: empty proofs list ───────────────────────────────────────────────────────────

test "run: rejects an empty rollup_proofs list" {
    const input = rollup_aggregation_ssz.RollupAggregationProofPrivateInput{
        .rollup_proofs = &.{},
    };
    try std.testing.expectError(error.EmptyProofs, rollup_aggregation.run(std.testing.allocator, input));
}
