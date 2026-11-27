//! Chains of adjacent rollup proofs written as literal public inputs: each proof starts where the
//! one before it ends and continues its hashes, and `prove` gives every proof a real devmode proof
//! for its program VK and public input.

const std = @import("std");
const core_serde = @import("core_serde");
const devmode_prover = @import("devmode_prover");
const rollup_aggregation_ssz = @import("rollup_aggregation").rollup_aggregation_ssz;

const Allocator = std.mem.Allocator;
const rollup_proof = core_serde.rollup_proof;

pub const first_block: u64 = 1_000_501;
pub const default_program_vk: [32]u8 = @splat(0x2a);

/// What one proof of a chain is made of; every other field follows from its position in the chain.
pub const Spec = struct {
    block_count: u64 = 10,
    messages: []const [32]u8 = &.{},
    offsets: []const u16 = &.{},
    filtered_addresses: []const [20]u8 = &.{},
    program_vks: []const [32]u8 = &.{},
    program_vk: [32]u8 = default_program_vk,
    bridge_rolling_hash: [32]u8 = @splat(0),
    bridge_message_number: u64 = 0,
};

/// The value of the boundary `index` of the chain: the end of proof `index - 1` and the start of
/// proof `index`.
pub fn boundary(kind: enum { block_hash, data_rolling_hash, ftx_rolling_hash }, index: u64) [32]u8 {
    const base: u8 = switch (kind) {
        .block_hash => 0x10,
        .data_rolling_hash => 0x30,
        .ftx_rolling_hash => 0x50,
    };
    return @splat(base + @as(u8, @intCast(index)));
}

pub fn tailBytes(index: u64) u64 {
    return 100 * index;
}

pub fn ftxNumber(index: u64) u64 {
    return 10 + 2 * index;
}

pub const chain_config_hash: [32]u8 = @splat(0xc0);

pub fn timestamp(block: u64) u64 {
    return 1_763_000_000 + block;
}

/// The last block of the chain of `specs`.
pub fn endBlock(specs: []const Spec) u64 {
    var end = first_block - 1;
    for (specs) |spec| end += spec.block_count;
    return end;
}

const sample_messages = [_][32]u8{ @splat(0x71), @splat(0x72), @splat(0x73) };
const sample_addresses = [_][20]u8{ @splat(0xa1), @splat(0xa2) };

/// The chain of the committed sample request: two proofs with messages, offsets and addresses.
pub const sample = [_]Spec{
    .{
        .messages = sample_messages[0..2],
        .offsets = &.{ 2, 7 },
        .filtered_addresses = sample_addresses[0..1],
        .bridge_rolling_hash = @splat(0xb1),
        .bridge_message_number = 4,
    },
    .{
        .block_count = 5,
        .messages = sample_messages[2..],
        .offsets = &.{3},
        .filtered_addresses = &sample_addresses,
        .program_vk = @splat(0x2b),
    },
};

/// The proofs of `specs`, in order, with empty proof bytes.
pub fn build(a: Allocator, specs: []const Spec) ![]rollup_proof.VerifiableRollupProof {
    const proofs = try a.alloc(rollup_proof.VerifiableRollupProof, specs.len);
    var start = first_block;
    for (specs, proofs, 0..) |spec, *verifiable, i| {
        const index: u64 = i;
        const end = start + spec.block_count - 1;
        verifiable.* = .{
            .program_vk = spec.program_vk,
            .proof = .{
                .start_block_number = start,
                .proof = .{ .bytes = &.{} },
                .public_inputs = .{
                    .end_block_number = end,
                    .end_block_timestamp = timestamp(end),
                    .end_l1_l2_bridge_rolling_hash = spec.bridge_rolling_hash,
                    .end_l1_l2_bridge_rolling_hash_message_number = spec.bridge_message_number,
                    .dynamic_chain_config_hash = chain_config_hash,
                    .parent_ftx_rolling_hash = boundary(.ftx_rolling_hash, index),
                    .parent_ftx_number = ftxNumber(index),
                    .end_ftx_rolling_hash = boundary(.ftx_rolling_hash, index + 1),
                    .end_processed_ftx_number = ftxNumber(index + 1),
                    .parent_data_rolling_hash = boundary(.data_rolling_hash, index),
                    .end_data_rolling_hash = boundary(.data_rolling_hash, index + 1),
                    .parent_block_hash = boundary(.block_hash, index),
                    .end_block_hash = boundary(.block_hash, index + 1),
                    .parent_shared_blob_carry_over_tail_bytes = tailBytes(index),
                    .final_shared_blob_carry_over_tail_bytes = tailBytes(index + 1),
                    .l2_l1_messages = .{ .items = spec.messages },
                    .filtered_addresses = .{ .items = spec.filtered_addresses },
                    .program_vks = .{ .items = spec.program_vks },
                    .block_count = spec.block_count,
                    .l2_messaging_blocks_offsets = .{ .items = spec.offsets },
                },
            },
        };
        start = end + 1;
    }
    return proofs;
}

/// Replaces the proof of every entry of `proofs` with one that is valid for its program VK and its
/// public input as they are now, which an edit of either leaves unproved.
pub fn prove(a: Allocator, proofs: []rollup_proof.VerifiableRollupProof) !void {
    for (proofs) |*verifiable| {
        const hash = try rollup_proof.hashPublicInputs(a, verifiable.proof.public_inputs);
        verifiable.proof.proof = .{ .bytes = try devmode_prover.prove(a, verifiable.program_vk, hash) };
    }
}

pub fn input(proofs: []const rollup_proof.VerifiableRollupProof) rollup_aggregation_ssz.RollupAggregationInput {
    return .{ .rollup_proofs = .{ .items = proofs } };
}

/// The proved proofs of `specs` as a request.
pub fn request(a: Allocator, specs: []const Spec) !rollup_aggregation_ssz.RollupAggregationInput {
    const proofs = try build(a, specs);
    try prove(a, proofs);
    return input(proofs);
}
