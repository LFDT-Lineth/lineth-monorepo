//! Rollup guest business logic: `RollupProofPrivateInput -> RollupOutput`, entirely by echo or
//! sentinel. Every output field is either copied from a defined place in the input, or set to a
//! fixed, precomputed sentinel constant — nothing is computed (no hashing, no accumulator folding,
//! no proof verification). This guest exercises the wire format and its own decode/encode bounds,
//! not the rollup's real recursive-proof logic.

const std = @import("std");
const rollup_ssz = @import("rollup_ssz");

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

// tag: "lineth.stub.rollup.l2L1BridgeTransactionTree"
pub const L2_L1_BRIDGE_TRANSACTION_TREE: [32]u8 = hexToArray32("bc436fcfbb175835d12e0a12f4534f60cd92dbd4babf87db2339277a72ecd22a");
// tag: "lineth.stub.rollup.endDataRollingHash"
pub const END_DATA_ROLLING_HASH: [32]u8 = hexToArray32("1fe617c10b3bfc97fd6d0090c608df47de544a4b7d4f6379300bd96167da2ada");
// tag: "lineth.stub.rollup.filteredAddressesHash"
pub const FILTERED_ADDRESSES_HASH: [32]u8 = hexToArray32("8fa4b00e95cd0784a49f00efe0d12f67715a99a6cf54ac4ca5606ebc4d0a42ba");
// tag: "lineth.stub.rollup.endOffset" (first 8 bytes)
pub const END_OFFSET: u64 = hexToU64("1ab5956f53caf2ea");
// tag: "lineth.stub.rollup.l2L1Roots"
pub const L2_L1_ROOTS_ELEMENT: [32]u8 = hexToArray32("45c25758659787f96843b2171dd2091c964ee7fe11518fabf1a4c944b4f75e0e");

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
