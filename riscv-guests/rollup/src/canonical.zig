//! The canonical payload of a conflation: its full block RLPs truncated and encoded into the bytes
//! its zstd frame decompresses to, cross-checked against the conflation's l2-execution proof.

const std = @import("std");
const lineth_da = @import("lineth_da");
const core_serde = @import("core_serde");
const chain = @import("chain.zig");
const rollup_ssz = @import("rollup_ssz.zig");

const Allocator = std.mem.Allocator;
const l2_execution_proof = core_serde.l2_execution_proof;
const truncation = lineth_da.encoding.truncation;

pub const Hash = truncation.Hash;
pub const Address = truncation.Address;

pub const Error = truncation.Error || error{
    /// The conflation has no block, or its block count differs from its proof's range.
    BlockCountMismatch,
    /// keccak256 of the senders of the conflation's transactions differs from the proof's
    /// `tx_froms_hash`.
    SendersHashMismatch,
};

pub const Conflation = struct {
    /// The bytes the conflation's zstd frame decompresses to.
    payload: []const u8,
    /// The hash of each block's header, in block order.
    block_hashes: []const Hash,
    /// The parent hash of each block's header, in block order.
    parent_hashes: []const Hash,
};

/// The canonical payload of `conflation` for the chain `chain_id`, and its blocks' hashes and
/// parent hashes. The blocks are those of `proof`'s range and their recovered senders hash to the
/// proof's `tx_froms_hash`. Everything is allocated from `allocator`, a bump allocator whose memory
/// is reclaimed as a whole.
pub fn derive(
    allocator: Allocator,
    conflation: rollup_ssz.ConflationWitness,
    proof: l2_execution_proof.L2ExecutionProof,
    chain_id: u64,
) Error!Conflation {
    const count = conflation.block_rlps.len;
    if (count == 0 or chain.blockCount(proof) != count) return error.BlockCountMismatch;

    const truncated = try allocator.alloc(truncation.TruncatedBlock, count);
    const block_hashes = try allocator.alloc(Hash, count);
    const parent_hashes = try allocator.alloc(Hash, count);
    var sender_count: usize = 0;
    for (conflation.block_rlps, truncated, block_hashes, parent_hashes) |block_rlp, *block, *block_hash, *parent_hash| {
        block.* = try truncation.truncateBlockRlp(allocator, allocator, block_rlp, chain_id);
        block_hash.* = block.block_hash;
        parent_hash.* = block.parent_hash;
        sender_count += block.froms.len;
    }

    const senders = try allocator.alloc(Address, sender_count);
    var next: usize = 0;
    for (truncated) |block| {
        @memcpy(senders[next..][0..block.froms.len], block.froms);
        next += block.froms.len;
    }
    var senders_hash: Hash = undefined;
    lineth_da.accelerators.keccak256(std.mem.sliceAsBytes(senders), &senders_hash);
    if (!std.mem.eql(u8, &senders_hash, &proof.public_inputs.tx_froms_hash)) return error.SendersHashMismatch;

    const payload = try truncation.encodeBlocks(allocator, allocator, truncated);
    return .{ .payload = payload, .block_hashes = block_hashes, .parent_hashes = parent_hashes };
}
