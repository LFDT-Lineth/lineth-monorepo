//! Shared test-only fixtures for the wire codec of the rollup guest: one `sampleInput`, used by the
//! round-trip test and the malformed-input tests, so there is exactly one place that defines what a
//! `RollupProofPrivateInput` with every field populated looks like on the wire. Its values are
//! wire-format filler: the conflations, chunks and proofs do not agree with each other.

const std = @import("std");
const rollup_ssz = @import("rollup").rollup_ssz;
const l2_execution_proof = @import("core_serde").l2_execution_proof;

pub fn repeat32(byte: u8) [32]u8 {
    return @splat(byte);
}

fn repeat20(byte: u8) [20]u8 {
    return @splat(byte);
}

// ── Top-level input fields ───────────────────────────────────────────────────────────────────
pub const PARENT_DATA_ROLLING_HASH = repeat32(0x47);
pub const PARENT_DATA_TAIL_TAKE_BYTES: u64 = 4;
pub const CHAIN_ID: u64 = 59144;
pub const BOUNDARY_PREV_DATA_ROLLING_HASH = repeat32(0x39);

// ── Conflation block RLPs ─────────────────────────────────────────────────────────────────────
// RLP-shaped filler (a valid "long list" / "32-byte string" prefix), not real RLP: the codec carries
// `block_rlps` as bytes, so only byte identity across encode/decode matters here.
const CONFLATION_0_BLOCK_RLP_0 = [_]u8{ 0xf9, 0x02, 0x15, 0xa0 };
const CONFLATION_0_BLOCK_RLP_1 = [_]u8{ 0xf9, 0x02, 0x16, 0xb1 };
const CONFLATION_1_BLOCK_RLP_0 = [_]u8{ 0xf9, 0x02, 0x15, 0xaa };
const CONFLATION_1_BLOCK_RLP_1 = [_]u8{ 0xf9, 0x02, 0x16, 0xbb };

// ── Chunks ────────────────────────────────────────────────────────────────────────────────────
// A blob of `rollup_ssz.BLOB_BYTES_LENGTH` bytes where byte `i` is `i % BLOB_BYTE_PERIOD`, with its
// commitment and proof, and a calldata submission.
pub const BLOB_CHUNK_HASH = repeat32(0x1a);
const BLOB_BYTE_PERIOD = 251;
pub const BLOB_KZG_COMMITMENT = [_]u8{0xc1} ** rollup_ssz.BYTES_PER_COMMITMENT;
pub const BLOB_KZG_PROOF = [_]u8{0xb2} ** rollup_ssz.BYTES_PER_PROOF;
pub const CALLDATA_CHUNK_HASH = repeat32(0x2a);
pub const CALLDATA_BYTES = [_]u8{ 0xca, 0x11, 0xda, 0x7a, 0x01 };

// ── l2_execution_proofs[0] ────────────────────────────────────────────────────────────────────
const SHARED_PROGRAM_VK = repeat32(0xaa);

const PROOF0_START_BLOCK_NUMBER: u64 = 10;
const PROOF0_PROOF_BYTES = [_]u8{ 0xab, 0xcd, 0xef };
const PROOF0_PARENT_BLOCK_HASH = repeat32(0x0a);
const PROOF0_END_BLOCK_HASH = repeat32(0x0b);
const PROOF0_END_BLOCK_NUMBER: u64 = 11;
const PROOF0_END_BLOCK_TIMESTAMP: u64 = 1763000200;
const PROOF0_END_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x03);
const PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 4;
const PROOF0_DYNAMIC_CHAIN_CONFIG_HASH = repeat32(0xc0);
const PROOF0_PARENT_FTX_ROLLING_HASH = repeat32(0x04);
const PROOF0_PARENT_FTX_NUMBER: u64 = 15;
const PROOF0_END_FTX_ROLLING_HASH = repeat32(0x05);
const PROOF0_END_PROCESSED_FTX_NUMBER: u64 = 18;
const PROOF0_TX_FROMS_HASH = repeat32(0x08);
const PROOF0_L2_L1_MESSAGE_0 = repeat32(0x08);
const PROOF0_L2_L1_MESSAGE_1 = repeat32(0x18);
const PROOF0_FILTERED_ADDRESS_0 = repeat20(0x04);
const PROOF0_FILTERED_ADDRESS_1 = repeat20(0x03);
const PROOF0_BLOCK_COUNT: u64 = 2;
const PROOF0_MESSAGING_BLOCKS_OFFSETS = [_]u16{ 1, 2 };

// ── l2_execution_proofs[1] ────────────────────────────────────────────────────────────────────
const PROOF1_START_BLOCK_NUMBER: u64 = 12;
const PROOF1_PROOF_BYTES = [_]u8{ 0xab, 0xcd, 0xff };
const PROOF1_PARENT_BLOCK_HASH = repeat32(0x0b);
const PROOF1_END_BLOCK_HASH = repeat32(0x0c);
const PROOF1_END_BLOCK_NUMBER: u64 = 14;
const PROOF1_END_BLOCK_TIMESTAMP: u64 = 1763000210;
const PROOF1_END_L1L2_BRIDGE_ROLLING_HASH = repeat32(0x13);
const PROOF1_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER: u64 = 6;
const PROOF1_DYNAMIC_CHAIN_CONFIG_HASH = repeat32(0xc0);
const PROOF1_PARENT_FTX_ROLLING_HASH = repeat32(0x05);
const PROOF1_PARENT_FTX_NUMBER: u64 = 18;
const PROOF1_END_FTX_ROLLING_HASH = repeat32(0x06);
const PROOF1_END_PROCESSED_FTX_NUMBER: u64 = 19;
const PROOF1_TX_FROMS_HASH = repeat32(0x09);
const PROOF1_L2_L1_MESSAGE_0 = repeat32(0x28);
const PROOF1_FILTERED_ADDRESS_0 = repeat20(0x03);
const PROOF1_FILTERED_ADDRESS_1 = repeat20(0x05);
const PROOF1_BLOCK_COUNT: u64 = 3;
const PROOF1_MESSAGING_BLOCKS_OFFSETS = [_]u16{3};

/// The sample's first l2-execution proof.
pub fn sampleProof0(alloc: std.mem.Allocator) !l2_execution_proof.VerifiableL2ExecutionProof {
    return .{
        .program_vk = SHARED_PROGRAM_VK,
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = PROOF0_PARENT_BLOCK_HASH,
                .end_block_hash = PROOF0_END_BLOCK_HASH,
                .end_block_number = PROOF0_END_BLOCK_NUMBER,
                .end_block_timestamp = PROOF0_END_BLOCK_TIMESTAMP,
                .l2_l1_messages = try alloc.dupe([32]u8, &[_][32]u8{ PROOF0_L2_L1_MESSAGE_0, PROOF0_L2_L1_MESSAGE_1 }),
                .end_l1_l2_bridge_rolling_hash = PROOF0_END_L1L2_BRIDGE_ROLLING_HASH,
                .end_l1_l2_bridge_rolling_hash_message_number = PROOF0_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER,
                .dynamic_chain_config_hash = PROOF0_DYNAMIC_CHAIN_CONFIG_HASH,
                .parent_ftx_rolling_hash = PROOF0_PARENT_FTX_ROLLING_HASH,
                .parent_ftx_number = PROOF0_PARENT_FTX_NUMBER,
                .end_ftx_rolling_hash = PROOF0_END_FTX_ROLLING_HASH,
                .end_processed_ftx_number = PROOF0_END_PROCESSED_FTX_NUMBER,
                .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ PROOF0_FILTERED_ADDRESS_0, PROOF0_FILTERED_ADDRESS_1 }),
                .tx_froms_hash = PROOF0_TX_FROMS_HASH,
                .block_count = PROOF0_BLOCK_COUNT,
                .l2_messaging_blocks_offsets = &PROOF0_MESSAGING_BLOCKS_OFFSETS,
            },
            .start_block_number = PROOF0_START_BLOCK_NUMBER,
            .proof = &PROOF0_PROOF_BYTES,
        },
    };
}

/// The sample's second l2-execution proof.
fn sampleProof1(alloc: std.mem.Allocator) !l2_execution_proof.VerifiableL2ExecutionProof {
    return .{
        .program_vk = SHARED_PROGRAM_VK,
        .proof = .{
            .public_inputs = .{
                .parent_block_hash = PROOF1_PARENT_BLOCK_HASH,
                .end_block_hash = PROOF1_END_BLOCK_HASH,
                .end_block_number = PROOF1_END_BLOCK_NUMBER,
                .end_block_timestamp = PROOF1_END_BLOCK_TIMESTAMP,
                .l2_l1_messages = try alloc.dupe([32]u8, &[_][32]u8{PROOF1_L2_L1_MESSAGE_0}),
                .end_l1_l2_bridge_rolling_hash = PROOF1_END_L1L2_BRIDGE_ROLLING_HASH,
                .end_l1_l2_bridge_rolling_hash_message_number = PROOF1_END_L1L2_BRIDGE_ROLLING_HASH_MSG_NUMBER,
                .dynamic_chain_config_hash = PROOF1_DYNAMIC_CHAIN_CONFIG_HASH,
                .parent_ftx_rolling_hash = PROOF1_PARENT_FTX_ROLLING_HASH,
                .parent_ftx_number = PROOF1_PARENT_FTX_NUMBER,
                .end_ftx_rolling_hash = PROOF1_END_FTX_ROLLING_HASH,
                .end_processed_ftx_number = PROOF1_END_PROCESSED_FTX_NUMBER,
                .filtered_addresses = try alloc.dupe([20]u8, &[_][20]u8{ PROOF1_FILTERED_ADDRESS_0, PROOF1_FILTERED_ADDRESS_1 }),
                .tx_froms_hash = PROOF1_TX_FROMS_HASH,
                .block_count = PROOF1_BLOCK_COUNT,
                .l2_messaging_blocks_offsets = &PROOF1_MESSAGING_BLOCKS_OFFSETS,
            },
            .start_block_number = PROOF1_START_BLOCK_NUMBER,
            .proof = &PROOF1_PROOF_BYTES,
        },
    };
}

/// The sample's blob of `rollup_ssz.BLOB_BYTES_LENGTH` bytes.
pub fn sampleBlob(alloc: std.mem.Allocator) ![]u8 {
    const blob = try alloc.alloc(u8, rollup_ssz.BLOB_BYTES_LENGTH);
    for (blob, 0..) |*byte, i| byte.* = @intCast(i % BLOB_BYTE_PERIOD);
    return blob;
}

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

    return .{
        .parent_data_rolling_hash = PARENT_DATA_ROLLING_HASH,
        .parent_data_tail_take_bytes = PARENT_DATA_TAIL_TAKE_BYTES,
        .chain_id = CHAIN_ID,
        .conflations = conflations,
        .chunks = try alloc.dupe(rollup_ssz.ChunkWitness, &[_]rollup_ssz.ChunkWitness{
            .{
                .chunk_hash = BLOB_CHUNK_HASH,
                .is_calldata = false,
                .blob_bytes = try sampleBlob(alloc),
                .kzg_commitment = &BLOB_KZG_COMMITMENT,
                .kzg_proof = &BLOB_KZG_PROOF,
            },
            .{ .chunk_hash = CALLDATA_CHUNK_HASH, .is_calldata = true, .calldata_bytes = &CALLDATA_BYTES },
        }),
        .l2_execution_proofs = try alloc.dupe(l2_execution_proof.VerifiableL2ExecutionProof, &[_]l2_execution_proof.VerifiableL2ExecutionProof{
            try sampleProof0(alloc),
            try sampleProof1(alloc),
        }),
        .boundary_prev_data_rolling_hash = BOUNDARY_PREV_DATA_ROLLING_HASH,
    };
}
