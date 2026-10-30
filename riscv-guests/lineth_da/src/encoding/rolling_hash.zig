//! The dataRollingHash: the keccak256 accumulator over the ordered chunk hashes published on L1,
//! `dataRollingHash_i = keccak256(dataRollingHash_{i-1} ‖ chunkHash_i)`.

const std = @import("std");

const accelerators = @import("zesu_accelerators");

pub const Hash = [32]u8;

pub const Error = error{
    /// `boundaryPrevDataRollingHash` is absent or does not open the parent hash.
    BoundaryPreimageMismatch,
};

/// One fold step.
pub fn next(parent: Hash, chunk_hash: Hash) Hash {
    var preimage: [64]u8 = undefined;
    @memcpy(preimage[0..32], &parent);
    @memcpy(preimage[32..64], &chunk_hash);
    var out: Hash = undefined;
    accelerators.keccak256(&preimage, &out);
    return out;
}

/// The hash after folding `chunk_hashes` onto `parent`. A positive `parent_data_tail_take_bytes`
/// starts the range inside the first chunk, which `parent` already includes: `boundary_prev` must
/// then open `parent` as the hash before that chunk, and the first chunk is not folded again.
pub fn fold(
    parent: Hash,
    chunk_hashes: []const Hash,
    parent_data_tail_take_bytes: usize,
    boundary_prev: ?Hash,
) Error!Hash {
    var unfolded = chunk_hashes;
    if (parent_data_tail_take_bytes > 0) {
        const previous = boundary_prev orelse return error.BoundaryPreimageMismatch;
        if (chunk_hashes.len == 0 or !std.mem.eql(u8, &next(previous, chunk_hashes[0]), &parent)) {
            return error.BoundaryPreimageMismatch;
        }
        unfolded = chunk_hashes[1..];
    }
    var hash = parent;
    for (unfolded) |chunk_hash| hash = next(hash, chunk_hash);
    return hash;
}
