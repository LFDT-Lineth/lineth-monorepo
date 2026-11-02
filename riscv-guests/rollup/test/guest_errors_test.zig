//! The exit codes of the guest: one stable nonzero category per step of the statement.

const std = @import("std");

const rollup = @import("rollup");

const guest_errors = rollup.guest_errors;

/// The errors of a function that returns an error union.
fn ErrorsOf(comptime function: anytype) type {
    return @typeInfo(@typeInfo(@TypeOf(function)).@"fn".return_type.?).error_union.error_set;
}

/// Every error the guest's pipeline can end in: decoding the input, the statement and encoding the
/// output.
const guest_error_sets = [_]type{ErrorsOf(rollup.rollup.runFramed)};

fn expectCode(code: u64, err: anyerror) !void {
    try std.testing.expectEqual(code, guest_errors.exitCode(err));
}

test "every rejection of the statement exits with the category of its step" {
    try expectCode(2, error.MalformedFrame);
    try expectCode(3, error.InvalidSsz);
    try expectCode(4, error.BoundsViolation);
    for ([_]anyerror{ error.InvalidRequest, error.NoChunks }) |err| try expectCode(5, err);
    try expectCode(6, error.OutputEncodeFailed);

    // Blocks and senders.
    for ([_]anyerror{ error.MalformedBlock, error.SenderRecoveryFailed, error.BlockCountMismatch, error.SendersHashMismatch }) |err| try expectCode(7, err);
    // Chunk binding.
    for ([_]anyerror{ error.MalformedChunk, error.NonCanonicalBlob, error.ChunkHashMismatch, error.BoundaryPreimageMismatch }) |err| try expectCode(8, err);
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

test "every error the guest returns exits with a named category" {
    comptime {
        for (guest_error_sets) |Set| {
            for (@typeInfo(Set).error_set.?) |e| {
                if (guest_errors.exitCode(@field(Set, e.name)) == guest_errors.CODE_UNKNOWN) {
                    @compileError("the guest returns '" ++ e.name ++ "', which exits with the unknown category: add an arm to guest_errors.exitCode");
                }
            }
        }
    }
}

test "an error the guest does not name exits with the unknown category" {
    try expectCode(1, error.NotAnErrorOfTheGuest);
}
