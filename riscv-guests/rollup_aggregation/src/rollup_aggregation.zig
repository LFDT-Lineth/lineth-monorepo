//! The rollup-aggregation guest statement: `RollupAggregationInput -> FinalizationPublicInput`. One
//! aggregation proof covers M consecutive rollup proofs: it verifies each recursively, checks that
//! they tile the block range and continue one another, and assembles the public inputs of the whole
//! range.

const std = @import("std");
const core_serde = @import("core_serde");
const guest_common = @import("guest_common");
const lineth_recursion = @import("lineth_recursion");
const l2_l1_roots = @import("l2_l1_roots.zig");
const rollup_aggregation_ssz = @import("rollup_aggregation_ssz.zig");

const Allocator = std.mem.Allocator;
const l1_l2_bridge = guest_common.l1_l2_bridge;
const lists = guest_common.lists;
const messaging_offsets = guest_common.messaging_offsets;
const recursion = lineth_recursion.recursion;
const rollup_proof = core_serde.rollup_proof;

pub const Error = messaging_offsets.Error || recursion.Error || error{
    /// The request has no rollup proof.
    InvalidRequest,
    /// A proof starts at another block than the one after its predecessor's end.
    TilingBroken,
    /// Two consecutive proofs disagree on the data rolling hash, the shared-blob tail, the block
    /// hash, the chain configuration or the forced-transaction state at their junction.
    ContinuityBroken,
};

/// The framed output for the framed input `raw_input`: the statement over the decoded input, its
/// public inputs encoded. Everything is allocated from `allocator`.
pub fn runFramed(allocator: Allocator, raw_input: []const u8) ![]const u8 {
    const decoded = try rollup_aggregation_ssz.decodeInput(allocator, raw_input);
    const public_inputs = try run(allocator, decoded);
    return rollup_aggregation_ssz.encodeOutput(allocator, public_inputs) catch |err| switch (err) {
        error.OutOfMemory => error.OutOfMemory,
        error.BoundsViolation => error.OutputEncodeFailed,
    };
}

/// The public inputs of the aggregation proof over `input`, or the error of the first rule it
/// breaks. Everything is allocated from `allocator`, a bump allocator whose memory is reclaimed as
/// a whole; the public inputs alias it. The output encoder rejects a list beyond its wire schema's
/// bound.
pub fn run(allocator: Allocator, input: rollup_aggregation_ssz.RollupAggregationInput) Error!rollup_aggregation_ssz.FinalizationPublicInput {
    const proofs = input.rollup_proofs.items;
    if (proofs.len == 0) return error.InvalidRequest;

    const recursive_verifier = try recursion.Verifier.init(allocator, proofs);
    for (proofs) |verifiable| try verifyRollupProof(allocator, recursive_verifier, verifiable);
    for (proofs[0 .. proofs.len - 1], proofs[1..]) |left, right| try verifyJunction(left.proof, right.proof);

    const first = proofs[0].proof;
    const last = proofs[proofs.len - 1].proof;
    const range_start = first.start_block_number;

    var message_count: usize = 0;
    var address_count: usize = 0;
    var offset_count: usize = 0;
    for (proofs) |verifiable| {
        const pi = verifiable.proof.public_inputs;
        message_count += pi.l2_l1_messages.items.len;
        address_count += pi.filtered_addresses.items.len;
        offset_count += pi.l2_messaging_blocks_offsets.items.len;
    }
    const messages = try allocator.alloc([32]u8, message_count);
    const addresses = try allocator.alloc([20]u8, address_count);
    const offsets = try allocator.alloc(u16, offset_count);
    var message_pos: usize = 0;
    var address_pos: usize = 0;
    var offset_pos: usize = 0;
    var end_bridge: l1_l2_bridge.Pair = .none;
    for (proofs) |verifiable| {
        const pi = verifiable.proof.public_inputs;
        @memcpy(messages[message_pos..][0..pi.l2_l1_messages.items.len], pi.l2_l1_messages.items);
        message_pos += pi.l2_l1_messages.items.len;
        @memcpy(addresses[address_pos..][0..pi.filtered_addresses.items.len], pi.filtered_addresses.items);
        address_pos += pi.filtered_addresses.items.len;
        const proof_offsets = offsets[offset_pos..][0..pi.l2_messaging_blocks_offsets.items.len];
        try messaging_offsets.rebase(.{
            .start_block_number = verifiable.proof.start_block_number,
            .end_block_number = pi.end_block_number,
            .block_count = pi.block_count,
            .offsets = pi.l2_messaging_blocks_offsets.items,
        }, range_start, proof_offsets);
        offset_pos += proof_offsets.len;
        end_bridge = l1_l2_bridge.advance(end_bridge, pi);
    }

    return .{
        .end_block_number = last.public_inputs.end_block_number,
        .end_block_timestamp = last.public_inputs.end_block_timestamp,
        .end_l1_l2_bridge_rolling_hash = end_bridge.rolling_hash,
        .end_l1_l2_bridge_rolling_hash_message_number = end_bridge.message_number,
        .dynamic_chain_config_hash = first.public_inputs.dynamic_chain_config_hash,
        .parent_ftx_rolling_hash = first.public_inputs.parent_ftx_rolling_hash,
        .parent_ftx_number = first.public_inputs.parent_ftx_number,
        .end_ftx_rolling_hash = last.public_inputs.end_ftx_rolling_hash,
        .end_processed_ftx_number = last.public_inputs.end_processed_ftx_number,
        .parent_data_rolling_hash = first.public_inputs.parent_data_rolling_hash,
        .end_data_rolling_hash = last.public_inputs.end_data_rolling_hash,
        .parent_block_hash = first.public_inputs.parent_block_hash,
        .end_block_hash = last.public_inputs.end_block_hash,
        .parent_shared_blob_carry_over_tail_bytes = first.public_inputs.parent_shared_blob_carry_over_tail_bytes,
        .final_shared_blob_carry_over_tail_bytes = last.public_inputs.final_shared_blob_carry_over_tail_bytes,
        .l2_l1_tree_depth = l2_l1_roots.tree_depth,
        .l2_l1_roots = .{ .items = try l2_l1_roots.build(allocator, messages) },
        .filtered_addresses = .{ .items = lists.sortedDistinct(20, addresses) },
        .program_ids = .{ .items = try programIds(allocator, proofs) },
        .l2_messaging_blocks_offsets = .{ .items = offsets },
    };
}

/// Verifies a rollup proof under the program VK it carries, against the hash of its public input,
/// which is rejected when beyond a bound of its wire schema. `allocator` holds the encoding that is
/// hashed.
fn verifyRollupProof(allocator: Allocator, recursive_verifier: recursion.Verifier, verifiable: rollup_proof.VerifiableRollupProof) Error!void {
    const public_input_hash = rollup_proof.hashPublicInputs(allocator, verifiable.proof.public_inputs) catch |err| switch (err) {
        error.OutOfMemory => return error.OutOfMemory,
        error.BoundsViolation => return error.RecursiveProofRejected,
    };
    try recursive_verifier.verify(verifiable.program_vk, verifiable.proof.proof.bytes, public_input_hash);
}

/// `right` starts right after the end of `left` and continues it: its parent data rolling hash,
/// shared-blob tail, parent block hash and forced-transaction state, and its chain configuration,
/// are the ones `left` ends with.
fn verifyJunction(left: rollup_proof.RollupProof, right: rollup_proof.RollupProof) Error!void {
    if (right.start_block_number != @as(u128, left.public_inputs.end_block_number) + 1) return error.TilingBroken;
    const l = left.public_inputs;
    const r = right.public_inputs;
    if (!std.mem.eql(u8, &l.end_data_rolling_hash, &r.parent_data_rolling_hash) or
        l.final_shared_blob_carry_over_tail_bytes != r.parent_shared_blob_carry_over_tail_bytes or
        !std.mem.eql(u8, &l.end_block_hash, &r.parent_block_hash) or
        !std.mem.eql(u8, &l.dynamic_chain_config_hash, &r.dynamic_chain_config_hash) or
        !std.mem.eql(u8, &l.end_ftx_rolling_hash, &r.parent_ftx_rolling_hash) or
        l.end_processed_ftx_number != r.parent_ftx_number) return error.ContinuityBroken;
}

/// The program IDs of the verified programs, distinct and ascending. A program VK is its own ID.
fn programIds(allocator: Allocator, proofs: []const rollup_proof.VerifiableRollupProof) Allocator.Error![][32]u8 {
    return guest_common.program_vks.collect(allocator, proofs);
}
