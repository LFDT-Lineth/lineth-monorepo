const std = @import("std");
const verifier_ray = @import("verifier_ray");

const protocol = verifier_ray.protocol;
const grandproduct = verifier_ray.query.grandproduct;
const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;

// Hand-built cases pinning the boundary checks (and their error paths)
// directly via ScalarRef lookups into a runtime ctx, mirroring
// logderivativesum_test.zig — so an adversary cannot bypass them by altering
// proof cells.

// makeCtx builds a single-round Context whose cells slice is the given slice.
fn makeCtx(comptime cells: []const protocol.Scalar) protocol.Context {
    const rounds: []const protocol.RoundMessage = &[_]protocol.RoundMessage{
        .{ .cells = cells },
    };
    return .{ .all_coins = &.{}, .rounds = rounds };
}

fn baseScalar(v: u32) protocol.Scalar {
    return .{ .base = field.Element.init(v) };
}

const zero = baseScalar(0);
const one = baseScalar(1);
const three = baseScalar(3);
const four = baseScalar(4);
const seven = baseScalar(7);

// oneQuerySystem returns a System with one query whose z_final and result are
// at (round=0, index=0) and (round=0, index=1) respectively, expecting
// Result == expected.
fn oneQuerySystem(comptime expected: ?u64) grandproduct.System {
    return .{ .queries = &[_]grandproduct.Query{.{
        .z_final_refs = &[_]grandproduct.ScalarRef{.{ .round = 0, .index = 0 }},
        .result_ref = .{ .round = 0, .index = 1 },
        .expected = expected,
    }} };
}

// twoRefSystem returns a System with one query that has two z_final_refs
// (indices 0 and 1) multiplied against result at index 2, expecting Result == 1
// (the permutation / default message-bus shape).
fn twoRefSystem() grandproduct.System {
    return .{ .queries = &[_]grandproduct.Query{.{
        .z_final_refs = &[_]grandproduct.ScalarRef{
            .{ .round = 0, .index = 0 },
            .{ .round = 0, .index = 1 },
        },
        .result_ref = .{ .round = 0, .index = 2 },
        .expected = 1,
    }} };
}

test "grandproduct accepts a matching final product with no expected-result check" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ four, four };
    try grandproduct.verify(oneQuerySystem(null), makeCtx(cells));
}

test "grandproduct rejects a final product that disagrees with Result" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ four, seven };
    try std.testing.expectError(
        error.FinalProductMismatch,
        grandproduct.verify(oneQuerySystem(null), makeCtx(cells)),
    );
}

test "permutation-shaped query accepts Result == 1" {
    // z_final = 1, result = 1 (1*1 == 1, and 1 == expected).
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ one, one };
    try grandproduct.verify(oneQuerySystem(1), makeCtx(cells));
}

test "permutation-shaped query rejects Result != 1 even when final product matches" {
    // z_final = 3, result = 3: FinalProductMismatch would NOT fire (3 == 3),
    // but ResultMismatch must, since 3 != 1.
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, three };
    try std.testing.expectError(
        error.ResultMismatch,
        grandproduct.verify(oneQuerySystem(1), makeCtx(cells)),
    );
}

test "message-bus-shaped query rejects a zero result when expecting one" {
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ zero, zero };
    try std.testing.expectError(
        error.ResultMismatch,
        grandproduct.verify(oneQuerySystem(1), makeCtx(cells)),
    );
}

test "grandproduct accepts multiple z_final_refs whose product matches result" {
    // z_final[0]=1, z_final[1]=1, result=1 (1*1==1, and 1==expected).
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ one, one, one };
    try grandproduct.verify(twoRefSystem(), makeCtx(cells));
}

test "grandproduct rejects multiple z_final_refs whose product disagrees with result" {
    // z_final[0]=3, z_final[1]=4, result=7 (3*4==12 != 7).
    const cells: []const protocol.Scalar = &[_]protocol.Scalar{ three, four, seven };
    try std.testing.expectError(
        error.FinalProductMismatch,
        grandproduct.verify(twoRefSystem(), makeCtx(cells)),
    );
}

test "empty query system verifies trivially" {
    const empty_system = grandproduct.System{};
    try grandproduct.verify(empty_system, makeCtx(&[_]protocol.Scalar{}));
}

test "ext.lift(1) matches ext.one() for the expected-result comparison" {
    // Sanity check that the sub-verifier's `ext.Ext.lift(base.Element.init(e))`
    // path used for `expected` produces the same value as ext.Ext.one() — the
    // honest-prover Result value for a permutation / unsharded message-bus.
    try std.testing.expect(ext.Ext.lift(field.Element.init(1)).eql(ext.Ext.one()));
}
