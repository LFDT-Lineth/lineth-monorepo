const types = @import("types.zig");
const fiat_shamir = @import("../crypto/fiat_shamir.zig");
const field = @import("../field/koalabear.zig");

/// Error from the bounds-checked cell accessor `Context.cell`. Split out so
/// sub-verifiers can compose it into their own error sets.
pub const CellError = error{CellRefOutOfRange};

pub const Error = error{ InvalidRoundCount, MissingDynamicModuleSize } || CellError;

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
    /// Number of dynamically-sized modules whose runtime sizes the prover
    /// absorbs into the transcript at every round advance (prover-ray's
    /// `Runtime.AdvanceRound`, which feeds `NewElement(size)` for each dynamic
    /// module before the round's commitment/columns/cells). `replayWithTranscript`
    /// mirrors this by absorbing the first `dynamic_module_count` entries of the
    /// caller-supplied `module_sizes` at the start of each round. 0 for protocols
    /// with no dynamic modules, in which case the absorption is a no-op and
    /// `module_sizes` may be empty.
    dynamic_module_count: usize = 0,
};

/// All protocol-level data derived from a proof by the higher-level verifier.
/// Produced by `replayWithTranscript`; consumed by sub-verifiers.
pub const Context = struct {
    /// All Fiat-Shamir coins derived across every round, laid out flat.
    /// Indexed by the compiled system's `round_coin_offsets`.
    all_coins: []const Coin,
    /// The verifier-visible round messages bound into the shared transcript.
    /// Sub-verifiers read cell openings via `cell()`.
    rounds: []const RoundMessage,

    /// Bounds-checked access to a transcript cell by its (round, index) ref.
    /// `round`/`index` come from the trusted comptime System, but `rounds` and
    /// each round's `cells` slice length come from the (untrusted) proof, so an
    /// adversarial proof with a short round/cells slice would otherwise read out
    /// of bounds — a memory-safety issue in bounds-check-off R5 builds. Returns an
    /// error rather than trapping/reading garbage. (Soundness does not depend on
    /// this: PCS runs first and rejects malformed proofs; this is robustness.)
    pub fn cell(self: Context, round: usize, index: usize) CellError!Scalar {
        if (round >= self.rounds.len) return error.CellRefOutOfRange;
        const cells = self.rounds[round].cells;
        if (index >= cells.len) return error.CellRefOutOfRange;
        return cells[index];
    }
};

/// Replays the prover–verifier transcript to derive all Fiat-Shamir coins,
/// using a *caller-owned* `transcript`.
///
/// For each message round, absorbs the round's oracle commitments, public
/// columns, and cell scalars into the Poseidon2 Merkle-Damgård transcript, then
/// squeezes that round's coins into `all_coins` at the position fixed by `spec`.
///
/// The transcript is passed in by pointer and left in place after the last
/// protocol coin is squeezed, so a transcript-continuing sub-verifier (e.g. PCS,
/// which derives its own FRI challenges from the same Fiat-Shamir state) can
/// resume from exactly here. The protocol layer stays sub-verifier-agnostic: it
/// only knows about round messages and protocol coins, never the FRI squeeze
/// schedule.
///
/// `spec.round_coin_counts[0]` is the pre-round-1 phase and is always 0, so the
/// message rounds are `round_coin_counts[1..]`; `rounds` must have that length.
/// `spec` is comptime-validated for internal consistency, so its callers — both
/// `verifier.verify` and direct test callers — get the same guarantees.
pub fn replayWithTranscript(
    transcript: *fiat_shamir.Transcript,
    comptime spec: Spec,
    rounds: []const RoundMessage,
    /// Runtime sizes of the protocol's dynamically-sized modules, in the prover's
    /// module order. The first `spec.dynamic_module_count` entries are absorbed
    /// into the transcript at every round advance, mirroring prover-ray's
    /// `AdvanceRound`. May be empty when `spec.dynamic_module_count == 0`.
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

    // Dynamic-module sizes are absorbed once per round advance, so the caller
    // must supply at least `dynamic_module_count` of them.
    if (module_sizes.len < spec.dynamic_module_count) return error.MissingDynamicModuleSize;

    var all_coins: [spec.total_round_coins]Coin = undefined;

    inline for (1..spec.round_coin_counts.len) |round_index| {
        // Mirror prover-ray's `Runtime.AdvanceRound`: before absorbing the
        // round's commitment/columns/cells, feed each dynamic module's runtime
        // size (as a base-field element) into the transcript. Runs at every round
        // advance, so a size is absorbed once per replayed round — matching the
        // prover exactly, which is what keeps the derived eval coin `r` in sync.
        if (comptime spec.dynamic_module_count > 0) {
            for (module_sizes[0..spec.dynamic_module_count]) |size| {
                transcript.updateElement(field.Element.init(@intCast(size)));
            }
        }

        const message = rounds[round_index - 1];
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
