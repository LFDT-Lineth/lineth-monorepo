//! Rollup-aggregation guest business logic: `RollupAggregationProofPrivateInput ->
//! RollupAggregationOutput`, entirely by echo or sentinel. Every output field is either copied
//! from a defined place in the input, or set to a fixed, precomputed sentinel constant — nothing
//! is computed (no hashing, no accumulator folding, no proof verification). This guest exercises
//! the wire format and its own decode/encode bounds, not the rollup-aggregation's real
//! recursive-proof logic.

const std = @import("std");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz");

// ── Sentinels ─────────────────────────────────────────────────────────────────────────────────
// Each value is keccak256 of the tag string next to it, as a single hex string rather than a byte
// array — pasteable into any external keccak calculator to re-derive and check. u64 sentinels take
// the first 8 bytes, big-endian. Zig has no comptime keccak here (the accelerator's keccak is a
// runtime opcode), so the hex is precomputed; `test_stub_sentinels.py` in rollup_spec independently
// recomputes keccak256 of each tag string and asserts it matches the hex below.

fn hexToArray32(comptime hex: *const [64]u8) [32]u8 {
    var out: [32]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return out;
}

fn hexToU64(comptime hex: *const [16]u8) u64 {
    var out: [8]u8 = undefined;
    _ = std.fmt.hexToBytes(&out, hex) catch unreachable;
    return std.mem.readInt(u64, &out, .big);
}

// tag: "lineth.stub.rollup-aggregation.l2L1BridgeTransactionTree"
pub const L2_L1_BRIDGE_TRANSACTION_TREE: [32]u8 = hexToArray32("0918836198239a5edf0936db3e28a64d5c4e195fd728e6ddfc3544fc95008ab3");
// tag: "lineth.stub.rollup-aggregation.filteredAddressesHash"
pub const FILTERED_ADDRESSES_HASH: [32]u8 = hexToArray32("63d40d3ea387065027b369a25cc441dd7685e2a9a9e0e56556ec2e82bbe2273c");
// tag: "lineth.stub.rollup-aggregation.l2MessagingBlocksOffsets" (first 8 bytes)
pub const L2_MESSAGING_BLOCKS_OFFSETS_ELEMENT: u64 = hexToU64("d18d873fe2a9f192");

/// Runs the rollup-aggregation guest's echo/sentinel mapping over a decoded input. Requires at
/// least one `rollup_proofs` element — "first"/"last" source every per-proof field.
pub fn run(alloc: std.mem.Allocator, input: rollup_aggregation_ssz.RollupAggregationProofPrivateInput) !rollup_aggregation_ssz.RollupAggregationOutput {
    if (input.rollup_proofs.len == 0) return error.EmptyProofs;
    const first = input.rollup_proofs[0];
    const last = input.rollup_proofs[input.rollup_proofs.len - 1];

    const program_vks = try dedupSortedProgramVks(alloc, input.rollup_proofs);

    var l2_l1_roots: std.ArrayListUnmanaged([32]u8) = .empty;
    var filtered_addresses: std.ArrayListUnmanaged([20]u8) = .empty;
    for (input.rollup_proofs) |p| {
        for (p.proof.l2_l1_roots) |root| try l2_l1_roots.append(alloc, root);
        for (p.proof.filtered_addresses) |addr| try filtered_addresses.append(alloc, addr);
    }

    const l2_messaging_blocks_offsets = try alloc.dupe(u64, &[_]u64{L2_MESSAGING_BLOCKS_OFFSETS_ELEMENT});

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
            .parent_data_rolling_hash = first.proof.public_inputs.parent_data_rolling_hash,
            .end_data_rolling_hash = last.proof.public_inputs.end_data_rolling_hash,
            .parent_block_hash = first.proof.public_inputs.parent_block_hash,
            .end_block_hash = last.proof.public_inputs.end_block_hash,
            .start_offset = first.proof.public_inputs.start_offset,
            .end_offset = last.proof.public_inputs.end_offset,
            .program_vks = program_vks,
        },
        .l2_l1_roots = try l2_l1_roots.toOwnedSlice(alloc),
        .filtered_addresses = try filtered_addresses.toOwnedSlice(alloc),
        .l2_messaging_blocks_offsets = l2_messaging_blocks_offsets,
    };
}

fn lessThanBytes32(_: void, a: [32]u8, b: [32]u8) bool {
    return std.mem.order(u8, &a, &b) == .lt;
}

/// The union of every embedded `RollupPublicInput.program_vks` and every `rollup_proofs` element's
/// own `program_vk`, deduplicated and sorted ascending bytewise.
fn dedupSortedProgramVks(alloc: std.mem.Allocator, proofs: []const rollup_aggregation_ssz.VerifiableRollupProof) ![]const [32]u8 {
    var all: std.ArrayListUnmanaged([32]u8) = .empty;
    for (proofs) |p| {
        try all.append(alloc, p.program_vk);
        for (p.proof.public_inputs.program_vks) |vk| try all.append(alloc, vk);
    }
    std.mem.sort([32]u8, all.items, {}, lessThanBytes32);

    var out: std.ArrayListUnmanaged([32]u8) = .empty;
    for (all.items, 0..) |vk, i| {
        if (i == 0 or !std.mem.eql(u8, &vk, &all.items[i - 1])) try out.append(alloc, vk);
    }
    return out.toOwnedSlice(alloc);
}
