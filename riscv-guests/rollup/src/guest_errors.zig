//! Deterministic guest exit-code taxonomy for the rollup guest.
//!
//! Failures map to coarse, category-stable exit codes, one category per step of the statement:
//! adding or renaming an individual error never renumbers a category, so codes stay meaningful to
//! operators across guest versions. Success exits 0; every category here is nonzero.
//!
//! Standalone by design: Zig error literals are global (matched by name program-wide), so this file
//! needs no imports.

/// Matches the width of the guest's exit primitive.
pub const ExitCode = u64;

pub const CODE_UNKNOWN: ExitCode = 1;
/// The 2-byte schema id is missing or doesn't match `rollup_ssz.INPUT_SCHEMA_ID` — the envelope
/// itself is not recognizable, before any SSZ container is even attempted.
pub const CODE_MALFORMED_FRAME: ExitCode = 2;
/// The schema id matched, but the SSZ body's fixed-head layout, offsets, or nesting are not the
/// canonical encoding (short buffer, misaligned/out-of-order/out-of-bounds offset, truncated
/// variable region).
pub const CODE_MALFORMED_SSZ: ExitCode = 3;
/// A list or byte field decoded to more elements/bytes than its wire-format bound (the `MAX_*`
/// constants) allows.
pub const CODE_BOUNDS_VIOLATION: ExitCode = 4;
/// The request has no conflation or no l2-execution proof, or its conflations and proofs are not
/// paired one to one.
pub const CODE_MALFORMED_REQUEST: ExitCode = 5;
/// The output value was computed but its SSZ encoding failed (allocator exhaustion within the
/// guest's fixed heap).
pub const CODE_OUTPUT_ENCODE_FAILED: ExitCode = 6;
/// Steps 1 and 3: a full block RLP is not an accepted block or its sender cannot be recovered, a
/// conflation's block count differs from its proof's range, or the senders do not hash to the
/// proof's `txFromsHash`.
pub const CODE_BLOCK_BINDING: ExitCode = 7;
/// Step 2: a chunk contradicts its kind, is not the canonical packing of its payload, does not hash
/// to its `chunkHash`, or the boundary preimage does not open the parent dataRollingHash.
pub const CODE_CHUNK_BINDING: ExitCode = 8;
/// Step 2: a blob's EIP-4844 proof does not open its commitment to the blob.
pub const CODE_BLOB_KZG: ExitCode = 9;
/// Step 2: the chunks break a layout rule of the DA stream, or a segment's length prefix is missing,
/// zero or reaches past the stream.
pub const CODE_STREAM_LAYOUT: ExitCode = 10;
/// Step 2: a segment's bytes are not one complete zstd frame of the dictionary, or the frame
/// decompresses to other bytes than the conflation's canonical payload.
pub const CODE_DECOMPRESSION: ExitCode = 11;
/// Step 4: an l2-execution proof is rejected by the recursive verifier.
pub const CODE_RECURSION: ExitCode = 12;
/// Step 5: an l2-execution proof's end block hash differs from the conflation's block at its
/// boundary, or the blocks do not chain by parent hash from the first proof's parent block hash.
pub const CODE_BLOCK_HASH_CHAIN: ExitCode = 13;
/// The l2-execution proofs do not tile the block range, or (step 7) two consecutive proofs break
/// the continuity of block hash, chain configuration or forced-transaction state.
pub const CODE_TILING_CONTINUITY: ExitCode = 14;
/// A proof's block count differs from its range, or its messaging-block offsets are not strictly
/// increasing within the range or do not fit the rebased 16-bit offset.
pub const CODE_MESSAGING_OFFSETS: ExitCode = 15;
/// An allocator refused a request within the guest's fixed heap.
pub const CODE_OUT_OF_MEMORY: ExitCode = 16;

/// Every error a rollup guest function deliberately returns, in `exitCode`'s arm order.
pub const rollup_errors = error{
    MalformedFrame,
    InvalidSsz,
    BoundsViolation,
    InvalidRequest,
    OutputEncodeFailed,
    MalformedBlock,
    SenderRecoveryFailed,
    BlockCountMismatch,
    SendersHashMismatch,
    NoChunks,
    MalformedChunk,
    NonCanonicalBlob,
    ChunkHashMismatch,
    BoundaryPreimageMismatch,
    BlobKzgMismatch,
    PartialBlobBeforeBlob,
    InvalidTailTake,
    MissingSegmentLength,
    InvalidSegmentLength,
    ChunkOwnsNoBytes,
    CalldataEndsMidSegment,
    InvalidFrame,
    PayloadMismatch,
    RecursiveProofRejected,
    EndBlockHashMismatch,
    ParentHashMismatch,
    TilingBroken,
    ContinuityBroken,
    InvalidMessagingOffsets,
    OutOfMemory,
};

/// Maps a guest failure to its deterministic, category-stable exit code.
pub fn exitCode(err: anyerror) ExitCode {
    return switch (err) {
        error.MalformedFrame => CODE_MALFORMED_FRAME,
        error.InvalidSsz => CODE_MALFORMED_SSZ,
        error.BoundsViolation => CODE_BOUNDS_VIOLATION,
        error.InvalidRequest => CODE_MALFORMED_REQUEST,
        error.OutputEncodeFailed => CODE_OUTPUT_ENCODE_FAILED,
        error.MalformedBlock,
        error.SenderRecoveryFailed,
        error.BlockCountMismatch,
        error.SendersHashMismatch,
        => CODE_BLOCK_BINDING,
        error.NoChunks,
        error.MalformedChunk,
        error.NonCanonicalBlob,
        error.ChunkHashMismatch,
        error.BoundaryPreimageMismatch,
        => CODE_CHUNK_BINDING,
        error.BlobKzgMismatch => CODE_BLOB_KZG,
        error.PartialBlobBeforeBlob,
        error.InvalidTailTake,
        error.MissingSegmentLength,
        error.InvalidSegmentLength,
        error.ChunkOwnsNoBytes,
        error.CalldataEndsMidSegment,
        => CODE_STREAM_LAYOUT,
        error.InvalidFrame,
        error.PayloadMismatch,
        => CODE_DECOMPRESSION,
        error.RecursiveProofRejected => CODE_RECURSION,
        error.EndBlockHashMismatch,
        error.ParentHashMismatch,
        => CODE_BLOCK_HASH_CHAIN,
        error.TilingBroken,
        error.ContinuityBroken,
        => CODE_TILING_CONTINUITY,
        error.InvalidMessagingOffsets => CODE_MESSAGING_OFFSETS,
        error.OutOfMemory => CODE_OUT_OF_MEMORY,
        else => CODE_UNKNOWN,
    };
}
