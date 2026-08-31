//! Deterministic guest exit-code taxonomy for the rollup guest.
//!
//! Failures map to coarse, category-stable exit codes: adding or renaming an individual error never
//! renumbers a category, so codes stay meaningful to operators across guest versions. Success exits
//! 0; every category here is nonzero.
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
/// A list decoded to more elements than its wire-format bound allows (mirrors the `MAX_*`
/// constants `rollup_ssz.py` declares).
pub const CODE_BOUNDS_VIOLATION: ExitCode = 4;
/// `l2_execution_proofs` decoded successfully but is empty — the rollup guest needs at least one
/// exec proof to source its "first"/"last" echoed fields from.
pub const CODE_EMPTY_PROOFS: ExitCode = 5;
/// The output value was computed but its SSZ encoding failed (allocator exhaustion within the
/// guest's fixed heap).
pub const CODE_OUTPUT_ENCODE_FAILED: ExitCode = 6;

/// Every error a rollup guest function deliberately returns, in `exitCode`'s arm order.
pub const rollup_errors = error{
    MalformedFrame,
    InvalidSsz,
    BoundsViolation,
    EmptyProofs,
    OutputEncodeFailed,
};

/// Maps a guest failure to its deterministic, category-stable exit code.
pub fn exitCode(err: anyerror) ExitCode {
    return switch (err) {
        error.MalformedFrame => CODE_MALFORMED_FRAME,
        error.InvalidSsz => CODE_MALFORMED_SSZ,
        error.BoundsViolation => CODE_BOUNDS_VIOLATION,
        error.EmptyProofs => CODE_EMPTY_PROOFS,
        error.OutputEncodeFailed => CODE_OUTPUT_ENCODE_FAILED,
        else => CODE_UNKNOWN,
    };
}
