const std = @import("std");
const rollup_ssz = @import("rollup_ssz");
const rollup = @import("rollup");

const golden_input_hex = @embedFile("testdata/10-14-getZkRollupProofV1.request.ssz.hex");

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
// The golden vector has 2 l2_execution_proofs, both carrying the SAME program_vk — so
// `dedupSortedProgramVks` collapses `program_vks` to a single element, which is itself a useful
// assertion (dedup actually ran) distinct from just checking membership.

test "run: copied fields echo the mapped source (first/last l2_execution_proofs, input)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    const input = try rollup_ssz.decodeInput(alloc, golden_input);
    const out = try rollup.run(alloc, input);
    const pi = out.public_inputs;

    // last (proof1, end_block_number 14) sourced fields.
    try std.testing.expectEqual(@as(u64, 14), pi.end_block_number);
    try std.testing.expectEqual(@as(u64, 1763000210), pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &repeat32(0x03), &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 4), pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x05), &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 18), pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0b), &pi.end_block_hash);

    // first (proof0, start_block_number 10) sourced fields.
    try std.testing.expectEqualSlices(u8, &repeat32(0x02), &pi.parent_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(@as(u64, 0), pi.parent_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0x04), &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(@as(u64, 15), pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &repeat32(0xc0), &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &repeat32(0x0a), &pi.parent_block_hash);

    // Top-level input fields.
    try std.testing.expectEqualSlices(u8, &repeat32(0x47), &pi.parent_data_rolling_hash);
    try std.testing.expectEqual(@as(u64, 4), pi.start_offset);
    try std.testing.expectEqual(@as(u64, 10), out.start_block_number);

    // program_vks: both proofs share 0xaa*32 -> deduplicated to exactly one element.
    try std.testing.expectEqual(@as(usize, 1), pi.program_vks.len);
    try std.testing.expectEqualSlices(u8, &repeat32(0xaa), &pi.program_vks[0]);

    // filtered_addresses: concatenation of every proof's filtered_addresses, input order.
    try std.testing.expectEqual(@as(usize, 4), out.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &repeat20(0x03), &out.filtered_addresses[0]);
    try std.testing.expectEqualSlices(u8, &repeat20(0x04), &out.filtered_addresses[1]);
    try std.testing.expectEqualSlices(u8, &repeat20(0x03), &out.filtered_addresses[2]);
    try std.testing.expectEqualSlices(u8, &repeat20(0x04), &out.filtered_addresses[3]);
}

test "run: sentinel fields equal their pinned constants exactly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const golden_input = try hexToOwnedBytes(alloc, golden_input_hex);
    const input = try rollup_ssz.decodeInput(alloc, golden_input);
    const out = try rollup.run(alloc, input);
    const pi = out.public_inputs;

    try std.testing.expectEqualSlices(u8, &rollup.L2_L1_BRIDGE_TRANSACTION_TREE, &pi.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &rollup.FILTERED_ADDRESSES_HASH, &pi.filtered_addresses_hash);
    try std.testing.expectEqualSlices(u8, &rollup.END_DATA_ROLLING_HASH, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(rollup.END_OFFSET, pi.end_offset);
    try std.testing.expectEqual(@as(usize, 1), out.l2_l1_roots.len);
    try std.testing.expectEqualSlices(u8, &rollup.L2_L1_ROOTS_ELEMENT, &out.l2_l1_roots[0]);

    // Exact pinned hex, transcribed independently of `rollup.zig`'s own constants — catches a wrong
    // constant definition (e.g. hashing the wrong tag, or a wrong algorithm), not just a copy/paste
    // of the same bug into both the constant and this assertion.
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0xbc, 0x43, 0x6f, 0xcf, 0xbb, 0x17, 0x58, 0x35, 0xd1, 0x2e, 0x0a, 0x12, 0xf4, 0x53, 0x4f, 0x60,
        0xcd, 0x92, 0xdb, 0xd4, 0xba, 0xbf, 0x87, 0xdb, 0x23, 0x39, 0x27, 0x7a, 0x72, 0xec, 0xd2, 0x2a,
    }, &pi.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0x1f, 0xe6, 0x17, 0xc1, 0x0b, 0x3b, 0xfc, 0x97, 0xfd, 0x6d, 0x00, 0x90, 0xc6, 0x08, 0xdf, 0x47,
        0xde, 0x54, 0x4a, 0x4b, 0x7d, 0x4f, 0x63, 0x79, 0x30, 0x0b, 0xd9, 0x61, 0x67, 0xda, 0x2a, 0xda,
    }, &pi.end_data_rolling_hash);
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0x8f, 0xa4, 0xb0, 0x0e, 0x95, 0xcd, 0x07, 0x84, 0xa4, 0x9f, 0x00, 0xef, 0xe0, 0xd1, 0x2f, 0x67,
        0x71, 0x5a, 0x99, 0xa6, 0xcf, 0x54, 0xac, 0x4c, 0xa5, 0x60, 0x6e, 0xbc, 0x4d, 0x0a, 0x42, 0xba,
    }, &pi.filtered_addresses_hash);
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0x45, 0xc2, 0x57, 0x58, 0x65, 0x97, 0x87, 0xf9, 0x68, 0x43, 0xb2, 0x17, 0x1d, 0xd2, 0x09, 0x1c,
        0x96, 0x4e, 0xe7, 0xfe, 0x11, 0x51, 0x8f, 0xab, 0xf1, 0xa4, 0xc9, 0x44, 0xb4, 0xf7, 0x5e, 0x0e,
    }, &out.l2_l1_roots[0]);
    try std.testing.expectEqual(@as(u64, 0x1ab5956f53caf2ea), pi.end_offset);
}

// ── Malformed input: empty proofs list ───────────────────────────────────────────────────────────

test "run: rejects an empty l2_execution_proofs list" {
    const input = rollup_ssz.RollupProofPrivateInput{
        .parent_data_rolling_hash = repeat32(0),
        .start_offset = 0,
        .chain_id = 0,
        .conflations = &.{},
        .chunks = &.{},
        .l2_execution_proofs = &.{},
        .opaque_prefix_bytes = &.{},
        .opaque_suffix_bytes = &.{},
        .boundary_prev_data_rolling_hash = null,
    };
    try std.testing.expectError(error.EmptyProofs, rollup.run(std.testing.allocator, input));
}
