const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");
const rollup_aggregation = @import("rollup_aggregation");

fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

/// A readable, self-contained `RollupAggregationProofPrivateInput`: two rollup proofs whose
/// `parent_ftx_number` differs (15 vs 18), so the first/last split below is independently
/// observable. No external fixture — every field value is visible right here.
fn sampleInput(alloc: std.mem.Allocator) !rollup_aggregation_ssz.RollupAggregationProofPrivateInput {
    const pi0: rollup_aggregation_ssz.RollupPublicInput = .{
        .end_block_number = 11,
        .end_block_timestamp = 1763000457,
        .l2_l1_bridge_transaction_tree = repeat32(0x11),
        .parent_l1_l2_bridge_rolling_hash = repeat32(0x22),
        .parent_l1_l2_bridge_rolling_hash_message_number = 0,
        .end_l1_l2_bridge_rolling_hash = repeat32(0x33),
        .end_l1_l2_bridge_rolling_hash_message_number = 7,
        .dynamic_chain_config_hash = repeat32(0xc0),
        .parent_ftx_rolling_hash = repeat32(0x44),
        .parent_ftx_number = 15,
        .end_ftx_rolling_hash = repeat32(0x55),
        .end_processed_ftx_number = 18,
        .filtered_addresses_hash = repeat32(0x66),
        .parent_data_rolling_hash = repeat32(0x47),
        .end_data_rolling_hash = repeat32(0x8d),
        .parent_block_hash = repeat32(0x0a),
        .end_block_hash = repeat32(0x0b),
        .start_offset = 0,
        .end_offset = 131072,
        .program_vks = try alloc.dupe([32]u8, &[_][32]u8{repeat32(0xaa)}),
    };
    const proof0: rollup_aggregation_ssz.VerifiableRollupProof = .{
        .program_vk = repeat32(0xbb),
        .proof = .{
            .public_inputs = pi0,
            .start_block_number = 10,
            .proof = &[_]u8{ 0xab, 0xcd, 0xef },
            .l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{ repeat32(0x77), repeat32(0x88) }),
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{repeat20(0x01)}),
        },
    };

    var pi1 = pi0;
    pi1.end_block_number = 18;
    pi1.end_block_timestamp = 1763000557;
    pi1.parent_ftx_number = 18;
    const proof1: rollup_aggregation_ssz.VerifiableRollupProof = .{
        .program_vk = repeat32(0xbb),
        .proof = .{
            .public_inputs = pi1,
            .start_block_number = 15,
            .proof = &[_]u8{ 0xab, 0xcd, 0xff },
            .l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{ repeat32(0x77), repeat32(0x88) }),
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{repeat20(0x01)}),
        },
    };

    return .{
        .rollup_proofs = try alloc.dupe(rollup_aggregation_ssz.VerifiableRollupProof, &[_]rollup_aggregation_ssz.VerifiableRollupProof{ proof0, proof1 }),
    };
}

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
