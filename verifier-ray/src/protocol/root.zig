const types = @import("types.zig");
const fiat_shamir = @import("../crypto/fiat_shamir.zig");

pub const Error = error{InvalidRoundCount};

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
};

/// All protocol-level data derived from a proof by the higher-level verifier.
/// Produced by `replay`; consumed by sub-verifiers.
pub const Context = struct {
    /// All Fiat-Shamir coins derived across every round, laid out flat.
    /// Indexed by the compiled system's `round_coin_offsets`.
    all_coins: []const Coin,
    /// The verifier-visible round messages bound into the shared transcript.
    /// Sub-verifiers read cell openings directly from `rounds[i].cells`.
    rounds: []const RoundMessage,
};

pub fn ReplayStats(comptime coin_count: usize) type {
    return struct {
        coins: [coin_count]Coin,
        compression_count: usize,
    };
}

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
pub fn replay(
    comptime spec: Spec,
    rounds: []const RoundMessage,
) Error![spec.total_round_coins]Coin {
    return (try replayWithStats(spec, rounds)).coins;
}

/// Replays the transcript and returns benchmark-friendly Poseidon2 statistics.
///
/// `replay` intentionally keeps the original API and delegates here, so coin
/// generation remains implemented in one place.
pub fn replayWithStats(
    comptime spec: Spec,
    rounds: []const RoundMessage,
) Error!ReplayStats(spec.total_round_coins) {
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

    var transcript = fiat_shamir.Transcript.init();
    var all_coins: [spec.total_round_coins]Coin = undefined;

    inline for (1..spec.round_coin_counts.len) |round_index| {
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

    return .{
        .coins = all_coins,
        .compression_count = transcript.compressionCount(),
    };
}
