const std = @import("std");
const rollup_ssz = @import("rollup_ssz");
const rollup = @import("rollup");

const golden_input = @embedFile("testdata/10-14-getZkRollupProofV1.request.ssz");

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

    const input = try rollup_ssz.decodeInput(alloc, golden_input);
    const out = try rollup.run(alloc, input);
    const pi = out.public_inputs;

    try std.testing.expectEqualSlices(u8, &rollup.L2_L1_BRIDGE_TRANSACTION_TREE, &pi.l2_l1_bridge_transaction_tree);
    try std.testing.expectEqualSlices(u8, &rollup.FILTERED_ADDRESSES_HASH, &pi.filtered_addresses_hash);
    try std.testing.expectEqualSlices(u8, &rollup.END_DATA_ROLLING_HASH, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(rollup.END_OFFSET, pi.end_offset);
    try std.testing.expectEqual(@as(usize, 1), out.l2_l1_roots.len);
    try std.testing.expectEqualSlices(u8, &rollup.L2_L1_ROOTS_ELEMENT, &out.l2_l1_roots[0]);

    // Exact pinned hex, transcribed from the task spec, not just "equals the exported constant" —
    // catches a wrong constant definition, not just a copy/paste of the same bug into the assertion.
    try std.testing.expectEqualSlices(u8, &[_]u8{
        0xbc, 0x43, 0x6f, 0xcf, 0xbb, 0x17, 0x58, 0x35, 0xd1, 0x2e, 0x0a, 0x12, 0xf4, 0x53, 0x4f, 0x60,
        0xcd, 0x92, 0xdb, 0xd4, 0xba, 0xbf, 0x87, 0xdb, 0x23, 0x39, 0x27, 0x7a, 0x72, 0xec, 0xd2, 0x2a,
    }, &pi.l2_l1_bridge_transaction_tree);
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
