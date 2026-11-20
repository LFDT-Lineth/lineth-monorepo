//! Blob commitments and proofs by Constantine's KZG prover, over the one embedded trusted-setup
//! context of the process, which the crypto providers build on first use.

const std = @import("std");

/// Constantine's own declarations, translated from its headers.
const c = @import("constantine_c");

pub const blob_len = 4096 * 32;
pub const point_len = 48;

comptime {
    // The statuses are packed enums of one byte, which is how the archive returns them.
    std.debug.assert(@sizeOf(c.ctt_eth_kzg_status) == 1);
    std.debug.assert(@sizeOf(c.ctt_eth_trusted_setup_status) == 1);
    std.debug.assert(returnType(c.ctt_eth_kzg_blob_to_kzg_commitment) == c.ctt_eth_kzg_status);
    std.debug.assert(returnType(c.ctt_eth_kzg_compute_blob_kzg_proof) == c.ctt_eth_kzg_status);
    std.debug.assert(returnType(c.ctt_eth_kzg_context_new_embedded) == c.ctt_eth_trusted_setup_status);
    // The byte arrays of the callers have the layouts of Constantine's structs.
    std.debug.assert(@sizeOf(c.ctt_eth_kzg_blob) == blob_len);
    std.debug.assert(@sizeOf(c.ctt_eth_kzg_commitment) == point_len);
    std.debug.assert(@sizeOf(c.ctt_eth_kzg_proof) == point_len);
}

fn returnType(comptime function: anytype) type {
    return @typeInfo(@TypeOf(function)).@"fn".return_type.?;
}

pub const Error = error{
    /// Constantine could not build its trusted-setup context.
    ContextUnavailable,
    /// Constantine rejected the blob or the commitment.
    KzgFailed,
};

/// The EIP-4844 commitment of a blob and the blob proof of that commitment.
pub const Opening = struct {
    commitment: [point_len]u8,
    proof: [point_len]u8,
};

/// The embedded KZG context the host providers export.
extern fn lineth_zkvm_kzg_context() ?*const anyopaque;

fn sharedContext() Error!*const c.ctt_eth_kzg_context {
    const ctx = lineth_zkvm_kzg_context() orelse return error.ContextUnavailable;
    return @ptrCast(ctx);
}

/// The commitment of `blob`.
pub fn commit(blob: *const [blob_len]u8) Error![point_len]u8 {
    const ctx = try sharedContext();
    var commitment: c.ctt_eth_kzg_commitment = undefined;
    if (c.ctt_eth_kzg_blob_to_kzg_commitment(ctx, &commitment, @ptrCast(blob)) != c.cttEthKzg_Success) return error.KzgFailed;
    return commitment.raw;
}

/// The blob proof of `blob` against `commitment`, which has to be the commitment of `blob`.
pub fn prove(blob: *const [blob_len]u8, commitment: *const [point_len]u8) Error![point_len]u8 {
    const ctx = try sharedContext();
    var proof: c.ctt_eth_kzg_proof = undefined;
    if (c.ctt_eth_kzg_compute_blob_kzg_proof(ctx, &proof, @ptrCast(blob), @ptrCast(commitment)) != c.cttEthKzg_Success) return error.KzgFailed;
    return proof.raw;
}

/// The commitment of `blob` and its blob proof.
pub fn open(blob: *const [blob_len]u8) Error!Opening {
    const commitment = try commit(blob);
    return .{ .commitment = commitment, .proof = try prove(blob, &commitment) };
}
