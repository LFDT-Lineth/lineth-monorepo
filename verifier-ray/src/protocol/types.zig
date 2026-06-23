const ext = @import("../field/koalabear_ext.zig");
const value = @import("../field/value.zig");
const commitment_mod = @import("../crypto/commitment.zig");

/// For the verifier, only oracle/public visibility values are meaningful; prover-ray's
/// internal visibility is not relevant. The numeric tags intentionally match
/// prover-ray's visibility encoding so that golden-vector tests can assert
/// both sides use the same wire values.
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
};
