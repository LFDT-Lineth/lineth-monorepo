//! Deterministic guest exit-code taxonomy (Readme.md §2.5 "Guest Termination Semantics").
//!
//! Failures map to coarse, category-stable exit codes: adding or renaming an individual error never
//! renumbers a category, so codes stay meaningful to operators across guest versions. Success exits
//! 0; every category here is nonzero, per the standard's failed-termination requirement.
//!
//! Standalone by design: Zig error literals are global (matched by name program-wide), so this file
//! needs no imports.
//!
//! Adding a new Linea-layer error means adding it to BOTH `linea_errors` and `exitCode`'s switch in
//! the same change — the comptime guard in the guest's test suite fails on any `linea_errors`
//! member that `exitCode` leaves at CODE_UNKNOWN.

/// Matches the width of the guest's exit primitive.
pub const ExitCode = u64;

/// Fallback for engine/EVM/zesu-internal errors and anything not yet triaged — an unmapped error
/// still fails the guest with a nonzero exit.
pub const CODE_UNKNOWN: ExitCode = 1;
pub const CODE_INVALID_SSZ_ENVELOPE: ExitCode = 2;
pub const CODE_INVALID_STATELESS_INPUT: ExitCode = 3;
pub const CODE_CONFLATION_INVARIANT: ExitCode = 4;
pub const CODE_POLICY_REJECT: ExitCode = 5;
pub const CODE_FORCED_TX_VIOLATION: ExitCode = 6;
/// A node needed to resolve an MPT proof path was missing from the witness pool (a proof of ABSENCE
/// resolves to `null`/`0` instead, never here), at whichever layer surfaces the read: a direct
/// Linea-layer MPT read, the witness header chain, or the EVM's witness-backed database during
/// delegated per-block execution.
pub const CODE_WITNESS_RESOLUTION: ExitCode = 7;

/// Every error a Linea-layer function deliberately returns, in `exitCode`'s arm order.
pub const linea_errors = error{
    InvalidSsz,

    InvalidStatelessInput,

    EmptyPayloads,
    ChainIdMismatch,
    ParentHashChainMismatch,
    BaseFeeNotConstant,
    FeeRecipientMismatch,
    MissingParentHeaderWitness,
    InvalidGenesisParentHash,

    ExecutionRequestsNotSupported,
    WithdrawalsNotSupported,
    UnsupportedFork,

    ForcedTxOutOfOrder,
    ForcedTxDeadlineExceeded,
    ForcedTxSenderRecoveryFailed,
    UnknownForcedTxAcceptance,
    IncludedForcedTxNotInBlock,
    InvalidForcedTxFoundInBlock,
    BadNonceMismatch,
    BadBalanceMismatch,
    FilteredAddressToOnContractCreation,
    ForcedTxSenderAbsent,

    RollingHashNumberOverflow,
    RollingHashNumberDecreased,
    InvalidBridgeMessageLog,
    InvalidProof,
    InvalidWitness,
};

/// Maps a guest failure to its deterministic, category-stable exit code (Readme.md §2.5).
pub fn exitCode(err: anyerror) ExitCode {
    return switch (err) {
        error.InvalidSsz => CODE_INVALID_SSZ_ENVELOPE,

        error.InvalidStatelessInput => CODE_INVALID_STATELESS_INPUT,

        error.EmptyPayloads,
        error.ChainIdMismatch,
        error.ParentHashChainMismatch,
        error.BaseFeeNotConstant,
        error.FeeRecipientMismatch,
        error.MissingParentHeaderWitness,
        error.InvalidGenesisParentHash,
        => CODE_CONFLATION_INVARIANT,

        error.ExecutionRequestsNotSupported,
        error.WithdrawalsNotSupported,
        error.UnsupportedFork,
        => CODE_POLICY_REJECT,

        error.ForcedTxOutOfOrder,
        error.ForcedTxDeadlineExceeded,
        error.ForcedTxSenderRecoveryFailed,
        error.UnknownForcedTxAcceptance,
        error.IncludedForcedTxNotInBlock,
        error.InvalidForcedTxFoundInBlock,
        error.BadNonceMismatch,
        error.BadBalanceMismatch,
        error.FilteredAddressToOnContractCreation,
        error.ForcedTxSenderAbsent,
        => CODE_FORCED_TX_VIOLATION,

        error.RollingHashNumberOverflow,
        error.RollingHashNumberDecreased,
        error.InvalidBridgeMessageLog,
        error.InvalidProof,
        error.InvalidWitness,
        => CODE_WITNESS_RESOLUTION,

        else => CODE_UNKNOWN,
    };
}
