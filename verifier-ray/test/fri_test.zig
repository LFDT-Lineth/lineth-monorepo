const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;
const merkle = verifier_ray.crypto.merkle;
const fri = verifier_ray.query.fri;

// ─── crypto.merkle ───────────────────────────────────────────────────────────

test "merkle branch recovers the root of a hand-built two-leaf tree" {
    const leaf0 = poseidon2.hashElements(&.{field.Element.init(1)});
    const leaf1 = poseidon2.hashElements(&.{field.Element.init(2)});
    const root = merkle.hashNode(leaf0, leaf1, null);

    // Opening the even leaf: no swap at the only level.
    const branch0 = merkle.Branch{ .leaf = leaf0, .siblings = &.{leaf1} };
    const recovered0 = try branch0.recoverRoot(0);
    try std.testing.expect(std.meta.eql(recovered0, root));

    // Opening the odd leaf: the parity swap must land the same root.
    const branch1 = merkle.Branch{ .leaf = leaf1, .siblings = &.{leaf0} };
    const recovered1 = try branch1.recoverRoot(1);
    try std.testing.expect(std.meta.eql(recovered1, root));
}

test "merkle branch rejects a wrong sibling" {
    const leaf0 = poseidon2.hashElements(&.{field.Element.init(1)});
    const leaf1 = poseidon2.hashElements(&.{field.Element.init(2)});
    const wrong_sibling = poseidon2.hashElements(&.{field.Element.init(3)});
    const root = merkle.hashNode(leaf0, leaf1, null);

    const branch = merkle.Branch{ .leaf = leaf0, .siblings = &.{wrong_sibling} };
    const recovered = try branch.recoverRoot(0);
    try std.testing.expect(!std.meta.eql(recovered, root));
}

test "merkle branch with no siblings is rejected" {
    const leaf = poseidon2.hashElements(&.{field.Element.init(1)});
    const branch = merkle.Branch{ .leaf = leaf, .siblings = &.{} };
    try std.testing.expectError(error.EmptyBranch, branch.recoverRoot(0));
}

// ─── query.fri: a single fold round, no Merkle tree involved ───────────────
//
// log_codeword_size = 1 (domain size 2), num_rounds = 1: round 0 is both the
// first and the last round, so its level (aux[0], always present per
// ResolvedQuery's contract) folds straight into the final polynomial. There
// is no running layer to authenticate (num_rounds - 1 == 0), so this case
// exercises checkFolds' arithmetic in isolation.
//
// x = domainPoint(log_size=1, position=0) = generator^bitrev_1(0) = generator^0 = 1,
// so 1/x = 1 too, and the fold reduces to plain field arithmetic:
//   sum  = self + sib           = 3 + 5   = 8
//   diff = (self - sib) * 1 * a = (3 - 5) * 7 = -14
//   expected = (sum + diff) / 2 = (8 - 14) / 2 = -3

const one_round_params: fri.Params = .{ .log_codeword_size = 1, .num_rounds = 1, .log_final_poly_size = 0, .num_queries = 1 };

test "checkFolds accepts a single honest fold round" {
    const params = one_round_params;
    const aux_pair = fri.Pair{ .self = ext.Ext.lift(field.Element.init(3)), .sibling = ext.Ext.lift(field.Element.init(5)) };
    const alpha = ext.Ext.lift(field.Element.init(7));
    const final = ext.Ext.lift(field.Element.init(field.modulus - 3));

    const resolved = [_]fri.ResolvedQuery{.{
        .rounds = &.{.{ .self = ext.Ext.zero(), .sibling = ext.Ext.zero() }},
        .aux = &.{aux_pair},
        .final = final,
    }};

    try fri.checkFolds(params, &resolved, &.{alpha}, &.{0});
}

test "checkFolds rejects a mismatched final polynomial" {
    const params = one_round_params;
    const aux_pair = fri.Pair{ .self = ext.Ext.lift(field.Element.init(3)), .sibling = ext.Ext.lift(field.Element.init(5)) };
    const alpha = ext.Ext.lift(field.Element.init(7));
    // Off by one from the honest value derived above.
    const wrong_final = ext.Ext.lift(field.Element.init(field.modulus - 2));

    const resolved = [_]fri.ResolvedQuery{.{
        .rounds = &.{.{ .self = ext.Ext.zero(), .sibling = ext.Ext.zero() }},
        .aux = &.{aux_pair},
        .final = wrong_final,
    }};

    try std.testing.expectError(
        error.FinalPolyMismatch,
        fri.checkFolds(params, &resolved, &.{alpha}, &.{0}),
    );
}

// ─── query.fri: two rounds, exercising resolveRunningLayers ─────────────────
//
// log_codeword_size = 2 (domain size 4), num_rounds = 2: round 0 introduces
// the top level and folds into round 1's *running* codeword (a real,
// hand-built two-leaf Merkle tree); round 1 has no level of its own and folds
// straight into the final polynomial.
//
// Query position s = 0 throughout, so every domain point this test needs is
// x = generator^bitrev(0) = 1 and its square is again 1 -- the fold point
// never leaves 1, keeping the arithmetic by-hand-checkable:
//   round 0: sum = 3+5 = 8, diff = (3-5)*1*7 = -14, folded = (8-14)/2 = -3
//            -> this is round 1's running "self" value
//   round 1: sum = -3+11 = 8, diff = (-3-11)*1*13 = -182, folded = (8-182)/2 = -87

const two_round_params: fri.Params = .{ .log_codeword_size = 2, .num_rounds = 2, .log_final_poly_size = 0, .num_queries = 1 };

fn extToOctuplet(value: ext.Ext) poseidon2.Digest {
    return .{
        value.B0.a0,          value.B0.a1,
        value.B1.a0,          value.B1.a1,
        value.B2.a0,          value.B2.a1,
        field.Element.zero(), field.Element.zero(),
    };
}

const TwoRoundFixture = struct {
    proof: fri.Proof,
    fold_alphas: [2]ext.Ext,
    positions: [1]usize,
    round1_self: ext.Ext, // = the honest round-0 fold result, redundant with proof but handy for assertions
};

// Built once, at comptime (forced by the top-level `const` below): the nested
// slices this fixture returns (round_roots, running_queries, and each
// branch's siblings) would otherwise point into buildTwoRoundFixture's stack
// frame and dangle the moment it returns. A comptime-evaluated container-level
// const gets its backing data placed in static storage instead.
fn buildTwoRoundFixture() TwoRoundFixture {
    // Poseidon2's permutation loop trips the default 1000-backwards-branch
    // comptime budget; this fixture is forced to comptime (see
    // two_round_fixture below) so it needs a larger one.
    @setEvalBranchQuota(1_000_000);
    const round1_self = ext.Ext.lift(field.Element.init(field.modulus - 3)); // -3, see derivation above
    const round1_sibling = ext.Ext.lift(field.Element.init(11));

    const leaf0 = extToOctuplet(round1_self);
    const leaf1 = extToOctuplet(round1_sibling);
    const root1 = merkle.hashNode(leaf0, leaf1, null);

    return .{
        .proof = .{
            .round_roots = &.{root1},
            .final_poly = &.{ext.Ext.lift(field.Element.init(field.modulus - 87))}, // -87, see derivation above
            .running_queries = &.{&.{.{ .leaf = leaf0, .siblings = &.{leaf1} }}},
        },
        .fold_alphas = .{ ext.Ext.lift(field.Element.init(7)), ext.Ext.lift(field.Element.init(13)) },
        .positions = .{0},
        .round1_self = round1_self,
    };
}

const two_round_fixture: TwoRoundFixture = buildTwoRoundFixture();

test "checkOpeningProofShape + resolveRunningLayers + checkFolds accept an honest two-round proof" {
    const params = two_round_params;
    const fixture = two_round_fixture;

    try fri.checkOpeningProofShape(params, fixture.proof, &fixture.fold_alphas, &fixture.positions);

    var rounds: [2]fri.Pair = undefined;
    try fri.resolveRunningLayers(params, fixture.proof.round_roots, fixture.proof.running_queries[0], fixture.positions[0], &rounds);
    try std.testing.expect(rounds[1].self.eql(fixture.round1_self));
    try std.testing.expect(rounds[1].sibling.eql(ext.Ext.lift(field.Element.init(11))));

    const aux0 = fri.Pair{ .self = ext.Ext.lift(field.Element.init(3)), .sibling = ext.Ext.lift(field.Element.init(5)) };
    const resolved = [_]fri.ResolvedQuery{.{
        .rounds = &rounds,
        .aux = &.{ aux0, null },
        .final = fixture.proof.final_poly[0],
    }};

    try fri.checkFolds(params, &resolved, &fixture.fold_alphas, &fixture.positions);
}

test "resolveRunningLayers rejects a running root that does not match the branch" {
    const params = two_round_params;
    const fixture = two_round_fixture;
    const wrong_roots = [_]poseidon2.Digest{poseidon2.hashElements(&.{field.Element.init(999)})};

    var rounds: [2]fri.Pair = undefined;
    try std.testing.expectError(
        error.MerkleProofInvalid,
        fri.resolveRunningLayers(params, &wrong_roots, fixture.proof.running_queries[0], fixture.positions[0], &rounds),
    );
}

test "checkFolds rejects a level pair that disagrees with the resolved running layer" {
    const params = two_round_params;
    const fixture = two_round_fixture;

    var rounds: [2]fri.Pair = undefined;
    try fri.resolveRunningLayers(params, fixture.proof.round_roots, fixture.proof.running_queries[0], fixture.positions[0], &rounds);

    // A level pair that does not fold to rounds[1].self (honestly {3, 5}).
    const wrong_aux0 = fri.Pair{ .self = ext.Ext.lift(field.Element.init(4)), .sibling = ext.Ext.lift(field.Element.init(5)) };
    const resolved = [_]fri.ResolvedQuery{.{
        .rounds = &rounds,
        .aux = &.{ wrong_aux0, null },
        .final = fixture.proof.final_poly[0],
    }};

    try std.testing.expectError(
        error.FoldMismatch,
        fri.checkFolds(params, &resolved, &fixture.fold_alphas, &fixture.positions),
    );
}
