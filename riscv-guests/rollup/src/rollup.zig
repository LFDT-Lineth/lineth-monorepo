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
const lineth_recursion = @import("lineth_recursion");
const canonical = @import("canonical.zig");
const chain = @import("chain.zig");
const rollup_ssz = @import("rollup_ssz.zig");

const Allocator = std.mem.Allocator;
const decompress = lineth_da.decompress;
const messaging_offsets = guest_common.messaging_offsets;
const recursion = lineth_recursion.recursion;
const l2_execution_proof = core_serde.l2_execution_proof;
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
    const proofs = input.l2_execution_proofs.items;
    const witnesses = input.conflations.items;
    if (witnesses.len == 0 or proofs.len == 0 or witnesses.len != proofs.len) return error.InvalidRequest;

    const conflations = try allocator.alloc(canonical.Conflation, proofs.len);
    const payloads = try allocator.alloc([]const u8, proofs.len);
    for (witnesses, proofs, conflations, payloads) |witness, verifiable, *conflation, *payload| {
        conflation.* = try canonical.derive(allocator, witness, verifiable.proof, input.chain_id);
        payload.* = conflation.payload;
    }

    const verified = try verifyDataAvailability(allocator, input, payloads);

    const first = proofs[0].proof;
    const last = proofs[proofs.len - 1].proof;
    const range_start = first.start_block_number;
    const range_end = last.public_inputs.end_block_number;

    try chain.verifyTiling(proofs);
    const recursive_verifier = try recursion.Verifier.init(allocator, proofs);
    for (proofs) |verifiable| try verifyL2ExecutionProof(allocator, recursive_verifier, verifiable);

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
        message_count += pi.l2_l1_messages.items.len;
        address_count += pi.filtered_addresses.items.len;
        offset_count += pi.l2_messaging_blocks_offsets.items.len;
    }
    const l2_l1_messages = try allocator.alloc([32]u8, message_count);
    const all_addresses = try allocator.alloc([20]u8, address_count);
    const offsets = try allocator.alloc(u16, offset_count);
    var message_pos: usize = 0;
    var address_pos: usize = 0;
    var offset_pos: usize = 0;
    var end_bridge: guest_common.l1_l2_bridge.Pair = .none;
    for (proofs) |verifiable| {
        const pi = verifiable.proof.public_inputs;
        @memcpy(l2_l1_messages[message_pos..][0..pi.l2_l1_messages.items.len], pi.l2_l1_messages.items);
        message_pos += pi.l2_l1_messages.items.len;
        @memcpy(all_addresses[address_pos..][0..pi.filtered_addresses.items.len], pi.filtered_addresses.items);
        address_pos += pi.filtered_addresses.items.len;
        const proof_offsets = offsets[offset_pos..][0..pi.l2_messaging_blocks_offsets.items.len];
        try messaging_offsets.rebase(.{
            .start_block_number = verifiable.proof.start_block_number,
            .end_block_number = pi.end_block_number,
            .block_count = pi.block_count,
            .offsets = pi.l2_messaging_blocks_offsets.items,
        }, range_start, proof_offsets);
        offset_pos += proof_offsets.len;
        end_bridge = guest_common.l1_l2_bridge.advance(end_bridge, pi);
    }

    return .{
        .end_block_number = range_end,
        .end_block_timestamp = last.public_inputs.end_block_timestamp,
        .end_l1_l2_bridge_rolling_hash = end_bridge.rolling_hash,
        .end_l1_l2_bridge_rolling_hash_message_number = end_bridge.message_number,
        .dynamic_chain_config_hash = first.public_inputs.dynamic_chain_config_hash,
        .parent_ftx_rolling_hash = first.public_inputs.parent_ftx_rolling_hash,
        .parent_ftx_number = first.public_inputs.parent_ftx_number,
        .end_ftx_rolling_hash = last.public_inputs.end_ftx_rolling_hash,
        .end_processed_ftx_number = last.public_inputs.end_processed_ftx_number,
        .parent_data_rolling_hash = input.parent_data_rolling_hash,
        .end_data_rolling_hash = verified.end_data_rolling_hash,
        .parent_block_hash = first.public_inputs.parent_block_hash,
        .end_block_hash = last.public_inputs.end_block_hash,
        .parent_shared_blob_carry_over_tail_bytes = input.parent_shared_blob_carry_over_tail_bytes,
        .final_shared_blob_carry_over_tail_bytes = verified.final_shared_blob_carry_over_tail_bytes,
        .l2_l1_messages = .{ .items = l2_l1_messages },
        .filtered_addresses = .{ .items = guest_common.lists.sortedDistinct(20, all_addresses) },
        .program_vks = .{ .items = try guest_common.program_vks.collect(allocator, proofs) },
        .block_count = range_end - range_start + 1,
        .l2_messaging_blocks_offsets = .{ .items = offsets },
    };
}

/// Verifies an l2-execution proof under the program VK it carries, against the hash of its public
/// input, which is rejected when beyond a bound of its wire schema. `allocator` holds the encoding
/// that is hashed.
fn verifyL2ExecutionProof(allocator: Allocator, recursive_verifier: recursion.Verifier, verifiable: l2_execution_proof.VerifiableL2ExecutionProof) Error!void {
    const public_input_hash = l2_execution_proof.hashPublicInputs(allocator, verifiable.proof.public_inputs) catch |err| switch (err) {
        error.OutOfMemory => return error.OutOfMemory,
        error.BoundsViolation => return error.RecursiveProofRejected,
    };
    try recursive_verifier.verify(verifiable.program_vk, verifiable.proof.proof.bytes, public_input_hash);
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
    const parent_blob = input.parent_blob_data_rolling_hash.items;
    return decompress.verifyDaChunks(
        allocator,
        decoder,
        input.chunks.items,
        payloads,
        input.parent_data_rolling_hash,
        input.parent_shared_blob_carry_over_tail_bytes,
        if (parent_blob.len == 1) parent_blob[0] else null,
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
