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
        .{ .columns = &.{}, .cells = cells },
    };
    return .{ .all_coins = &.{}, .rounds = rounds };
}

// oneQuerySystem returns a System with one query whose z_final and result are
// both at (round=0, index=0) and (round=0, index=1) respectively.
fn oneQuerySystem(comptime result_is_zero: bool) logderivativesum.System {
    return .{ .queries = &[_]logderivativesum.Query{.{
        .z_final_refs = &[_]logderivativesum.ScalarRef{.{ .round = 0, .index = 0 }},
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
const seven = baseScalar(7);
// twoRefSystem returns a System with one query that has two z_final_refs
// (indices 0 and 1) summed against result at index 2.
fn twoRefSystem() logderivativesum.System {
    return .{ .queries = &[_]logderivativesum.Query{.{
        .z_final_refs = &[_]logderivativesum.ScalarRef{
            .{ .round = 0, .index = 0 },
            .{ .round = 0, .index = 1 },
        },
        .result_ref = .{ .round = 0, .index = 2 },
        .result_is_zero = false,
    }} };
}

test "logderiv accepts a matching final sum" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ five, five };
    try logderivativesum.verify(oneQuerySystem(false), makeCtx(cells));
}

test "logderiv rejects a final sum that disagrees with Result" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ five, seven };
    try std.testing.expectError(
        error.FinalSumMismatch,
        logderivativesum.verify(oneQuerySystem(false), makeCtx(cells)),
    );
}

test "lookup rejects a non-zero aggregated result" {
    // Final-sum holds (3 == 3) so the FinalSumMismatch guard passes; the
    // result-is-zero guard must then reject the non-zero Result.
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, three };
    try std.testing.expectError(
        error.LookupResultNonZero,
        logderivativesum.verify(oneQuerySystem(true), makeCtx(cells)),
    );
}

test "lookup accepts a zero aggregated result" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ zero, zero };
    try logderivativesum.verify(oneQuerySystem(true), makeCtx(cells));
}

test "logderiv accepts multiple z_final_refs that sum to result" {
    // z_final[0]=3, z_final[1]=4, result=7 (3+4==7)
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, four, seven };
    try logderivativesum.verify(twoRefSystem(), makeCtx(cells));
}

test "logderiv rejects multiple z_final_refs whose sum disagrees with result" {
    // z_final[0]=3, z_final[1]=5, result=7 (3+5==8 != 7)
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, five, seven };
    try std.testing.expectError(
        error.FinalSumMismatch,
        logderivativesum.verify(twoRefSystem(), makeCtx(cells)),
    );
}
