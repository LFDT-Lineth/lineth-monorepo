//! Rollup guest business logic: `RollupProofPrivateInput -> RollupOutput`, entirely by echo or
//! sentinel. Every output field is either copied from a defined place in the input, or set to a
//! fixed sentinel constant — nothing is derived from the input itself (no hashing or accumulator
//! folding of request data, no proof verification). This guest exercises the wire format and its
//! own decode/encode bounds, not the rollup's real recursive-proof logic.

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");

// ── Sentinels ─────────────────────────────────────────────────────────────────────────────────
// Each value is keccak256 of its own tag string, computed at comptime — `std.crypto.hash.sha3.
// Keccak256` is pure Zig (the legacy 0x01-delimiter variant, i.e. Ethereum's keccak256, not NIST
// SHA3's 0x06 delimiter), so there is nothing precomputed or pinned to fall out of sync with its
// source string. u64 sentinels take the hash's first 8 bytes, big-endian.

const Keccak256 = std.crypto.hash.sha3.Keccak256;

fn sentinelHash(comptime tag: []const u8) [32]u8 {
    @setEvalBranchQuota(10_000); // one Keccak-f[1600] permutation (24 rounds) exceeds the default 1000
    var out: [32]u8 = undefined;
    Keccak256.hash(tag, &out, .{});
    return out;
}

fn sentinelU64(comptime tag: []const u8) u64 {
    return std.mem.readInt(u64, sentinelHash(tag)[0..8], .big);
}

pub const L2_L1_BRIDGE_TRANSACTION_TREE: [32]u8 = sentinelHash("lineth.stub.rollup.l2L1BridgeTransactionTree");
pub const END_DATA_ROLLING_HASH: [32]u8 = sentinelHash("lineth.stub.rollup.endDataRollingHash");
pub const FILTERED_ADDRESSES_HASH: [32]u8 = sentinelHash("lineth.stub.rollup.filteredAddressesHash");
pub const END_OFFSET: u64 = sentinelU64("lineth.stub.rollup.endOffset");
pub const L2_L1_ROOTS_ELEMENT: [32]u8 = sentinelHash("lineth.stub.rollup.l2L1Roots");

/// Runs the rollup guest's echo/sentinel mapping over a decoded input. Requires at least one
/// `l2_execution_proofs` element — "first"/"last" source every per-proof field.
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
