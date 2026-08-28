const protocol = @import("../protocol/root.zig");
const base = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");

pub const Error = error{
    FinalProductMismatch,
    ResultMismatch,
};

// ScalarRef locates a cell in ctx.rounds by its (round, index) coordinates.
// round is the proof.rounds index (0-based); index is the position within that
// round's cells slice. Mirrors logderivativesum.ScalarRef / the vanishing
// sub-verifier's cell_value expression nodes.
pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

// Query is one reduced wiop.GrandProduct. The grandproduct compiler turns it
// into Z running-product columns whose recurrence and row-0 boundary are
// ordinary vanishing constraints — already discharged by the vanishing
// sub-verifier. What remains is:
//
//     ∏_i Z_i[n-1] == Result                        (always)
//     Result == expected.?                           (when expected != null)
//
// The second identity is whichever of prover-ray's grandproduct.CheckResultIsOne
// (permutation arguments; expected == 1) or messagebus.CheckHandleSumInShard
// (message-bus handles; expected is the shard's expected accumulator, one in
// the unsharded case) the compiler attached to this GrandProduct — or neither,
// when a message-bus handle's in-shard check is skipped in favour of a
// downstream cross-shard layer (`expected == null`).
//
// z_final_refs and result_ref are ScalarRefs that index into ctx.rounds at
// verify time, so the verifier reads from the adversary's transcript rather
// than from baked-in honest-prover values.
pub const Query = struct {
    z_final_refs: []const ScalarRef,
    result_ref: ScalarRef,
    expected: ?u64 = null,
};

pub const System = struct {
    queries: []const Query = &.{},
};

pub fn verify(comptime system: System, ctx: protocol.Context) (Error || protocol.CellError)!void {
    inline for (system.queries) |query| {
        // ∏_i Z_i[n-1], reading each Z endpoint from the transcript. `cell` is
        // bounds-checked: the refs are trusted (comptime System) but the proof's
        // round/cells slices are not.
        var prod = ext.Ext.one();
        inline for (query.z_final_refs) |ref| {
            prod = prod.mul((try ctx.cell(ref.round, ref.index)).toExt());
        }

        // The result is also read from the transcript, not baked in.
        const result = (try ctx.cell(query.result_ref.round, query.result_ref.index)).toExt();

        // The final-product identity links the Z endpoints to the claimed result.
        if (!prod.eql(result)) return error.FinalProductMismatch;

        // Whichever boundary predicate the compiler attached — permutation's
        // Result == 1 or message-bus's Result == expected — reduces to this
        // single equality against a compile-time constant.
        if (query.expected) |e| {
            const want = ext.Ext.lift(base.Element.init(e));
            if (!result.eql(want)) return error.ResultMismatch;
        }
    }
}
