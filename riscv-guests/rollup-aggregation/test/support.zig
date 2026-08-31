//! Shared test-only fixtures for the rollup-aggregation guest: one `sampleInput`, used by the
//! round-trip test, the malformed-input tests, the business-logic tests, and
//! `tools/gen_fixture.zig` — so there is exactly one place that defines what a valid
//! `RollupAggregationProofPrivateInput` looks like for testing.
//!
//! Every field value below is a named constant, and every test asserting against it references the
//! same constant rather than retyping the literal — the builder and its assertions cannot silently
//! drift apart.

const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");

pub fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

pub fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// ── rollup_proofs[0] ──────────────────────────────────────────────────────────────────────────
pub const SHARED_PROGRAM_VK = repeat32(0xbb);

pub const PROOF0_START_BLOCK_NUMBER: u64 = 10;
pub const PROOF0_PROOF_BYTES = [_]u8{ 0xab, 0xcd, 0xef };
pub const PROOF0_L2_L1_ROOT_0 = repeat32(0x77);
pub const PROOF0_L2_L1_ROOT_1 = repeat32(0x88);
pub const PROOF0_FILTERED_ADDRESS_0 = repeat20(0x01);

pub const PROOF0_END_BLOCK_NUMBER: u64 = 11;
pub const PROOF0_END_BLOCK_TIMESTAMP: u64 = 1763000457;
pub const PROOF0_L2_L1_BRIDGE_TRANSACTION_TREE = repeat32(0x11);
pub const PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x22);
pub const PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 0;
pub const PROOF0_END_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x33);
pub const PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 7;
pub const PROOF0_DYNAMIC_CHAIN_CONFIG_HASH = repeat32(0xc0);
pub const PROOF0_PARENT_FTX_ROLLING_HASH = repeat32(0x44);
pub const PROOF0_PARENT_FTX_NUMBER: u64 = 15;
pub const PROOF0_END_FTX_ROLLING_HASH = repeat32(0x55);
pub const PROOF0_END_PROCESSED_FTX_NUMBER: u64 = 18;
pub const PROOF0_FILTERED_ADDRESSES_HASH = repeat32(0x66);
pub const PROOF0_PARENT_DATA_ROLLING_HASH = repeat32(0x47);
pub const PROOF0_END_DATA_ROLLING_HASH = repeat32(0x8d);
pub const PROOF0_PARENT_BLOCK_HASH = repeat32(0x0a);
pub const PROOF0_END_BLOCK_HASH = repeat32(0x0b);
pub const PROOF0_START_OFFSET: u64 = 0;
pub const PROOF0_END_OFFSET: u64 = 131072;
pub const PROOF0_PROGRAM_VKS_0 = repeat32(0xaa);

// ── rollup_proofs[1] ──────────────────────────────────────────────────────────────────────────
// Same PI shape as proof0 except the 3 fields explicitly overridden in `sampleInput` below
// (end_block_number, end_block_timestamp, parent_ftx_number) — this is what makes the first/last
// split in rollup_aggregation.zig's own tests independently observable.
pub const PROOF1_START_BLOCK_NUMBER: u64 = 15;
pub const PROOF1_PROOF_BYTES = [_]u8{ 0xab, 0xcd, 0xff };
pub const PROOF1_L2_L1_ROOT_0 = repeat32(0x77);
pub const PROOF1_L2_L1_ROOT_1 = repeat32(0x88);
pub const PROOF1_FILTERED_ADDRESS_0 = repeat20(0x01);
pub const PROOF1_END_BLOCK_NUMBER: u64 = 18;
pub const PROOF1_END_BLOCK_TIMESTAMP: u64 = 1763000557;
pub const PROOF1_PARENT_FTX_NUMBER: u64 = 18;

/// A readable `RollupAggregationProofPrivateInput` built entirely from the named constants above.
pub fn sampleInput(alloc: std.mem.Allocator) !rollup_aggregation_ssz.RollupAggregationProofPrivateInput {
    const pi0: rollup_aggregation_ssz.RollupPublicInput = .{
        .end_block_number = PROOF0_END_BLOCK_NUMBER,
        .end_block_timestamp = PROOF0_END_BLOCK_TIMESTAMP,
        .l2_l1_bridge_transaction_tree = PROOF0_L2_L1_BRIDGE_TRANSACTION_TREE,
        .parent_l1_l2_bridge_rolling_hash = PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH,
        .parent_l1_l2_bridge_rolling_hash_message_number = PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER,
        .end_l1_l2_bridge_rolling_hash = PROOF0_END_L1L2_BRIDGE_ROLLING_HASH,
        .end_l1_l2_bridge_rolling_hash_message_number = PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER,
        .dynamic_chain_config_hash = PROOF0_DYNAMIC_CHAIN_CONFIG_HASH,
        .parent_ftx_rolling_hash = PROOF0_PARENT_FTX_ROLLING_HASH,
        .parent_ftx_number = PROOF0_PARENT_FTX_NUMBER,
        .end_ftx_rolling_hash = PROOF0_END_FTX_ROLLING_HASH,
        .end_processed_ftx_number = PROOF0_END_PROCESSED_FTX_NUMBER,
        .filtered_addresses_hash = PROOF0_FILTERED_ADDRESSES_HASH,
        .parent_data_rolling_hash = PROOF0_PARENT_DATA_ROLLING_HASH,
        .end_data_rolling_hash = PROOF0_END_DATA_ROLLING_HASH,
        .parent_block_hash = PROOF0_PARENT_BLOCK_HASH,
        .end_block_hash = PROOF0_END_BLOCK_HASH,
        .start_offset = PROOF0_START_OFFSET,
        .end_offset = PROOF0_END_OFFSET,
        .program_vks = try alloc.dupe([32]u8, &[_][32]u8{PROOF0_PROGRAM_VKS_0}),
    };
    const proof0: rollup_aggregation_ssz.VerifiableRollupProof = .{
        .program_vk = SHARED_PROGRAM_VK,
        .proof = .{
            .public_inputs = pi0,
            .start_block_number = PROOF0_START_BLOCK_NUMBER,
            .proof = &PROOF0_PROOF_BYTES,
            .l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{ PROOF0_L2_L1_ROOT_0, PROOF0_L2_L1_ROOT_1 }),
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{PROOF0_FILTERED_ADDRESS_0}),
        },
    };

    var pi1 = pi0;
    pi1.end_block_number = PROOF1_END_BLOCK_NUMBER;
    pi1.end_block_timestamp = PROOF1_END_BLOCK_TIMESTAMP;
    pi1.parent_ftx_number = PROOF1_PARENT_FTX_NUMBER;
    const proof1: rollup_aggregation_ssz.VerifiableRollupProof = .{
        .program_vk = SHARED_PROGRAM_VK,
        .proof = .{
            .public_inputs = pi1,
            .start_block_number = PROOF1_START_BLOCK_NUMBER,
            .proof = &PROOF1_PROOF_BYTES,
            .l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{ PROOF1_L2_L1_ROOT_0, PROOF1_L2_L1_ROOT_1 }),
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{PROOF1_FILTERED_ADDRESS_0}),
        },
    };

    return .{
        .rollup_proofs = try alloc.dupe(rollup_aggregation_ssz.VerifiableRollupProof, &[_]rollup_aggregation_ssz.VerifiableRollupProof{ proof0, proof1 }),
    };
}
