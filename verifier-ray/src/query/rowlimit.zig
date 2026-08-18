pub const Error = error{
    MissingDynamicModuleSize,
    RowLimitExceeded,
};

/// The runtime size of one module contributing to a row-limit check's side sum.
/// Mirrors vanishing.ModuleSize; kept as its own copy rather than shared,
/// following this codebase's convention of each sub-verifier owning its data
/// shapes independently (see e.g. pcs.SizeSource, which is shaped differently
/// again for its own needs).
pub const ModuleSize = union(enum) {
    static: usize,
    dynamic: usize,
};

/// One subgroup's row-limit check: prover-ray's lookuptologderivsum compiler
/// bin-packs lookups sharing an includings table into subgroups that share a
/// multiplicity column M, so each subgroup drains its own row budget
/// independently. included_modules/includings_modules list one ModuleSize per
/// fragment on each side (a repeated module appears once per fragment,
/// deliberately double-counting it — each fragment is its own pass over that
/// module's rows).
pub const Check = struct {
    included_modules: []const ModuleSize,
    includings_modules: []const ModuleSize,
    limit: u64,
};

pub const System = struct {
    checks: []const Check = &.{},
};

/// Sums the runtime row count of every module in `modules`, resolving dynamic
/// sizes from `module_sizes` (the proof-supplied per-dynamic-module sizes, in
/// the same order prover-ray's Runtime.AdvanceRound absorbs them).
fn sumRows(modules: []const ModuleSize, module_sizes: []const usize) Error!u64 {
    var sum: u64 = 0;
    for (modules) |m| {
        sum += switch (m) {
            .static => |n| n,
            .dynamic => |idx| blk: {
                if (idx >= module_sizes.len) return error.MissingDynamicModuleSize;
                break :blk module_sizes[idx];
            },
        };
    }
    return sum;
}

/// Verifies every row-limit check in `system`. This is the verifier-side
/// counterpart to prover-ray's lookuptologderivsum.RowLimitVerifierAction.Check:
/// a dynamic module's runtime size is prover-declared and unbound by anything
/// else in the transcript (PCS commits to the module's data, not its claimed
/// size), so this check is what stops a prover from claiming a far larger
/// dynamic size than it compiled against — one whose accumulators would
/// overflow the small field the reduced lookup constraints run in.
pub fn verify(comptime system: System, module_sizes: []const usize) Error!void {
    inline for (system.checks) |check| {
        const included_rows = try sumRows(check.included_modules, module_sizes);
        if (included_rows >= check.limit) return error.RowLimitExceeded;

        const includings_rows = try sumRows(check.includings_modules, module_sizes);
        if (includings_rows >= check.limit) return error.RowLimitExceeded;
    }
}
