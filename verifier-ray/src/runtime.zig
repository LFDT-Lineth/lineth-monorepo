const commitment_mod = @import("crypto/commitment.zig");
const fiat_shamir = @import("crypto/fiat_shamir.zig");
const ext = @import("field/koalabear_ext.zig");
const value = @import("field/value.zig");

pub const Error = error{
    NoRounds,
    UnexpectedRound,
    LastRound,
    OutputTooSmall,
};

/// For the verifier, only oracle/public visibility values are meaningful; prover-ray's
/// internal visibility is not relevant. The numeric tags intentionally match
/// prover-ray's visibility encoding.
pub const Visibility = enum(u8) {
    oracle = 1,
    public = 2,
};

pub const Vector = value.Vector;
pub const Scalar = value.Scalar;
pub const Coin = ext.Ext;
pub const Commitment = commitment_mod.Commitment;

pub const ColumnMessage = union(enum) {
    oracle_commitment: Commitment,
    public_column: Vector,
};

/// Verifier-visible data sent before deriving the next round's coins. Oracle
/// columns are represented only by their commitments; public columns and cells
/// carry their raw values because the verifier sees them directly.
pub const RoundMessage = struct {
    columns: []const ColumnMessage = &.{},
    cells: []const Scalar = &.{},
    next_round_coin_count: usize = 0,
};

pub const Runtime = struct {
    transcript: fiat_shamir.Transcript,
    current_round: usize,
    total_rounds: usize,

    pub fn initWithRoundCount(round_count: usize) Runtime {
        return .{
            .transcript = fiat_shamir.Transcript.init(),
            .current_round = 0,
            .total_rounds = round_count,
        };
    }

    /// Absorbs one round's verifier-visible messages, advances the round counter,
    /// and derives the extension-field coins used by the next round.
    ///
    /// `expected_round` must match the runtime's current round and must not be the
    /// final round, because there is no next round to receive derived coins.
    ///
    /// `message` contains oracle commitments, public column assignments, and
    /// public cells that are absorbed into the Fiat-Shamir transcript before
    /// any coins are sampled.
    ///
    /// `out_coins` is caller-owned backing storage. The runtime writes exactly
    /// `message.next_round_coin_count` coins into the beginning of that slice and
    /// returns the initialized prefix.
    pub fn advanceRoundWithMessage(
        self: *Runtime,
        expected_round: usize,
        message: RoundMessage,
        out_coins: []Coin,
    ) Error![]const Coin {
        if (self.total_rounds == 0) return Error.NoRounds;
        if (self.current_round != expected_round or expected_round >= self.total_rounds) {
            return Error.UnexpectedRound;
        }
        if (self.current_round + 1 >= self.total_rounds) return Error.LastRound;
        if (message.next_round_coin_count > out_coins.len) return Error.OutputTooSmall;

        for (message.columns) |column| {
            switch (column) {
                .oracle_commitment => |commitment| self.transcript.updateElements(&commitment),
                .public_column => |public_column| self.transcript.absorbVector(public_column),
            }
        }
        for (message.cells) |cell| {
            self.transcript.absorbScalar(cell);
        }

        self.current_round += 1;
        const coins = out_coins[0..message.next_round_coin_count];
        for (coins) |*coin| {
            coin.* = self.transcript.randomExt();
        }
        return coins;
    }
};
