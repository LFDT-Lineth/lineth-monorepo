const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");
const rollup_aggregation = @import("rollup_aggregation");
const support = @import("support.zig");
const repeat32 = support.repeat32;
const repeat20 = support.repeat20;
const sampleInput = support.sampleInput;

// ── Echo semantics ────────────────────────────────────────────────────────────────────────────
// `sampleInput` has 2 rollup_proofs: `parent_ftx_number` differs (15 vs 18), so the first/last
// split is independently observable, not just "any value would pass".

test "run: copied fields echo the mapped source (first/last rollup_proofs, PIs)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const input = try sampleInput(alloc);
    const out = try rollup_aggregation.run(alloc, input);
    const pi = out.public_inputs;

    // last (rollup_proofs[1]) sourced fields.
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_NUMBER, pi.end_block_number);
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_TIMESTAMP, pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_L1L2_BRIDGE_ROLLING_HASH, &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_FTX_ROLLING_HASH, &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_END_PROCESSED_FTX_NUMBER, pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_BLOCK_HASH, &pi.end_block_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_END_DATA_ROLLING_HASH, &pi.end_data_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_END_OFFSET, pi.end_offset);

    // first (rollup_proofs[0]) sourced fields.
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH, &pi.parent_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi.parent_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_FTX_ROLLING_HASH, &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_PARENT_FTX_NUMBER, pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_DYNAMIC_CHAIN_CONFIG_HASH, &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_BLOCK_HASH, &pi.parent_block_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_DATA_ROLLING_HASH, &pi.parent_data_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_START_OFFSET, pi.start_offset);

    // program_vks: union of {SHARED_PROGRAM_VK (both proofs' own program_vk), PROOF0_PROGRAM_VKS_0
    // (both PIs' program_vks)}, deduplicated and sorted ascending -> [PROOF0_PROGRAM_VKS_0, SHARED_PROGRAM_VK].
    try std.testing.expectEqual(@as(usize, 2), pi.program_vks.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PROGRAM_VKS_0, &pi.program_vks[0]);
    try std.testing.expectEqualSlices(u8, &support.SHARED_PROGRAM_VK, &pi.program_vks[1]);

    // l2_l1_roots/filtered_addresses: concatenation of every proof's own lists, input order.
    try std.testing.expectEqual(@as(usize, 4), out.l2_l1_roots.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_L2_L1_ROOT_0, &out.l2_l1_roots[0]);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_L2_L1_ROOT_1, &out.l2_l1_roots[1]);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_L2_L1_ROOT_0, &out.l2_l1_roots[2]);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_L2_L1_ROOT_1, &out.l2_l1_roots[3]);
    try std.testing.expectEqual(@as(usize, 2), out.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_FILTERED_ADDRESS_0, &out.filtered_addresses[0]);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_FILTERED_ADDRESS_0, &out.filtered_addresses[1]);
}

test "run: sentinel fields equal their pinned constants exactly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const input = try sampleInput(alloc);
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
