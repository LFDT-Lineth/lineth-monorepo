const std = @import("std");
const rollup_ssz = @import("rollup_ssz");
const rollup = @import("rollup");
const support = @import("support.zig");
const repeat32 = support.repeat32;
const repeat20 = support.repeat20;
const sampleInput = support.sampleInput;

// ── Echo semantics ────────────────────────────────────────────────────────────────────────────
// `sampleInput` has 2 l2_execution_proofs, both carrying the SAME program_vk (`SHARED_PROGRAM_VK`)
// — so `dedupSortedProgramVks` collapses `program_vks` to a single element, which is itself a
// useful assertion (dedup actually ran) distinct from just checking membership.

test "run: copied fields echo the mapped source (first/last l2_execution_proofs, input)" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const input = try sampleInput(alloc);
    const out = try rollup.run(alloc, input);
    const pi = out.public_inputs;

    // last (proof1) sourced fields.
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_NUMBER, pi.end_block_number);
    try std.testing.expectEqual(support.PROOF1_END_BLOCK_TIMESTAMP, pi.end_block_timestamp);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_END_L1L2_BRIDGE_ROLLING_HASH, &pi.end_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF1_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi.end_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_END_FTX_ROLLING_HASH, &pi.end_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF1_END_PROCESSED_FTX_NUMBER, pi.end_processed_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_END_BLOCK_HASH, &pi.end_block_hash);

    // first (proof0) sourced fields.
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH, &pi.parent_l1_l2_bridge_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER, pi.parent_l1_l2_bridge_rolling_hash_message_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_FTX_ROLLING_HASH, &pi.parent_ftx_rolling_hash);
    try std.testing.expectEqual(support.PROOF0_PARENT_FTX_NUMBER, pi.parent_ftx_number);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_DYNAMIC_CHAIN_CONFIG_HASH, &pi.dynamic_chain_config_hash);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_PARENT_BLOCK_HASH, &pi.parent_block_hash);

    // Top-level input fields.
    try std.testing.expectEqualSlices(u8, &support.PARENT_DATA_ROLLING_HASH, &pi.parent_data_rolling_hash);
    try std.testing.expectEqual(support.START_OFFSET, pi.start_offset);
    try std.testing.expectEqual(support.PROOF0_START_BLOCK_NUMBER, out.start_block_number);

    // program_vks: both proofs share SHARED_PROGRAM_VK -> deduplicated to exactly one element.
    try std.testing.expectEqual(@as(usize, 1), pi.program_vks.len);
    try std.testing.expectEqualSlices(u8, &support.SHARED_PROGRAM_VK, &pi.program_vks[0]);

    // filtered_addresses: concatenation of every proof's filtered_addresses, input order.
    try std.testing.expectEqual(@as(usize, 4), out.filtered_addresses.len);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_FILTERED_ADDRESS_0, &out.filtered_addresses[0]);
    try std.testing.expectEqualSlices(u8, &support.PROOF0_FILTERED_ADDRESS_1, &out.filtered_addresses[1]);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_FILTERED_ADDRESS_0, &out.filtered_addresses[2]);
    try std.testing.expectEqualSlices(u8, &support.PROOF1_FILTERED_ADDRESS_1, &out.filtered_addresses[3]);
}

test "run: sentinel fields equal their pinned constants exactly" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();
    const alloc = arena.allocator();

    const input = try sampleInput(alloc);
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
