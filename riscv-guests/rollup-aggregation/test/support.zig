//! Shared test-only fixtures for the rollup-aggregation guest: one `sampleInput`, used by the
//! round-trip test, the malformed-input tests, the business-logic tests, and
//! `tools/gen_fixture.zig` — so there is exactly one place that defines what a valid
//! `RollupAggregationProofPrivateInput` looks like for testing.

const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");

pub fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

pub fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

/// A readable `RollupAggregationProofPrivateInput`: two rollup proofs whose `parent_ftx_number`
/// differs (15 vs 18), so the first/last split rollup_aggregation.zig's own tests exercise is
/// independently observable.
pub fn sampleInput(alloc: std.mem.Allocator) !rollup_aggregation_ssz.RollupAggregationProofPrivateInput {
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
