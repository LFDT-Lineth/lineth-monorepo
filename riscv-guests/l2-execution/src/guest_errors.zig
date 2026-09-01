//! Deterministic guest exit-code taxonomy (Readme.md §2.5 "Guest Termination Semantics").
//!
//! Failures map to coarse, category-stable exit codes: adding or renaming an individual error never
//! renumbers a category, so codes stay meaningful to operators across guest versions. Success exits
//! 0; every category here is nonzero, per the standard's failed-termination requirement.
//!
//! Self-contained by design: Zig error literals are global (matched by name program-wide), so this
//! file references every error it classifies — Linea-layer and zesu-engine alike — without importing
//! the modules that raise them. The only import is `std`.
//!
//! Adding a new Linea-layer error means adding it to BOTH `linea_errors` and `exitCode`'s switch in
//! the same change — the comptime guard in the guest's test suite fails on any `linea_errors`
//! member that `exitCode` leaves at `ExitCode.unknown`.

const std = @import("std");

pub const ExitCode = enum {
    /// Clean termination: the conflation executed and the output committed. First member, so 0.
    success,
    /// Fallback for anything not yet triaged — an unmapped error still fails the guest with a
    /// nonzero exit. Engine/zesu outcomes have their own categories below (`engine_decode`,
    /// `engine_reject`), and `error.OutOfMemory` its own capacity signal, so a landing here means
    /// the failure came from somewhere the taxonomy doesn't yet name (envelope, host, or an
    /// untriaged Linea-layer path).
    unknown,
    invalid_ssz_envelope,
    invalid_stateless_input,
    conflation_invariant,
    policy_reject,
    forced_tx_violation,
    /// Witness-data failures: a witness node needed to resolve an MPT proof path is missing from
    /// the pool, an L1->L2 rolling-hash number read from bridge storage overflows u64 or decreases
    /// across the conflation, a `MessageSent` log, header-chain RLP, or witness payload fails to
    /// decode, or a witness miss surfaces through the EVM's witness-backed database during
    /// delegated per-block execution (`WitnessDbResolution`). A proof of ABSENCE resolves to
    /// `null`/`0` instead, never here.
    witness_resolution,
    /// Resource exhaustion: the fixed-size guest heap (a `FixedBufferAllocator`) ran out. Points
    /// at capacity, not logic — the input was too large for the configured heap, so the operator
    /// response is a bigger heap or a smaller witness, not a code fix.
    out_of_memory,
    /// The stateless-execution engine could not decode the input/witness bytes (RLP/SSZ/hex or
    /// container shape). The block never got as far as semantic validation.
    engine_decode,
    /// The stateless-execution engine decoded the block but rejected it as semantically invalid
    /// (state/receipts root mismatch, gas-limit or gas-used violation, invalid BAL, and the rest
    /// of block/tx validation). This is a proof-rejection outcome: the block is invalid.
    engine_reject,
};

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
    WitnessDbResolution,

    EngineDecode,
    EngineReject,
};

/// Maps a guest failure to its deterministic, category-stable exit code (Readme.md §2.5).
pub fn exitCode(err: anyerror) ExitCode {
    return switch (err) {
        error.InvalidSsz => .invalid_ssz_envelope,

        error.InvalidStatelessInput => .invalid_stateless_input,

        error.EmptyPayloads,
        error.ChainIdMismatch,
        error.ParentHashChainMismatch,
        error.BaseFeeNotConstant,
        error.FeeRecipientMismatch,
        error.MissingParentHeaderWitness,
        error.InvalidGenesisParentHash,
        => .conflation_invariant,

        error.ExecutionRequestsNotSupported,
        error.WithdrawalsNotSupported,
        error.UnsupportedFork,
        => .policy_reject,

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
        => .forced_tx_violation,

        error.RollingHashNumberOverflow,
        error.RollingHashNumberDecreased,
        error.InvalidBridgeMessageLog,
        error.InvalidProof,
        error.InvalidWitness,
        error.WitnessDbResolution,
        => .witness_resolution,

        error.OutOfMemory => .out_of_memory,

        // Zesu/engine outcomes, re-namespaced at the delegation boundary: the engine's own
        // InvalidProof/InvalidWitness/InvalidSsz share names with Linea-layer errors, so they're
        // collapsed into one of these two to land in an engine category, not a Linea one.
        error.EngineDecode => .engine_decode,
        error.EngineReject => .engine_reject,

        else => .unknown,
    };
}

/// Errors a wrapped zesu call may raise that are Linea-meaningful and keep their exit-code
/// category. `UnsupportedFork`/`OutOfMemory` map to their own categories; `WitnessDbResolution`
/// is the Linea-layer name for a witness miss surfaced through zesu's `WitnessDatabase`
/// (`ctx.ctx_error`), distinct from zesu's own `InvalidWitness`.
pub const ZesuPassthrough = error{ UnsupportedFork, OutOfMemory, WitnessDbResolution };

/// The zesu errors that mean "the input/witness bytes failed to decode" (RLP/SSZ/hex/container
/// shape), as opposed to "the decoded block is semantically invalid". Membership is by name and
/// tracks zesu's decode-layer vocabulary; a zesu bump that adds a decode error should add it here.
pub const ZesuDecode = error{
    InvalidRlp,
    InvalidSsz,
    BlockRlpTooLarge,
    InvalidHex,
    OddHexLength,
    MissingField,
    UnexpectedNull,
    InvalidSliceLength,
    InvalidDupIndex,
    InvalidHp,
    InvalidNode,
    NonCanonicalFieldElement,
    UnexpectedBlobFields,
};

/// Re-namespaces a zesu-origin error into a coarse engine category. Zig matches error literals by
/// name program-wide, so zesu's own `InvalidProof`/`InvalidWitness`/`InvalidSsz` would otherwise
/// be mis-filed into the Linea-layer witness/SSZ exit-code categories that share those names.
/// Mapping: a `ZesuPassthrough` member passes through unchanged; a `ZesuDecode` member becomes
/// `error.EngineDecode` (input bytes malformed); every other zesu error becomes
/// `error.EngineReject` (the engine rejected the block) — the right default, since anything
/// escaping per-block execution is an engine rejection. `pub` so the guest's test suite can pin
/// the wrap decision directly (origin, not name, determines the category).
pub fn zesuErr(err: anyerror) (ZesuPassthrough || error{ EngineDecode, EngineReject }) {
    // Reconstruct a matched member from its set by name: an anyerror value can't be cast into the
    // narrower inferred set directly, but @field yields a properly-typed member.
    const name = @errorName(err);
    inline for (@typeInfo(ZesuPassthrough).error_set.?) |e|
        if (std.mem.eql(u8, e.name, name)) return @field(ZesuPassthrough, e.name);
    inline for (@typeInfo(ZesuDecode).error_set.?) |e|
        if (std.mem.eql(u8, e.name, name)) return error.EngineDecode;
    return error.EngineReject;
}
