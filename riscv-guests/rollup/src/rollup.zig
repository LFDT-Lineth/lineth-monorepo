//! Rollup guest business logic: `RollupProofPrivateInput -> RollupOutput`, entirely by echo or
//! sentinel (Readme.md's per-field provenance table). Every output field is either copied from a
//! defined place in the input, or set to a fixed, precomputed sentinel constant — nothing is
//! computed (no hashing, no accumulator folding, no proof verification). This guest exercises the
//! wire format and its own decode/encode bounds, not the rollup's real recursive-proof logic.

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");

// ── Sentinels ─────────────────────────────────────────────────────────────────────────────────
// Each is `keccak256("lineth.stub.rollup.<fieldCamelCaseName>")`, precomputed; `END_OFFSET` takes
// the first 8 bytes of its 32-byte hash, big-endian.
pub const L2_L1_BRIDGE_TRANSACTION_TREE: [32]u8 = .{ 0xbc, 0x43, 0x6f, 0xcf, 0xbb, 0x17, 0x58, 0x35, 0xd1, 0x2e, 0x0a, 0x12, 0xf4, 0x53, 0x4f, 0x60, 0xcd, 0x92, 0xdb, 0xd4, 0xba, 0xbf, 0x87, 0xdb, 0x23, 0x39, 0x27, 0x7a, 0x72, 0xec, 0xd2, 0x2a };
pub const END_DATA_ROLLING_HASH: [32]u8 = .{ 0x1f, 0xe6, 0x17, 0xc1, 0x0b, 0x3b, 0xfc, 0x97, 0xfd, 0x6d, 0x00, 0x90, 0xc6, 0x08, 0xdf, 0x47, 0xde, 0x54, 0x4a, 0x4b, 0x7d, 0x4f, 0x63, 0x79, 0x30, 0x0b, 0xd9, 0x61, 0x67, 0xda, 0x2a, 0xda };
pub const FILTERED_ADDRESSES_HASH: [32]u8 = .{ 0x8f, 0xa4, 0xb0, 0x0e, 0x95, 0xcd, 0x07, 0x84, 0xa4, 0x9f, 0x00, 0xef, 0xe0, 0xd1, 0x2f, 0x67, 0x71, 0x5a, 0x99, 0xa6, 0xcf, 0x54, 0xac, 0x4c, 0xa5, 0x60, 0x6e, 0xbc, 0x4d, 0x0a, 0x42, 0xba };
pub const END_OFFSET: u64 = 0x1ab5956f53caf2ea;
pub const L2_L1_ROOTS_ELEMENT: [32]u8 = .{ 0x45, 0xc2, 0x57, 0x58, 0x65, 0x97, 0x87, 0xf9, 0x68, 0x43, 0xb2, 0x17, 0x1d, 0xd2, 0x09, 0x1c, 0x96, 0x4e, 0xe7, 0xfe, 0x11, 0x51, 0x8f, 0xab, 0xf1, 0xa4, 0xc9, 0x44, 0xb4, 0xf7, 0x5e, 0x0e };

/// Runs the rollup guest's echo/sentinel mapping over a decoded input (Readme.md's field-provenance
/// table). Requires at least one `l2_execution_proofs` element — "first"/"last" source every
/// per-proof field.
pub fn run(alloc: std.mem.Allocator, input: rollup_ssz.RollupProofPrivateInput) !rollup_ssz.RollupOutput {
    if (input.l2_execution_proofs.len == 0) return error.EmptyProofs;
    const first = input.l2_execution_proofs[0];
    const last = input.l2_execution_proofs[input.l2_execution_proofs.len - 1];

    const program_vks = try dedupSortedProgramVks(alloc, input.l2_execution_proofs);

    var filtered_addresses: std.ArrayListUnmanaged([20]u8) = .empty;
    for (input.l2_execution_proofs) |p| {
        for (p.proof.filtered_addresses) |addr| try filtered_addresses.append(alloc, addr);
    }

    const l2_l1_roots = try alloc.dupe([32]u8, &[_][32]u8{L2_L1_ROOTS_ELEMENT});

    return .{
        .public_inputs = .{
            .end_block_number = last.proof.public_inputs.end_block_number,
            .end_block_timestamp = last.proof.public_inputs.end_block_timestamp,
            .l2_l1_bridge_transaction_tree = L2_L1_BRIDGE_TRANSACTION_TREE,
            .parent_l1_l2_bridge_rolling_hash = first.proof.public_inputs.parent_l1_l2_bridge_rolling_hash,
            .parent_l1_l2_bridge_rolling_hash_message_number = first.proof.public_inputs.parent_l1_l2_bridge_rolling_hash_message_number,
            .end_l1_l2_bridge_rolling_hash = last.proof.public_inputs.end_l1_l2_bridge_rolling_hash,
            .end_l1_l2_bridge_rolling_hash_message_number = last.proof.public_inputs.end_l1_l2_bridge_rolling_hash_message_number,
            .dynamic_chain_config_hash = first.proof.public_inputs.dynamic_chain_config_hash,
            .parent_ftx_rolling_hash = first.proof.public_inputs.parent_ftx_rolling_hash,
            .parent_ftx_number = first.proof.public_inputs.parent_ftx_number,
            .end_ftx_rolling_hash = last.proof.public_inputs.end_ftx_rolling_hash,
            .end_processed_ftx_number = last.proof.public_inputs.end_processed_ftx_number,
            .filtered_addresses_hash = FILTERED_ADDRESSES_HASH,
            .parent_data_rolling_hash = input.parent_data_rolling_hash,
            .end_data_rolling_hash = END_DATA_ROLLING_HASH,
            .parent_block_hash = first.proof.public_inputs.parent_block_hash,
            .end_block_hash = last.proof.public_inputs.end_block_hash,
            .start_offset = input.start_offset,
            .end_offset = END_OFFSET,
            .program_vks = program_vks,
        },
        .start_block_number = first.proof.start_block_number,
        .l2_l1_roots = l2_l1_roots,
        .filtered_addresses = try filtered_addresses.toOwnedSlice(alloc),
    };
}

fn lessThanBytes32(_: void, a: [32]u8, b: [32]u8) bool {
    return std.mem.order(u8, &a, &b) == .lt;
}

/// Every `l2_execution_proofs` element's `program_vk`, deduplicated and sorted ascending bytewise.
fn dedupSortedProgramVks(alloc: std.mem.Allocator, proofs: []const rollup_ssz.VerifiableL2ExecutionProof) ![]const [32]u8 {
    const vks = try alloc.alloc([32]u8, proofs.len);
    for (proofs, 0..) |p, i| vks[i] = p.program_vk;
    std.mem.sort([32]u8, vks, {}, lessThanBytes32);

    var out: std.ArrayListUnmanaged([32]u8) = .empty;
    for (vks, 0..) |vk, i| {
        if (i == 0 or !std.mem.eql(u8, &vk, &vks[i - 1])) try out.append(alloc, vk);
    }
    return out.toOwnedSlice(alloc);
}
