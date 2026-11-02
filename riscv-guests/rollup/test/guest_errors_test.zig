//! The exit codes of the guest: one stable nonzero category per step of the statement.

const std = @import("std");

const guest_errors = @import("rollup").guest_errors;

fn expectCode(code: u64, err: anyerror) !void {
    try std.testing.expectEqual(code, guest_errors.exitCode(err));
}

test "every rejection of the statement exits with the category of its step" {
    try expectCode(2, error.MalformedFrame);
    try expectCode(3, error.InvalidSsz);
    try expectCode(4, error.BoundsViolation);
    try expectCode(5, error.InvalidRequest);
    try expectCode(6, error.OutputEncodeFailed);

    // Blocks and senders.
    for ([_]anyerror{ error.MalformedBlock, error.SenderRecoveryFailed, error.BlockCountMismatch, error.SendersHashMismatch }) |err| try expectCode(7, err);
    // Chunk binding.
    for ([_]anyerror{ error.NoChunks, error.MalformedChunk, error.NonCanonicalBlob, error.ChunkHashMismatch, error.BoundaryPreimageMismatch }) |err| try expectCode(8, err);
    try expectCode(9, error.BlobKzgMismatch);
    // Stream layout and segments.
    for ([_]anyerror{
        error.PartialBlobBeforeBlob,
        error.InvalidTailTake,
        error.MissingSegmentLength,
        error.InvalidSegmentLength,
        error.ChunkOwnsNoBytes,
        error.CalldataEndsMidSegment,
    }) |err| try expectCode(10, err);
    // Decompression.
    for ([_]anyerror{ error.InvalidFrame, error.PayloadMismatch }) |err| try expectCode(11, err);
    try expectCode(12, error.RecursiveProofRejected);
    for ([_]anyerror{ error.EndBlockHashMismatch, error.ParentHashMismatch }) |err| try expectCode(13, err);
    for ([_]anyerror{ error.TilingBroken, error.ContinuityBroken }) |err| try expectCode(14, err);
    try expectCode(15, error.InvalidMessagingOffsets);
    try expectCode(16, error.OutOfMemory);
}

test "an error the guest does not name exits with the unknown category" {
    try expectCode(1, error.InvalidDictionary);
}
