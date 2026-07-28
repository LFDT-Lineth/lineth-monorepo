const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const params = verifier_ray.pcs.params;
const tree = verifier_ray.pcs.tree;

// ── ext.halve ────────────────────────────────────────────────────────────────

test "ext.halve: lift(2).halve() == lift(1)" {
    const two = ext.Ext.lift(field.Element.init(2));
    try std.testing.expect(two.halve().eql(ext.Ext.one()));
}

test "ext.halve: x.halve() doubled recovers x for a non-base element" {
    const x = ext.Ext.fromUints(.{ 7, 11, 13, 17, 19, 23 });
    const half = x.halve();
    try std.testing.expect(half.add(half).eql(x));
}

// ── params.domainPoint ───────────────────────────────────────────────────────

test "domainPoint: log_size 0 is always one" {
    try std.testing.expect((try params.domainPoint(0, 0)).eql(field.Element.one()));
    try std.testing.expect((try params.domainPoint(0, 5)).eql(field.Element.one()));
}

test "domainPoint: size-2 domain gives {1, -1}" {
    // The size-2 domain generator is the order-2 root of unity = -1.
    const p0 = try params.domainPoint(1, 0);
    const p1 = try params.domainPoint(1, 1);
    try std.testing.expect(p0.eql(field.Element.one()));
    try std.testing.expect(p1.eql(field.Element.one().neg()));
}

test "domainPoint: every size-4 point is a distinct 4th root of unity" {
    const n = 4;
    const g = try field.rootOfUnityBy(n);
    // g has order exactly 4: g^4 == 1 and g^2 != 1.
    try std.testing.expect(g.pow(4).eql(field.Element.one()));
    try std.testing.expect(!g.pow(2).eql(field.Element.one()));

    var seen = [_]bool{false} ** n;
    for (0..n) |pos| {
        const pt = try params.domainPoint(2, pos);
        // pt must be some power of g in [0,4); locate it and mark it seen.
        var found = false;
        for (0..n) |k| {
            if (pt.eql(g.pow(@as(u64, k)))) {
                try std.testing.expect(!seen[k]); // no duplicates
                seen[k] = true;
                found = true;
                break;
            }
        }
        try std.testing.expect(found);
    }
    for (seen) |s| try std.testing.expect(s);
}

test "bitReversedExponent: matches a hand computation for log_size 3" {
    // pos=1 (001) reversed in 3 bits is 100 = 4.
    try std.testing.expectEqual(@as(u64, 4), params.bitReversedExponent(1, 3));
    // pos=3 (011) reversed in 3 bits is 110 = 6.
    try std.testing.expectEqual(@as(u64, 6), params.bitReversedExponent(3, 3));
    // pos=0 reverses to 0.
    try std.testing.expectEqual(@as(u64, 0), params.bitReversedExponent(0, 3));
}

// ── params.restrictTo / numRounds ────────────────────────────────────────────

test "params.numRounds and restrictTo" {
    const p = params.Params{
        .log_codeword_size = 10,
        .log_plaintext_size = 8,
        .num_queries = 4,
        .log_final_poly_size = 1,
    };
    try p.validate();
    try std.testing.expectEqual(@as(u8, 7), p.numRounds()); // 8 - 1

    const r = try p.restrictTo(5);
    // codeword shrinks by the same offset (8 - 5 = 3): 10 - 3 = 7.
    try std.testing.expectEqual(@as(u8, 7), r.log_codeword_size);
    try std.testing.expectEqual(@as(u8, 5), r.log_plaintext_size);
    try std.testing.expectEqual(@as(u8, 1), r.log_final_poly_size);
    try std.testing.expectEqual(@as(u8, 4), r.numRounds()); // 5 - 1
    // inverse rate (blow-up) preserved.
    try std.testing.expectEqual(
        p.log_codeword_size - p.log_plaintext_size,
        r.log_codeword_size - r.log_plaintext_size,
    );

    try std.testing.expectError(params.Error.RestrictOutOfRange, p.restrictTo(9));
    try std.testing.expectError(params.Error.RestrictOutOfRange, p.restrictTo(0));
}

test "params.validate rejects bad shapes" {
    try std.testing.expectError(
        params.Error.PlaintextNotSmallerThanCodeword,
        (params.Params{ .log_codeword_size = 8, .log_plaintext_size = 8, .num_queries = 1 }).validate(),
    );
    try std.testing.expectError(
        params.Error.ZeroQueries,
        (params.Params{ .log_codeword_size = 8, .log_plaintext_size = 4, .num_queries = 0 }).validate(),
    );
    try std.testing.expectError(
        params.Error.CodewordSizeTooLarge,
        (params.Params{ .log_codeword_size = 25, .log_plaintext_size = 4, .num_queries = 1 }).validate(),
    );
}

// ── tree.hashNode / Branch.recoverRoot ───────────────────────────────────────

fn leaf(v: u32) tree.Octuplet {
    var out: tree.Octuplet = undefined;
    for (&out, 0..) |*e, i| e.* = field.Element.init(v + @as(u32, @intCast(i)));
    return out;
}

fn octEql(a: tree.Octuplet, b: tree.Octuplet) bool {
    for (a, b) |x, y| {
        if (!x.eql(y)) return false;
    }
    return true;
}

test "recoverRoot: 2-leaf tree, no aux, both positions" {
    const l0 = leaf(10);
    const l1 = leaf(20);
    const root = tree.hashNode(l0, l1, null);

    // Leaf 0 (left child): sibling is l1.
    const b0 = tree.Branch{
        .leaf = l0,
        .siblings = &.{l1},
        .aux_siblings = &.{null},
    };
    try std.testing.expect(octEql(try b0.recoverRoot(0), root));

    // Leaf 1 (right child): sibling is l0.
    const b1 = tree.Branch{
        .leaf = l1,
        .siblings = &.{l0},
        .aux_siblings = &.{null},
    };
    try std.testing.expect(octEql(try b1.recoverRoot(1), root));
}

test "recoverRoot: 4-leaf tree, no aux, every position" {
    const ls = [_]tree.Octuplet{ leaf(1), leaf(2), leaf(3), leaf(4) };
    const n01 = tree.hashNode(ls[0], ls[1], null);
    const n23 = tree.hashNode(ls[2], ls[3], null);
    const root = tree.hashNode(n01, n23, null);

    // siblings run top-down: [uncle just below root, immediate sibling].
    const branches = [_]tree.Branch{
        .{ .leaf = ls[0], .siblings = &.{ n23, ls[1] }, .aux_siblings = &.{ null, null } },
        .{ .leaf = ls[1], .siblings = &.{ n23, ls[0] }, .aux_siblings = &.{ null, null } },
        .{ .leaf = ls[2], .siblings = &.{ n01, ls[3] }, .aux_siblings = &.{ null, null } },
        .{ .leaf = ls[3], .siblings = &.{ n01, ls[2] }, .aux_siblings = &.{ null, null } },
    };
    for (branches, 0..) |b, idx| {
        try std.testing.expect(octEql(try b.recoverRoot(idx), root));
    }
}

test "recoverRoot: aux leaf is hashed in" {
    const l0 = leaf(5);
    const l1 = leaf(6);
    const aux = leaf(99);
    const root = tree.hashNode(l0, l1, aux);

    const b0 = tree.Branch{
        .leaf = l0,
        .siblings = &.{l1},
        .aux_siblings = &.{aux},
    };
    try std.testing.expect(octEql(try b0.recoverRoot(0), root));

    // Wrong aux (or none) must not reproduce the root.
    const b_noaux = tree.Branch{
        .leaf = l0,
        .siblings = &.{l1},
        .aux_siblings = &.{null},
    };
    try std.testing.expect(!octEql(try b_noaux.recoverRoot(0), root));
}

test "recoverRoot: errors on malformed and empty branches" {
    const l0 = leaf(1);
    try std.testing.expectError(tree.Error.MalformedProof, (tree.Branch{
        .leaf = l0,
        .siblings = &.{ leaf(2), leaf(3) },
        .aux_siblings = &.{null},
    }).recoverRoot(0));
    try std.testing.expectError(tree.Error.EmptyProof, (tree.Branch{
        .leaf = l0,
        .siblings = &.{},
        .aux_siblings = &.{},
    }).recoverRoot(0));
}
