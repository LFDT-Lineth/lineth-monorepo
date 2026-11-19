//! The messaging-block offsets of an l2-execution proof, rebased into the block range of the
//! rollup proof that contains it. An offset counts blocks from the proof's own first block (the
//! first block is offset 1); the rebased offset counts them from the first block of the rollup
//! proof.

const std = @import("std");
const core_serde = @import("core_serde");
const chain = @import("chain.zig");

const l2_execution_proof = core_serde.l2_execution_proof;

pub const Error = error{
    /// The proof's `block_count` differs from its block range, an offset does not lie strictly
    /// above the one before it within the range, or a rebased offset exceeds 16 bits.
    InvalidMessagingOffsets,
};

/// Writes the proof's offsets, rebased onto `range_start`, the first block of the rollup proof, to
/// `out`, which holds one element per offset.
pub fn rebase(proof: l2_execution_proof.L2ExecutionProof, range_start: u64, out: []u16) Error!void {
    const pi = proof.public_inputs;
    std.debug.assert(out.len == pi.l2_messaging_blocks_offsets.items.len);
    const count = chain.blockCount(proof) orelse return error.InvalidMessagingOffsets;
    if (pi.block_count != count) return error.InvalidMessagingOffsets;
    const shift = std.math.sub(u64, proof.start_block_number, range_start) catch return error.InvalidMessagingOffsets;

    var previous: u16 = 0;
    for (pi.l2_messaging_blocks_offsets.items, out) |offset, *rebased| {
        if (offset <= previous or offset > count) return error.InvalidMessagingOffsets;
        rebased.* = std.math.cast(u16, shift + offset) orelse return error.InvalidMessagingOffsets;
        previous = offset;
    }
}
