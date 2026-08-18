//! Shared test-only fixtures for the rollup guest: one `sampleInput`, used by the round-trip test,
//! the malformed-input tests, the business-logic tests, and `tools/gen_fixture.zig` — so there is
//! exactly one place that defines what a valid `RollupProofPrivateInput` looks like for testing.

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");

pub fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

pub fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

/// A readable `RollupProofPrivateInput`: two conflations, two l2-execution proofs sharing one
/// `program_vk` (so `dedupSortedProgramVks` has something to actually collapse). `block_rlps`
/// content is RLP-shaped filler, not valid RLP — this guest never parses it (a zero-copy passthrough
/// field), so only its byte identity across encode/decode matters for these tests.
pub fn sampleInput(alloc: std.mem.Allocator) !rollup_ssz.RollupProofPrivateInput {
    const conflations = try alloc.dupe(rollup_ssz.ConflationWitness, &[_]rollup_ssz.ConflationWitness{
        .{ .block_rlps = try alloc.dupe([]const u8, &[_][]const u8{
            &[_]u8{ 0xf9, 0x02, 0x15, 0xa0 },
            &[_]u8{ 0xf9, 0x02, 0x16, 0xb1 },
        }) },
        .{ .block_rlps = try alloc.dupe([]const u8, &[_][]const u8{
            &[_]u8{ 0xf9, 0x02, 0x15, 0xaa },
            &[_]u8{ 0xf9, 0x02, 0x16, 0xbb },
        }) },
    });

    const proof0: rollup_ssz.VerifiableL2ExecutionProof = .{
        .program_vk = repeat32(0xaa),
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = repeat32(0x0a),
                .end_block_hash = repeat32(0x0b),
                .end_block_number = 11,
                .end_block_timestamp = 1763000200,
                .l2_l1_messages_hash = repeat32(0x06),
                .parent_l1_l2_bridge_rolling_hash = repeat32(0x02),
                .parent_l1_l2_bridge_rolling_hash_message_number = 0,
                .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
                .end_l1_l2_bridge_rolling_hash_message_number = 4,
                .dynamic_chain_config_hash = repeat32(0xc0),
                .parent_ftx_rolling_hash = repeat32(0x04),
                .parent_ftx_number = 15,
                .end_ftx_rolling_hash = repeat32(0x05),
                .end_processed_ftx_number = 18,
                .filtered_addresses_hash = repeat32(0x07),
                .tx_froms_hash = repeat32(0x08),
            },
            .start_block_number = 10,
            .proof = &[_]u8{ 0xab, 0xcd, 0xef },
            .l2_l1_messages = try alloc.dupe([32]u8, &[_][32]u8{repeat32(0x08)}),
            .tx_froms = try alloc.dupe([20]u8, &[_][20]u8{ repeat20(0x01), repeat20(0x02) }),
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ repeat20(0x03), repeat20(0x04) }),
        },
    };
    const proof1: rollup_ssz.VerifiableL2ExecutionProof = .{
        .program_vk = repeat32(0xaa), // same VK as proof0 — exercises the dedup rollup.zig's tests check
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = repeat32(0x0b),
                .end_block_hash = repeat32(0x0b),
                .end_block_number = 14,
                .end_block_timestamp = 1763000210,
                .l2_l1_messages_hash = repeat32(0x06),
                .parent_l1_l2_bridge_rolling_hash = repeat32(0x03),
                .parent_l1_l2_bridge_rolling_hash_message_number = 4,
                .end_l1_l2_bridge_rolling_hash = repeat32(0x03),
                .end_l1_l2_bridge_rolling_hash_message_number = 4,
                .dynamic_chain_config_hash = repeat32(0xc0),
                .parent_ftx_rolling_hash = repeat32(0x05),
                .parent_ftx_number = 18,
                .end_ftx_rolling_hash = repeat32(0x05),
                .end_processed_ftx_number = 18,
                .filtered_addresses_hash = repeat32(0x07),
                .tx_froms_hash = repeat32(0x08),
            },
            .start_block_number = 12,
            .proof = &[_]u8{ 0xab, 0xcd, 0xff },
            .l2_l1_messages = &.{},
            .tx_froms = &.{},
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ repeat20(0x03), repeat20(0x04) }),
        },
    };

    return .{
        .parent_data_rolling_hash = repeat32(0x47),
        .start_offset = 4,
        .chain_id = 59144,
        .conflations = conflations,
        .chunks = try alloc.dupe([32]u8, &[_][32]u8{repeat32(0x1a)}),
        .l2_execution_proofs = try alloc.dupe(rollup_ssz.VerifiableL2ExecutionProof, &[_]rollup_ssz.VerifiableL2ExecutionProof{ proof0, proof1 }),
        .opaque_prefix_bytes = &[_]u8{ 0xab, 0xab, 0xab, 0xab },
        .opaque_suffix_bytes = &.{},
        .boundary_prev_data_rolling_hash = repeat32(0x39),
    };
}
