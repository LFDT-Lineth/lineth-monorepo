const std = @import("std");
const verifier_ray = @import("verifier_ray");

const protocol = verifier_ray.protocol;
const logderivativesum = verifier_ray.query.logderivativesum;
const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;

// The golden fixtures already cover honest end-to-end proofs across every
// LogDerivativeSumCompiler/Lookup scenario; these hand-built cases pin the
// boundary checks (and their error paths) directly using ScalarRef lookups into
// a runtime ctx, so an adversary cannot bypass them by altering proof cells.

// makeCtx builds a single-round Context whose cells slice is the given slice.
fn makeCtx(comptime cells: []const protocol.Scalar) protocol.Context {
    const rounds: []const protocol.RoundMessage = &[_]protocol.RoundMessage{
        .{ .cells = cells },
    };
    return .{ .all_coins = &.{}, .rounds = rounds };
}

// oneQuerySystem returns a System with one query whose single Z column has its
// average cell at (round=0, index=0), on a static module of size n, and whose
// result is at (round=0, index=1).
fn oneQuerySystem(comptime result_is_zero: bool, comptime n: usize) logderivativesum.System {
    return .{ .queries = &[_]logderivativesum.Query{.{
        .entries = &[_]logderivativesum.Entry{
            .{ .average_ref = .{ .round = 0, .index = 0 }, .size = .{ .static = n } },
        },
        .result_ref = .{ .round = 0, .index = 1 },
        .result_is_zero = result_is_zero,
    }} };
}

fn baseScalar(v: u32) protocol.Scalar {
    return .{ .base = field.Element.init(v) };
}

const zero = baseScalar(0);
const three = baseScalar(3);
const four = baseScalar(4);
const five = baseScalar(5);
const twenty = baseScalar(20);
const twenty_two = baseScalar(22);
const twenty_four = baseScalar(24);

// twoEntrySystem returns a System with one query that has two Z columns
// (average cells at indices 0 and 1, on static modules of sizes 2 and 4)
// summed against result at index 2.
fn twoEntrySystem() logderivativesum.System {
    return .{ .queries = &[_]logderivativesum.Query{.{
        .entries = &[_]logderivativesum.Entry{
            .{ .average_ref = .{ .round = 0, .index = 0 }, .size = .{ .static = 2 } },
            .{ .average_ref = .{ .round = 0, .index = 1 }, .size = .{ .static = 4 } },
        },
        .result_ref = .{ .round = 0, .index = 2 },
        .result_is_zero = false,
    }} };
}

// dynamicSystem returns a System whose single Z column lives on the dynamic
// module of size index 0.
fn dynamicSystem() logderivativesum.System {
    return .{ .queries = &[_]logderivativesum.Query{.{
        .entries = &[_]logderivativesum.Entry{
            .{ .average_ref = .{ .round = 0, .index = 0 }, .size = .{ .dynamic = 0 } },
        },
        .result_ref = .{ .round = 0, .index = 1 },
        .result_is_zero = false,
    }} };
}

test "logderiv accepts a matching column total n·C" {
    // n = 4, C = 5, result = 20.
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ five, twenty };
    try logderivativesum.verify(oneQuerySystem(false, 4), makeCtx(cells), &.{});
}

test "logderiv rejects a result equal to C instead of n·C" {
    // The average is not the total: with n = 4, C = 5 claims a total of 20.
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ five, five };
    try std.testing.expectError(
        error.FinalSumMismatch,
        logderivativesum.verify(oneQuerySystem(false, 4), makeCtx(cells), &.{}),
    );
}

test "lookup rejects a non-zero aggregated result" {
    // Final-sum holds (1·3 == 3) so the FinalSumMismatch guard passes; the
    // result-is-zero guard must then reject the non-zero Result.
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, three };
    try std.testing.expectError(
        error.LookupResultNonZero,
        logderivativesum.verify(oneQuerySystem(true, 1), makeCtx(cells), &.{}),
    );
}

test "lookup accepts a zero aggregated result" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ zero, zero };
    try logderivativesum.verify(oneQuerySystem(true, 4), makeCtx(cells), &.{});
}

test "logderiv accepts multiple column totals that sum to result" {
    // 2·3 + 4·4 = 22
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, four, twenty_two };
    try logderivativesum.verify(twoEntrySystem(), makeCtx(cells), &.{});
}

test "logderiv rejects multiple column totals whose sum disagrees with result" {
    // 2·3 + 4·5 = 26 != 22
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, five, twenty_two };
    try std.testing.expectError(
        error.FinalSumMismatch,
        logderivativesum.verify(twoEntrySystem(), makeCtx(cells), &.{}),
    );
}

test "logderiv scales a dynamic column by its proof-supplied size" {
    // n = module_sizes[0] = 8, C = 3, result = 24.
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, twenty_four };
    try logderivativesum.verify(dynamicSystem(), makeCtx(cells), &[_]usize{8});
    // The same cells with another size disagree.
    try std.testing.expectError(
        error.FinalSumMismatch,
        logderivativesum.verify(dynamicSystem(), makeCtx(cells), &[_]usize{4}),
    );
}

test "logderiv rejects a missing dynamic module size" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, twenty_four };
    try std.testing.expectError(
        error.MissingDynamicModuleSize,
        logderivativesum.verify(dynamicSystem(), makeCtx(cells), &.{}),
    );
}
