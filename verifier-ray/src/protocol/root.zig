const types = @import("types.zig");
const fiat_shamir = @import("../crypto/fiat_shamir.zig");
const field = @import("../field/koalabear.zig");

pub const Error = error{ InvalidRoundCount, MissingDynamicModuleSize };

/// Raised when a sub-verifier's (comptime) cell reference points outside the
/// (runtime) round messages a proof actually carries. The `(round, index)`
/// coordinates come from codegen and are correct for an honest proof, but the
/// round messages are prover-supplied and their per-round `cells` lengths are
/// not otherwise pinned by `Spec`; without this check a malformed proof with a
/// short (or empty) `cells` slice would index out of bounds — a panic in a safe
/// build, UB in ReleaseFast. Surfacing it as an error makes a malformed proof a
/// clean rejection instead.
pub const CellError = error{CellRefOutOfRange};

pub const Visibility = types.Visibility;
pub const Vector = types.Vector;
pub const Scalar = types.Scalar;
pub const Coin = types.Coin;
pub const Commitment = types.Commitment;
pub const ColumnMessage = types.ColumnMessage;
pub const RoundMessage = types.RoundMessage;

/// Compile-time coin-routing specification shared across all sub-verifiers.
/// Extracted from the compiled IOP system by the Go codegen and emitted as a
/// standalone constant in the generated file alongside `verifier_mod.Systems`.
pub const Spec = struct {
    /// Number of coins squeezed after each round. Index 0 is always 0;
    /// the first coins are derived after the first round message is absorbed.
    round_coin_counts: []const usize,
    /// Starting position of each round's coins in the flat `all_coins` array.
    round_coin_offsets: []const usize,
    /// Total number of coins across all rounds; length of `all_coins`.
    total_round_coins: usize,
    /// The `module_sizes` slots to absorb into the transcript at the START of
    /// every round, in ascending `System.Modules` order — the exact order and
    /// value prover-ray's `AdvanceRound` feeds each dynamic module's size into
    /// Fiat-Shamir (`wiop/wiop_runtime.go` `AdvanceRound`). Binding the sizes
    /// into the transcript makes every coin — zeta, fold alphas, query positions
    /// — depend on the claimed `n`, closing the gap where a prover could reuse an
    /// opening under a different dynamic size. Empty for a fully-static protocol.
    ///
    /// Each entry is an index into the runtime `module_sizes` slice. Distinct
    /// from the vanishing `DynamicIndex`: the slot list is the FS absorb schedule,
    /// codegen emits both from the same ascending order so they coincide.
    dynamic_size_slots: []const usize = &.{},
};

/// All protocol-level data derived from a proof by the higher-level verifier.
/// Produced by `replay`; consumed by sub-verifiers.
pub const Context = struct {
    /// All Fiat-Shamir coins derived across every round, laid out flat.
    /// Indexed by the compiled system's `round_coin_offsets`.
    all_coins: []const Coin,
    /// The verifier-visible round messages bound into the shared transcript.
    /// Sub-verifiers read cell openings via `cell(round, index)`.
    rounds: []const RoundMessage,

    /// Bounds-checked read of a cell scalar at `(round, index)`. Returns
    /// `CellRefOutOfRange` if the proof's round messages do not carry that cell,
    /// rather than indexing out of bounds. Every sub-verifier that consumes a
    /// codegen-emitted cell reference (vanishing `cell_value`, logderivativesum
    /// `z_final`/`result`) MUST route through this instead of raw indexing.
    pub fn cell(self: Context, round: usize, index: usize) CellError!Scalar {
        if (round >= self.rounds.len) return CellError.CellRefOutOfRange;
        const cells = self.rounds[round].cells;
        if (index >= cells.len) return CellError.CellRefOutOfRange;
        return cells[index];
    }
};

/// Replays the prover–verifier transcript to derive all Fiat-Shamir coins.
///
/// For each message round, absorbs the round's oracle commitments, public
/// columns, and cell scalars into the Poseidon2 Merkle-Damgård transcript, then
/// squeezes that round's coins into `all_coins` at the position fixed by `spec`.
/// This is the only function that touches the Fiat-Shamir transcript.
///
/// `spec.round_coin_counts[0]` is the pre-round-1 phase and is always 0, so the
/// message rounds are `round_coin_counts[1..]`; `rounds` must have that length.
/// `spec` is comptime-validated for internal consistency, so its callers — both
/// `verifier.verify` and direct test callers — get the same guarantees.
///
/// The transcript is CALLER-OWNED and passed by pointer: this function absorbs
/// the round messages and squeezes the protocol coins into it, leaving it at the
/// state after the final round. A sub-verifier that must continue squeezing from
/// that same state (the FRI/PCS opener derives its fold challenges and query
/// positions there) is handed the SAME `transcript` pointer next, mirroring
/// prover-ray's `fs := rt.GetFS()`. This keeps `protocol` FRI-agnostic: it owns
/// only the protocol-coin phase; each scheme owns its own continuation.
pub fn replayWithTranscript(
    transcript: *fiat_shamir.Transcript,
    comptime spec: Spec,
    rounds: []const RoundMessage,
    module_sizes: []const usize,
) Error![spec.total_round_coins]Coin {
    comptime {
        if (spec.round_coin_counts.len == 0)
            @compileError("spec: round_coin_counts must have at least one entry (the pre-round-1 phase)");
        if (spec.round_coin_counts[0] != 0)
            @compileError("spec: round_coin_counts[0] must be 0 — no coins are derived before the first round is absorbed");
        if (spec.round_coin_offsets.len != spec.round_coin_counts.len)
            @compileError("spec: round_coin_offsets and round_coin_counts must have equal length");
        var expected_offset: usize = 0;
        for (spec.round_coin_counts, spec.round_coin_offsets) |count, offset| {
            if (offset != expected_offset)
                @compileError("spec: round_coin_offsets must be prefix sums of round_coin_counts");
            expected_offset += count;
        }
        if (spec.total_round_coins != expected_offset)
            @compileError("spec: total_round_coins must equal sum of round_coin_counts");
    }

    // round_coin_counts[0] is the pre-round-1 phase, so there is one message
    // round per remaining entry.
    if (rounds.len != spec.round_coin_counts.len - 1) return error.InvalidRoundCount;

    var all_coins: [spec.total_round_coins]Coin = undefined;

    inline for (1..spec.round_coin_counts.len) |round_index| {
        const message = rounds[round_index - 1];
        // Dynamic module sizes are absorbed FIRST each round, before the round's
        // commitments/columns/cells — byte-for-byte prover-ray's `AdvanceRound`
        // (sizes, then commitment, then columns, then cells). Absorbing here
        // (rather than once, up front) preserves the transcript state at every
        // squeeze point, so the per-round coins match the reference.
        for (spec.dynamic_size_slots) |slot| {
            if (slot >= module_sizes.len) return Error.MissingDynamicModuleSize;
            transcript.updateElement(field.Element.init(@intCast(module_sizes[slot])));
        }
        for (message.columns) |entry| {
            switch (entry) {
                .oracle_commitment => |c| transcript.updateElements(&c),
                .public_column => |col| transcript.absorbVector(col),
            }
        }
        for (message.cells) |cell| transcript.absorbScalar(cell);

        const offset = spec.round_coin_offsets[round_index];
        const count = spec.round_coin_counts[round_index];
        for (all_coins[offset..][0..count]) |*coin| coin.* = transcript.randomExt();
    }

    return all_coins;
}

/// Coin-only convenience over `replayWithTranscript` for callers that do not
/// continue the transcript (every sub-verifier except a transcript-continuing
/// one like PCS). Uses a throwaway local transcript.
pub fn replay(
    comptime spec: Spec,
    rounds: []const RoundMessage,
    module_sizes: []const usize,
) Error![spec.total_round_coins]Coin {
    var transcript = fiat_shamir.Transcript.init();
    return replayWithTranscript(&transcript, spec, rounds, module_sizes);
}
