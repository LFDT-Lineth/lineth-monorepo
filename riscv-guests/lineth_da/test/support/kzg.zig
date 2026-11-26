//! Blob commitments and proofs computed by Constantine's KZG prover under the trusted setup that
//! the host archive embeds: the real KZG data of the DA tests and samples.

const std = @import("std");

const kzg_prover = @import("kzg_prover");

pub const blob_len = kzg_prover.blob_len;

pub const Opening = kzg_prover.Opening;

/// Openings by the sha256 of their blob, kept for the life of the process. The tests and tools that
/// use it are single-threaded.
var openings: std.AutoHashMapUnmanaged([32]u8, Opening) = .empty;

/// The commitment of `blob` and its blob proof.
pub fn open(blob: *const [blob_len]u8) Opening {
    var digest: [32]u8 = undefined;
    std.crypto.hash.sha2.Sha256.hash(blob, &digest, .{});
    if (openings.get(digest)) |known| return known;

    const opening = kzg_prover.open(blob) catch |err| std.debug.panic("Constantine failed: {s}", .{@errorName(err)});
    openings.put(std.heap.page_allocator, digest, opening) catch @panic("out of memory");
    return opening;
}
