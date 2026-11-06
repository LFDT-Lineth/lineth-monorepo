//! The rollup guest statement: `RollupProofPrivateInput -> RollupPublicInput`. One rollup proof
//! covers N consecutive whole conflations, each paired with its l2-execution proof, over the chunks
//! of the data-availability stream that carries their compressed payloads.
//!
//! The steps run in this order: the canonical payload of every conflation and its binding to the
//! paired proof (`canonical`), the chunks bound to their hashes and the stream parsed and
//! decompressed against the canonical payloads (`lineth_da.decompress`), the tiling of the proofs,
//! the recursive verification of every proof (`recursion`), the binding of the proofs to the
//! conflations' blocks and the continuity between the proofs (`chain`), and the assembly of the
//! public inputs.

const std = @import("std");
const lineth_da = @import("lineth_da");
const core_serde = @import("core_serde");
const guest_common = @import("guest_common");
const canonical = @import("canonical.zig");
const chain = @import("chain.zig");
const messaging_offsets = @import("messaging_offsets.zig");
const recursion = @import("recursion.zig");
const rollup_ssz = @import("rollup_ssz.zig");

const Allocator = std.mem.Allocator;
const decompress = lineth_da.decompress;
const rollup_proof = core_serde.rollup_proof;

pub const Error = canonical.Error || decompress.Error || chain.Error || messaging_offsets.Error || recursion.Error || error{
    /// The request has no conflation or no proof, or the conflations and the proofs are not paired
    /// one to one.
    InvalidRequest,
};

/// The framed output for the framed input `raw_input`: the statement over the decoded input, its
/// public inputs encoded. Everything is allocated from `allocator`.
pub fn runFramed(allocator: Allocator, raw_input: []const u8) ![]const u8 {
    const decoded = try rollup_ssz.decodeInput(allocator, raw_input);
    const public_inputs = try run(allocator, decoded);
    return rollup_proof.encodeOutput(allocator, public_inputs) catch |err| switch (err) {
        error.OutOfMemory => error.OutOfMemory,
        error.BoundsViolation => error.OutputEncodeFailed,
    };
}

/// The public inputs of the rollup proof over `input`, or the error of the first rule it breaks.
/// Everything is allocated from `allocator`, a bump allocator whose memory is reclaimed as a whole;
/// the public inputs alias it. The output encoder rejects a list beyond its wire schema's bound.
pub fn run(allocator: Allocator, input: rollup_ssz.RollupProofPrivateInput) Error!rollup_proof.RollupPublicInput {
    const proofs = input.l2_execution_proofs;
    if (input.conflations.len == 0 or proofs.len == 0 or input.conflations.len != proofs.len) return error.InvalidRequest;

    const conflations = try allocator.alloc(canonical.Conflation, proofs.len);
    const payloads = try allocator.alloc([]const u8, proofs.len);
    for (input.conflations, proofs, conflations, payloads) |witness, verifiable, *conflation, *payload| {
        conflation.* = try canonical.derive(allocator, witness, verifiable.proof, input.chain_id);
        payload.* = conflation.payload;
    }

    const verified = try verifyDataAvailability(allocator, input, payloads);

    const first = proofs[0].proof;
    const last = proofs[proofs.len - 1].proof;
    const range_start = first.start_block_number;
    const range_end = last.public_inputs.end_block_number;

    try chain.verifyTiling(proofs);
    for (proofs) |verifiable| try recursion.verifyL2ExecutionProof(allocator, verifiable);

    const block_hashes = try concatHashes(allocator, conflations, .block_hashes);
    const parent_hashes = try concatHashes(allocator, conflations, .parent_hashes);
    try chain.verifyEndBlockHashes(proofs, range_start, block_hashes);
    try chain.verifyParentHashChain(first.public_inputs.parent_block_hash, parent_hashes, block_hashes);
    for (proofs[0 .. proofs.len - 1], proofs[1..]) |left, right| {
        try chain.verifyContinuity(left.proof.public_inputs, right.proof.public_inputs);
    }

    var message_count: usize = 0;
    var address_count: usize = 0;
    var offset_count: usize = 0;
    for (proofs) |verifiable| {
        const pi = verifiable.proof.public_inputs;
        message_count += pi.l2_l1_messages.len;
        address_count += pi.filtered_addresses.len;
        offset_count += pi.l2_messaging_blocks_offsets.len;
    }
    const l2_l1_messages = try allocator.alloc([32]u8, message_count);
    const all_addresses = try allocator.alloc([20]u8, address_count);
    const offsets = try allocator.alloc(u16, offset_count);
    const program_vks = try allocator.alloc([32]u8, proofs.len);
    var message_pos: usize = 0;
    var address_pos: usize = 0;
    var offset_pos: usize = 0;
    var end_bridge_rolling_hash: [32]u8 = @splat(0);
    var end_bridge_message_number: u64 = 0;
    for (proofs, program_vks) |verifiable, *program_vk| {
        const pi = verifiable.proof.public_inputs;
        @memcpy(l2_l1_messages[message_pos..][0..pi.l2_l1_messages.len], pi.l2_l1_messages);
        message_pos += pi.l2_l1_messages.len;
        @memcpy(all_addresses[address_pos..][0..pi.filtered_addresses.len], pi.filtered_addresses);
        address_pos += pi.filtered_addresses.len;
        const proof_offsets = offsets[offset_pos..][0..pi.l2_messaging_blocks_offsets.len];
        try messaging_offsets.rebase(verifiable.proof, range_start, proof_offsets);
        offset_pos += proof_offsets.len;
        program_vk.* = verifiable.program_vk;
        if (pi.end_l1_l2_bridge_rolling_hash_message_number != 0) {
            end_bridge_rolling_hash = pi.end_l1_l2_bridge_rolling_hash;
            end_bridge_message_number = pi.end_l1_l2_bridge_rolling_hash_message_number;
        }
    }

    return .{
        .end_block_number = range_end,
        .end_block_timestamp = last.public_inputs.end_block_timestamp,
        .end_l1_l2_bridge_rolling_hash = end_bridge_rolling_hash,
        .end_l1_l2_bridge_rolling_hash_message_number = end_bridge_message_number,
        .dynamic_chain_config_hash = first.public_inputs.dynamic_chain_config_hash,
        .parent_ftx_rolling_hash = first.public_inputs.parent_ftx_rolling_hash,
        .parent_ftx_number = first.public_inputs.parent_ftx_number,
        .end_ftx_rolling_hash = last.public_inputs.end_ftx_rolling_hash,
        .end_processed_ftx_number = last.public_inputs.end_processed_ftx_number,
        .parent_data_rolling_hash = input.parent_data_rolling_hash,
        .end_data_rolling_hash = verified.end_data_rolling_hash,
        .parent_block_hash = first.public_inputs.parent_block_hash,
        .end_block_hash = last.public_inputs.end_block_hash,
        .parent_data_tail_take_bytes = input.parent_data_tail_take_bytes,
        .final_data_tail_discard_bytes = verified.final_data_tail_discard_bytes,
        .l2_l1_messages = l2_l1_messages,
        .filtered_addresses = guest_common.lists.sortedDistinct(20, all_addresses),
        .program_vks = guest_common.lists.sortedDistinct(32, program_vks),
        .block_count = range_end - range_start + 1,
        .l2_messaging_blocks_offsets = offsets,
    };
}

/// The chunks bound to their hashes and the stream they carry parsed and decompressed against the
/// canonical `payloads`.
fn verifyDataAvailability(allocator: Allocator, input: rollup_ssz.RollupProofPrivateInput, payloads: []const []const u8) Error!decompress.Verified {
    const decoder = lineth_da.zstd_decoder.Decoder.init(allocator, lineth_da.dictionary_v2025_04_21) catch |err| switch (err) {
        error.OutOfMemory => return error.OutOfMemory,
        // The embedded dictionary is a well-formed zstd dictionary.
        error.InvalidDictionary => unreachable,
    };
    defer decoder.deinit();
    return decompress.verifyDaChunks(
        allocator,
        decoder,
        input.chunks,
        payloads,
        input.parent_data_rolling_hash,
        input.parent_data_tail_take_bytes,
        input.boundary_prev_data_rolling_hash,
    );
}

/// The hashes `field` names of every conflation, in order.
fn concatHashes(allocator: Allocator, conflations: []const canonical.Conflation, comptime field: enum { block_hashes, parent_hashes }) Allocator.Error![]const [32]u8 {
    var total: usize = 0;
    for (conflations) |conflation| total += @field(conflation, @tagName(field)).len;
    const hashes = try allocator.alloc([32]u8, total);
    var next: usize = 0;
    for (conflations) |conflation| {
        const part = @field(conflation, @tagName(field));
        @memcpy(hashes[next..][0..part.len], part);
        next += part.len;
    }
    return hashes;
}
