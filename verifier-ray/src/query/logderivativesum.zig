const protocol = @import("../protocol/root.zig");
const ext = @import("../field/koalabear_ext.zig");

pub const Error = error{
    FinalSumMismatch,
    LookupResultNonZero,
};

// ScalarRef locates a cell in ctx.rounds by its (round, index) coordinates.
// round is the proof.rounds index (0-based); index is the position within that
// round's cells slice. This mirrors the ObjectID.Slot() / .Position() encoding
// used by the vanishing sub-verifier's cell_value expression nodes.
pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

// Query is one reduced LogDerivativeSum query. The logderivativesum compiler
// turns each query into Z running-sum columns whose recurrence and L_0 initial
// condition are ordinary vanishing constraints — already discharged by the
// vanishing sub-verifier. All that remains is the boundary identity:
//
//     Σ_i Z_i[n-1] == Result        (and, for lookups, Result == 0)
//
// z_final_refs and result_ref are ScalarRefs that index into ctx.rounds at
// verify time, so the verifier reads from the adversary's transcript rather
// than from baked-in honest-prover values.
pub const Query = struct {
    z_final_refs: []const ScalarRef,
    result_ref: ScalarRef,
    result_is_zero: bool = false,
};

pub const System = struct {
    queries: []const Query = &.{},
};

pub fn verify(comptime system: System, ctx: protocol.Context) Error!void {
    inline for (system.queries) |query| {
        // Σ_i Z_i[n-1], reading each Z endpoint from the transcript.
        var sum = ext.Ext.zero();
        inline for (query.z_final_refs) |ref| {
            sum = sum.add(ctx.rounds[ref.round].cells[ref.index].toExt());
        }

        // The result is also read from the transcript, not baked in.
        const result = ctx.rounds[query.result_ref.round].cells[query.result_ref.index].toExt();

        // The final-sum identity links the Z endpoints to the claimed result.
        if (!sum.eql(result)) return error.FinalSumMismatch;

        // Lookup queries reduce to a LogDerivativeSum whose result must be 0.
        if (query.result_is_zero and !result.isZero()) return error.LookupResultNonZero;
    }
}
