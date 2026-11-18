//! Blob commitments and proofs computed by Constantine's KZG prover under the trusted setup that
//! the host archive embeds: the real KZG data of the DA tests and samples.

const std = @import("std");

pub const blob_len = 4096 * 32;

/// The EIP-4844 commitment of a blob and the blob proof of that commitment.
pub const Opening = struct {
    commitment: [48]u8,
    proof: [48]u8,
};

const Context = opaque {};

extern fn ctt_eth_kzg_context_new_embedded(ctx: *?*Context) u8;
extern fn ctt_eth_kzg_blob_to_kzg_commitment(ctx: *const Context, dst: *[48]u8, blob: *const [blob_len]u8) u8;
extern fn ctt_eth_kzg_compute_blob_kzg_proof(ctx: *const Context, proof: *[48]u8, blob: *const [blob_len]u8, commitment: *const [48]u8) u8;

/// Created on first use. The tests and tools that use it are single-threaded.
var context: ?*Context = null;
/// Openings by the sha256 of their blob, kept for the life of the process.
var openings: std.AutoHashMapUnmanaged([32]u8, Opening) = .empty;

fn embeddedContext() *const Context {
    if (context == null) check(ctt_eth_kzg_context_new_embedded(&context));
    return context.?;
}

fn check(status: u8) void {
    if (status != 0) std.debug.panic("Constantine failed with status {d}", .{status});
}

/// The commitment of `blob` and its blob proof.
pub fn open(blob: *const [blob_len]u8) Opening {
    var digest: [32]u8 = undefined;
    std.crypto.hash.sha2.Sha256.hash(blob, &digest, .{});
    if (openings.get(digest)) |known| return known;

    const ctx = embeddedContext();
    var opening: Opening = undefined;
    check(ctt_eth_kzg_blob_to_kzg_commitment(ctx, &opening.commitment, blob));
    check(ctt_eth_kzg_compute_blob_kzg_proof(ctx, &opening.proof, blob, &opening.commitment));
    openings.put(std.heap.page_allocator, digest, opening) catch @panic("out of memory");
    return opening;
}
