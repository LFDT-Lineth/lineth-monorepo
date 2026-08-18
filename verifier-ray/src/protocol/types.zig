const ext = @import("../field/koalabear_ext.zig");
const value = @import("../field/value.zig");
const commitment_mod = @import("../crypto/commitment.zig");

pub const Scalar = value.Scalar;
pub const Coin = ext.Ext;
pub const Commitment = commitment_mod.Commitment;

/// Verifier-visible data sent before deriving the next round's coins.
/// prover-ray columns never travel raw: a committed round is represented
/// solely by its Merkle root (`commitment`); a round with no committed columns
/// carries none. Cells always carry their raw scalar values because the
/// verifier is meant to observe them directly.
pub const RoundMessage = struct {
    commitment: ?Commitment = null,
    cells: []const Scalar = &.{},
};
