//! The messaging-block offsets of a proof, rebased into the block range of the proof that
//! aggregates it. An offset counts blocks from the proof's own first block (the first block is
//! offset 1); the rebased offset counts them from the first block of the range.

const std = @import("std");

pub const Error = error{
    /// The proof's `block_count` differs from its block range, an offset does not lie strictly
    /// above the one before it within the range, or a rebased offset exceeds 16 bits.
    InvalidMessagingOffsets,
};

/// What of a proof the rebasing reads: its block range, the block count it claims and its offsets.
pub const Proof = struct {
    start_block_number: u64,
    end_block_number: u64,
    block_count: u64,
    offsets: []align(1) const u16,
};

/// The number of blocks in the range, or null when the range is empty or does not fit 64 bits.
pub fn blockCount(start_block_number: u64, end_block_number: u64) ?u64 {
    if (end_block_number < start_block_number) return null;
    return std.math.add(u64, end_block_number - start_block_number, 1) catch null;
}

/// Writes the proof's offsets, rebased onto `range_start`, the first block of the range, to
/// `out`, which holds one element per offset.
pub fn rebase(proof: Proof, range_start: u64, out: []u16) Error!void {
    std.debug.assert(out.len == proof.offsets.len);
    const count = blockCount(proof.start_block_number, proof.end_block_number) orelse return error.InvalidMessagingOffsets;
    if (proof.block_count != count) return error.InvalidMessagingOffsets;
    const shift = std.math.sub(u64, proof.start_block_number, range_start) catch return error.InvalidMessagingOffsets;

    var previous: u16 = 0;
    for (proof.offsets, out) |offset, *rebased| {
        if (offset <= previous or offset > count) return error.InvalidMessagingOffsets;
        rebased.* = std.math.cast(u16, shift + offset) orelse return error.InvalidMessagingOffsets;
        previous = offset;
    }
}
