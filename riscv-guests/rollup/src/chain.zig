//! The l2-execution proofs of a rollup proof as a chain of block ranges: the range arithmetic, the
//! tiling of the ranges, the binding of the proofs to the blocks of the conflations, and the
//! continuity between consecutive proofs.

const std = @import("std");
const core_serde = @import("core_serde");

const l2_execution_proof = core_serde.l2_execution_proof;

pub const Hash = [32]u8;

pub const Error = error{
    /// A proof starts at another block than the one after its predecessor's end.
    TilingBroken,
    /// A proof's end block hash differs from the conflation's block at its boundary, or lies outside
    /// the blocks of the conflations.
    EndBlockHashMismatch,
    /// A block's parent hash differs from the hash of the block before it, or the first block's from
    /// the first proof's parent block hash.
    ParentHashMismatch,
    /// Two consecutive proofs disagree on the block hash, the chain configuration or the
    /// forced-transaction state at their junction.
    ContinuityBroken,
};

/// The number of blocks in the proof's range, or null when the range is empty or does not fit 64
/// bits.
pub fn blockCount(proof: l2_execution_proof.L2ExecutionProof) ?u64 {
    const end = proof.public_inputs.end_block_number;
    if (end < proof.start_block_number) return null;
    return std.math.add(u64, end - proof.start_block_number, 1) catch null;
}

/// Every proof starts right after the end of the proof before it.
pub fn verifyTiling(proofs: []const l2_execution_proof.VerifiableL2ExecutionProof) Error!void {
    if (proofs.len == 0) return;
    var expected_start: u128 = proofs[0].proof.start_block_number;
    for (proofs) |verifiable| {
        if (verifiable.proof.start_block_number != expected_start) return error.TilingBroken;
        expected_start = @as(u128, verifiable.proof.public_inputs.end_block_number) + 1;
    }
}

/// Every proof's end block hash is the hash of the block at the proof's end block number. The
/// `block_hashes` are those of every conflation's blocks in order, the first of which is block
/// number `range_start`.
pub fn verifyEndBlockHashes(
    proofs: []const l2_execution_proof.VerifiableL2ExecutionProof,
    range_start: u64,
    block_hashes: []const Hash,
) Error!void {
    for (proofs) |verifiable| {
        const pi = verifiable.proof.public_inputs;
        const index = std.math.sub(u64, pi.end_block_number, range_start) catch return error.EndBlockHashMismatch;
        if (index >= block_hashes.len or !std.mem.eql(u8, &pi.end_block_hash, &block_hashes[@intCast(index)])) {
            return error.EndBlockHashMismatch;
        }
    }
}

/// The first block descends from `parent_block_hash` and every other block from the one before it.
pub fn verifyParentHashChain(parent_block_hash: Hash, parent_hashes: []const Hash, block_hashes: []const Hash) Error!void {
    std.debug.assert(parent_hashes.len == block_hashes.len);
    if (parent_hashes.len == 0) return;
    if (!std.mem.eql(u8, &parent_hashes[0], &parent_block_hash)) return error.ParentHashMismatch;
    for (parent_hashes[1..], block_hashes[0 .. block_hashes.len - 1]) |parent_hash, previous_hash| {
        if (!std.mem.eql(u8, &parent_hash, &previous_hash)) return error.ParentHashMismatch;
    }
}

/// `right` continues `left`: its parent block hash, chain configuration and forced-transaction
/// state are the ones `left` ends with.
pub fn verifyContinuity(left: l2_execution_proof.L2ExecutionProofPublicInput, right: l2_execution_proof.L2ExecutionProofPublicInput) Error!void {
    if (!std.mem.eql(u8, &left.end_block_hash, &right.parent_block_hash) or
        !std.mem.eql(u8, &left.dynamic_chain_config_hash, &right.dynamic_chain_config_hash) or
        !std.mem.eql(u8, &left.end_ftx_rolling_hash, &right.parent_ftx_rolling_hash) or
        left.end_processed_ftx_number != right.parent_ftx_number) return error.ContinuityBroken;
}
