//! Shared test-only fixtures for the rollup guest: one `sampleInput`, used by the round-trip test,
//! the malformed-input tests, the business-logic tests, and `tools/gen_fixture.zig` — so there is
//! exactly one place that defines what a valid `RollupProofPrivateInput` looks like for testing.
//!
//! Every field value below is a named constant, and every test asserting against it references the
//! same constant rather than retyping the literal — the builder and its assertions cannot silently
//! drift apart.

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");

pub fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

pub fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// ── Top-level input fields ───────────────────────────────────────────────────────────────────
pub const PARENT_DATA_ROLLING_HASH = repeat32(0x47);
pub const START_OFFSET: u64 = 4;
pub const CHAIN_ID: u64 = 59144;
pub const CHUNK_0 = repeat32(0x1a);
pub const OPAQUE_PREFIX_BYTES = [_]u8{ 0xab, 0xab, 0xab, 0xab };
pub const BOUNDARY_PREV_DATA_ROLLING_HASH = repeat32(0x39);

// ── Conflation block RLPs ─────────────────────────────────────────────────────────────────────
// RLP-shaped filler (a valid "long list" / "32-byte string" prefix), not real RLP — this guest
// never parses `block_rlps` (a zero-copy passthrough field), so only byte identity across
// encode/decode matters here.
pub const CONFLATION_0_BLOCK_RLP_0 = [_]u8{ 0xf9, 0x02, 0x15, 0xa0 };
pub const CONFLATION_0_BLOCK_RLP_1 = [_]u8{ 0xf9, 0x02, 0x16, 0xb1 };
pub const CONFLATION_1_BLOCK_RLP_0 = [_]u8{ 0xf9, 0x02, 0x15, 0xaa };
pub const CONFLATION_1_BLOCK_RLP_1 = [_]u8{ 0xf9, 0x02, 0x16, 0xbb };

// ── l2_execution_proofs[0] ────────────────────────────────────────────────────────────────────
// Shared with proof1 — both carry the same VK so `dedupSortedProgramVks` has something to
// actually collapse.
pub const SHARED_PROGRAM_VK = repeat32(0xaa);

pub const PROOF0_START_BLOCK_NUMBER: u64 = 10;
pub const PROOF0_PROOF_BYTES = [_]u8{ 0xab, 0xcd, 0xef };
pub const PROOF0_PARENT_BLOCK_HASH = repeat32(0x0a);
pub const PROOF0_END_BLOCK_HASH = repeat32(0x0b);
pub const PROOF0_END_BLOCK_NUMBER: u64 = 11;
pub const PROOF0_END_BLOCK_TIMESTAMP: u64 = 1763000200;
pub const PROOF0_L2_L1_MESSAGES_HASH = repeat32(0x06);
pub const PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x02);
pub const PROOF0_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 0;
pub const PROOF0_END_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x03);
pub const PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 4;
pub const PROOF0_DYNAMIC_CHAIN_CONFIG_HASH = repeat32(0xc0);
pub const PROOF0_PARENT_FTX_ROLLING_HASH = repeat32(0x04);
pub const PROOF0_PARENT_FTX_NUMBER: u64 = 15;
pub const PROOF0_END_FTX_ROLLING_HASH = repeat32(0x05);
pub const PROOF0_END_PROCESSED_FTX_NUMBER: u64 = 18;
pub const PROOF0_FILTERED_ADDRESSES_HASH = repeat32(0x07);
pub const PROOF0_TX_FROMS_HASH = repeat32(0x08);
pub const PROOF0_L2_L1_MESSAGE_0 = repeat32(0x08);
pub const PROOF0_TX_FROM_0 = repeat20(0x01);
pub const PROOF0_TX_FROM_1 = repeat20(0x02);
pub const PROOF0_FILTERED_ADDRESS_0 = repeat20(0x03);
pub const PROOF0_FILTERED_ADDRESS_1 = repeat20(0x04);

// ── l2_execution_proofs[1] ────────────────────────────────────────────────────────────────────
pub const PROOF1_START_BLOCK_NUMBER: u64 = 12;
pub const PROOF1_PROOF_BYTES = [_]u8{ 0xab, 0xcd, 0xff };
pub const PROOF1_PARENT_BLOCK_HASH = repeat32(0x0b);
pub const PROOF1_END_BLOCK_HASH = repeat32(0x0b);
pub const PROOF1_END_BLOCK_NUMBER: u64 = 14;
pub const PROOF1_END_BLOCK_TIMESTAMP: u64 = 1763000210;
pub const PROOF1_L2_L1_MESSAGES_HASH = repeat32(0x06);
pub const PROOF1_PARENT_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x03);
pub const PROOF1_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 4;
pub const PROOF1_END_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x03);
pub const PROOF1_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 4;
pub const PROOF1_DYNAMIC_CHAIN_CONFIG_HASH = repeat32(0xc0);
pub const PROOF1_PARENT_FTX_ROLLING_HASH = repeat32(0x05);
pub const PROOF1_PARENT_FTX_NUMBER: u64 = 18;
pub const PROOF1_END_FTX_ROLLING_HASH = repeat32(0x05);
pub const PROOF1_END_PROCESSED_FTX_NUMBER: u64 = 18;
pub const PROOF1_FILTERED_ADDRESSES_HASH = repeat32(0x07);
pub const PROOF1_TX_FROMS_HASH = repeat32(0x08);
pub const PROOF1_FILTERED_ADDRESS_0 = repeat20(0x03);
pub const PROOF1_FILTERED_ADDRESS_1 = repeat20(0x04);

/// A readable `RollupProofPrivateInput` built entirely from the named constants above.
pub fn sampleInput(alloc: std.mem.Allocator) !rollup_ssz.RollupProofPrivateInput {
    const conflations = try alloc.dupe(rollup_ssz.ConflationWitness, &[_]rollup_ssz.ConflationWitness{
        .{ .block_rlps = try alloc.dupe([]const u8, &[_][]const u8{
            &CONFLATION_0_BLOCK_RLP_0,
            &CONFLATION_0_BLOCK_RLP_1,
        }) },
        .{ .block_rlps = try alloc.dupe([]const u8, &[_][]const u8{
            &CONFLATION_1_BLOCK_RLP_0,
            &CONFLATION_1_BLOCK_RLP_1,
        }) },
    });

    const proof0: rollup_ssz.VerifiableL2ExecutionProof = .{
        .program_vk = SHARED_PROGRAM_VK,
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = PROOF0_PARENT_BLOCK_HASH,
                .end_block_hash = PROOF0_END_BLOCK_HASH,
                .end_block_number = PROOF0_END_BLOCK_NUMBER,
                .end_block_timestamp = PROOF0_END_BLOCK_TIMESTAMP,
                .l2_l1_messages_hash = PROOF0_L2_L1_MESSAGES_HASH,
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
                .tx_froms_hash = PROOF0_TX_FROMS_HASH,
            },
            .start_block_number = PROOF0_START_BLOCK_NUMBER,
            .proof = &PROOF0_PROOF_BYTES,
            .l2_l1_messages = try alloc.dupe([32]u8, &[_][32]u8{PROOF0_L2_L1_MESSAGE_0}),
            .tx_froms = try alloc.dupe([20]u8, &[_][20]u8{ PROOF0_TX_FROM_0, PROOF0_TX_FROM_1 }),
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ PROOF0_FILTERED_ADDRESS_0, PROOF0_FILTERED_ADDRESS_1 }),
        },
    };
    const proof1: rollup_ssz.VerifiableL2ExecutionProof = .{
        .program_vk = SHARED_PROGRAM_VK,
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = PROOF1_PARENT_BLOCK_HASH,
                .end_block_hash = PROOF1_END_BLOCK_HASH,
                .end_block_number = PROOF1_END_BLOCK_NUMBER,
                .end_block_timestamp = PROOF1_END_BLOCK_TIMESTAMP,
                .l2_l1_messages_hash = PROOF1_L2_L1_MESSAGES_HASH,
                .parent_l1_l2_bridge_rolling_hash = PROOF1_PARENT_L1L2_BRIDGE_ROLLING_HASH,
                .parent_l1_l2_bridge_rolling_hash_message_number = PROOF1_PARENT_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER,
                .end_l1_l2_bridge_rolling_hash = PROOF1_END_L1L2_BRIDGE_ROLLING_HASH,
                .end_l1_l2_bridge_rolling_hash_message_number = PROOF1_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER,
                .dynamic_chain_config_hash = PROOF1_DYNAMIC_CHAIN_CONFIG_HASH,
                .parent_ftx_rolling_hash = PROOF1_PARENT_FTX_ROLLING_HASH,
                .parent_ftx_number = PROOF1_PARENT_FTX_NUMBER,
                .end_ftx_rolling_hash = PROOF1_END_FTX_ROLLING_HASH,
                .end_processed_ftx_number = PROOF1_END_PROCESSED_FTX_NUMBER,
                .filtered_addresses_hash = PROOF1_FILTERED_ADDRESSES_HASH,
                .tx_froms_hash = PROOF1_TX_FROMS_HASH,
            },
            .start_block_number = PROOF1_START_BLOCK_NUMBER,
            .proof = &PROOF1_PROOF_BYTES,
            .l2_l1_messages = &.{},
            .tx_froms = &.{},
            .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ PROOF1_FILTERED_ADDRESS_0, PROOF1_FILTERED_ADDRESS_1 }),
        },
    };

    return .{
        .parent_data_rolling_hash = PARENT_DATA_ROLLING_HASH,
        .start_offset = START_OFFSET,
        .chain_id = CHAIN_ID,
        .conflations = conflations,
        .chunks = try alloc.dupe([32]u8, &[_][32]u8{CHUNK_0}),
        .l2_execution_proofs = try alloc.dupe(rollup_ssz.VerifiableL2ExecutionProof, &[_]rollup_ssz.VerifiableL2ExecutionProof{ proof0, proof1 }),
        .opaque_prefix_bytes = &OPAQUE_PREFIX_BYTES,
        .opaque_suffix_bytes = &.{},
        .boundary_prev_data_rolling_hash = BOUNDARY_PREV_DATA_ROLLING_HASH,
    };
}
