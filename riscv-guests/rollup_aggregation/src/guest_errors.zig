//! Deterministic guest exit-code taxonomy for the rollup-aggregation guest.
//!
//! Failures map to coarse, category-stable exit codes, one category per step of the statement:
//! adding or renaming an individual error never renumbers a category, so codes stay meaningful to
//! operators across guest versions. Success exits 0; every category here is nonzero.
//!
//! Standalone by design: Zig error literals are global (matched by name program-wide), so this file
//! needs no imports.

/// Matches the width of the guest's exit primitive.
pub const ExitCode = u32;

pub const CODE_UNKNOWN: ExitCode = 1;
/// The 2-byte schema id is missing or doesn't match `rollup_aggregation_ssz.INPUT_SCHEMA_ID`.
pub const CODE_MALFORMED_FRAME: ExitCode = 2;
/// The schema id matched, but the SSZ body is not the canonical encoding within the bounds of the
/// wire schema.
pub const CODE_MALFORMED_SSZ: ExitCode = 3;
/// The request has no rollup proof.
pub const CODE_MALFORMED_REQUEST: ExitCode = 4;
/// The output value was computed but its SSZ encoder rejected it: a list beyond the bound of the
/// output's wire schema.
pub const CODE_OUTPUT_ENCODE_FAILED: ExitCode = 5;
/// A rollup proof is rejected by the recursive verifier, or its public input is beyond a bound of
/// its wire schema.
pub const CODE_RECURSION: ExitCode = 11;
/// The rollup proofs do not tile the block range, or two consecutive proofs break the continuity of
/// the data rolling hash, the shared-blob tail, the block hash, the chain configuration or the
/// forced-transaction state.
pub const CODE_TILING_CONTINUITY: ExitCode = 13;
/// A proof's `block_count` differs from its block range, or its messaging-block offsets are not
/// strictly increasing within the range or do not fit the rebased 16-bit offset.
pub const CODE_MESSAGING_OFFSETS: ExitCode = 14;
/// An allocator refused a request within the guest's fixed heap.
pub const CODE_OUT_OF_MEMORY: ExitCode = 15;

/// Maps a guest failure to its deterministic, category-stable exit code.
pub fn exitCode(err: anyerror) ExitCode {
    return switch (err) {
        error.MalformedFrame => CODE_MALFORMED_FRAME,
        error.InvalidSsz => CODE_MALFORMED_SSZ,
        error.InvalidRequest => CODE_MALFORMED_REQUEST,
        error.OutputEncodeFailed => CODE_OUTPUT_ENCODE_FAILED,
        error.RecursiveProofRejected => CODE_RECURSION,
        error.TilingBroken,
        error.ContinuityBroken,
        => CODE_TILING_CONTINUITY,
        error.InvalidMessagingOffsets => CODE_MESSAGING_OFFSETS,
        error.OutOfMemory => CODE_OUT_OF_MEMORY,
        else => CODE_UNKNOWN,
    };
}
