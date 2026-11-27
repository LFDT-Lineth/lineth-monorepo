const protocol = @import("../protocol/root.zig");
const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");

pub const Error = error{
    E6Mismatch,
    E6LaneNotBaseField,
    E6CellNotExtensionField,
} || protocol.CellError;

/// Number of base lanes of an E6 value (recursion.KindE6).
pub const lanes = 6;

/// ScalarRef locates a cell in ctx.rounds by its (round, index) coordinates.
/// Mirrors shared_randomness.ScalarRef / grandproduct.ScalarRef.
pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

/// One recursion.E6Check: the extension cell `cell` (an E6 public input of a
/// recursion program) must equal the E6 value of the six base cells `lanes`,
/// the openings of its public chip row, in the order
/// (B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1) of recursion.CellE6.
pub const Check = struct {
    cell: ScalarRef,
    lanes: [lanes]ScalarRef,
};

/// System is every E6Check registered on a compiled system.
pub const System = struct {
    checks: []const Check = &.{},
};

/// Verifies every E6Check: the verifier-side counterpart of prover-ray's
/// recursion.E6Check.Check. It is the only check tying an E6 public input to
/// the proof — wiop has no extension constant, so the recomposition cannot be
/// a constraint, and the vanishing and PCS checks bind only the lanes. Without
/// it a prover could claim any E6 public input.
///
/// Both the lanes and the public input are read from `ctx` (the adversary's
/// transcript); the public inputs are already merged into `ctx.rounds[*].cells`
/// by `verifier.verify` before any sub-verifier runs. A lane must be a base
/// field element and the public input an extension element, as the codegen
/// (BuildE6CheckSystem) requires of the declared cells. Under `verifier.verify`
/// the public-input binding already rejects a cell whose tag differs from its
/// declaration (CellFieldMismatch); checking the tag here keeps this
/// sub-verifier sound when called on its own.
///
/// A `System{}` zero value (no checks) verifies trivially: a system exposing
/// no E6 public input has nothing for this sub-verifier to enforce.
pub fn verify(system: System, ctx: protocol.Context) Error!void {
    for (system.checks) |check| {
        var lane: [lanes]field.Element = undefined;
        for (check.lanes, &lane) |ref, *dst| {
            dst.* = switch (try ctx.cell(ref.round, ref.index)) {
                .base => |b| b,
                .ext => return error.E6LaneNotBaseField,
            };
        }
        const want = ext.Ext{
            .B0 = .{ .a0 = lane[0], .a1 = lane[1] },
            .B1 = .{ .a0 = lane[2], .a1 = lane[3] },
            .B2 = .{ .a0 = lane[4], .a1 = lane[5] },
        };
        const got = switch (try ctx.cell(check.cell.round, check.cell.index)) {
            .ext => |e| e,
            .base => return error.E6CellNotExtensionField,
        };
        if (!want.eql(got)) return error.E6Mismatch;
    }
}
