const ext = @import("../field/koalabear_ext.zig");

pub const Error = error{
    FinalSumMismatch,
    LookupResultNonZero,
};

// Query is one reduced LogDerivativeSum query. The logderivativesum compiler
// turns each query into Z running-sum columns whose recurrence and L_0 initial
// condition are ordinary vanishing constraints — already discharged by the
// vanishing sub-verifier. All that remains is the boundary identity:
//
//     Σ_i Z_i[n-1] == Result        (and, for lookups, Result == 0)
//
// z_finals and result are concrete extension-field values extracted from the
// honest prover run; no ctx lookup is needed at verify time.
pub const Query = struct {
    z_finals: []const [6]u32,
    result: [6]u32,
    result_is_zero: bool = false,
};

pub const System = struct {
    queries: []const Query = &.{},
};

pub fn verify(comptime system: System) Error!void {
    inline for (system.queries) |query| {
        // Σ_i Z_i[n-1].
        var sum = ext.Ext.zero();
        inline for (query.z_finals) |limbs| {
            sum = sum.add(ext.Ext.fromUints(limbs));
        }

        // The final-sum identity links the Z endpoints to the claimed result.
        const result = ext.Ext.fromUints(query.result);
        if (!sum.eql(result)) return error.FinalSumMismatch;

        // Lookup queries reduce to a LogDerivativeSum whose result must be 0.
        if (query.result_is_zero and !result.isZero()) return error.LookupResultNonZero;
    }
}
