const protocol = @import("../protocol/root.zig");
const ext = @import("../field/koalabear_ext.zig");
const base = @import("../field/koalabear.zig");

pub const Error = error{
    FinalSumMismatch,
    LookupResultNonZero,
    MissingDynamicModuleSize,
};

// ScalarRef locates a cell in ctx.rounds by its (round, index) coordinates.
// round is the proof.rounds index (0-based); index is the position within that
// round's cells slice. This mirrors the ObjectID.Slot() / .Position() encoding
// used by the vanishing sub-verifier's cell_value expression nodes.
pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

/// The runtime size of a Z column's module: static, or the index of the
/// proof-supplied size of a dynamic module. Mirrors vanishing.ModuleSize; kept
/// as its own copy, following this codebase's convention of each sub-verifier
/// owning its data shapes (see rowlimit.ModuleSize).
pub const ModuleSize = union(enum) {
    static: usize,
    dynamic: usize,
};

// Entry is one Z column: its average cell C and its module's size n. The
// column's total is n·C.
pub const Entry = struct {
    average_ref: ScalarRef,
    size: ModuleSize,
};

// Query is one reduced LogDerivativeSum query. The logderivativesum compiler
// turns each query into cyclic Z running-sum columns, one average cell C per
// column, with the recurrence
//
//     Z[i] − Z[i−1] = v[i] − C   on every row, row 0 reading Z[n−1]
//
// — an ordinary vanishing constraint, already discharged by the vanishing
// sub-verifier (which binds C, a cell_value leaf of the recurrence). Summed
// over the rows it makes n·C the column's total, so all that remains is the
// boundary identity:
//
//     Σ_i n_i·C_i == Result        (and, for lookups, Result == 0)
//
// average refs and result_ref are ScalarRefs that index into ctx.rounds at
// verify time, so the verifier reads from the adversary's transcript rather
// than from baked-in honest-prover values; dynamic sizes come from the same
// proof-supplied module_sizes the vanishing sub-verifier uses for the
// recurrence's domain.
pub const Query = struct {
    entries: []const Entry,
    result_ref: ScalarRef,
    result_is_zero: bool = false,
};

pub const System = struct {
    queries: []const Query = &.{},
};

fn moduleSize(size: ModuleSize, module_sizes: []const usize) Error!usize {
    return switch (size) {
        .static => |n| n,
        .dynamic => |idx| blk: {
            if (idx >= module_sizes.len) return error.MissingDynamicModuleSize;
            break :blk module_sizes[idx];
        },
    };
}

pub fn verify(system: System, ctx: protocol.Context, module_sizes: []const usize) (Error || protocol.CellError)!void {
    // Runtime loops over runtime data. When `system` was comptime these were
    // `inline for`s, so every query and every entry became straight-line
    // code — pure .text for what is a short data traversal.
    for (system.queries) |query| {
        // Σ_i n_i·C_i, reading each average cell from the transcript. `cell` is
        // bounds-checked: the refs are trusted (generated System) but the proof's
        // round/cells slices are not. A dynamic size has already been checked
        // to be a valid domain size (a power of two below the field's 2-adicity)
        // by the vanishing sub-verifier, so it is canonical in the base field.
        var sum = ext.Ext.zero();
        for (query.entries) |entry| {
            const n = try moduleSize(entry.size, module_sizes);
            const c = (try ctx.cell(entry.average_ref.round, entry.average_ref.index)).toExt();
            sum = sum.add(c.mulByBase(base.Element.init(@intCast(n))));
        }

        // The result is also read from the transcript, not baked in.
        const result = (try ctx.cell(query.result_ref.round, query.result_ref.index)).toExt();

        // The final-sum identity links the column totals to the claimed result.
        if (!sum.eql(result)) return error.FinalSumMismatch;

        // Lookup queries reduce to a LogDerivativeSum whose result must be 0.
        if (query.result_is_zero and !result.isZero()) return error.LookupResultNonZero;
    }
}
