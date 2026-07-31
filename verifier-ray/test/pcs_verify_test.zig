const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const pcs = verifier_ray.pcs.root;
const params_mod = pcs.params;
const layout_mod = pcs.layout;
const tree = pcs.tree;
const pl = pcs.paired_leaf;
const verify = pcs.verify;

fn le(v: u32) field.Element {
    return field.Element.init(v);
}
fn e(v: u32) ext.Ext {
    return ext.Ext.fromUints(.{ v, v +% 1, v +% 2, v +% 3, v +% 4, v +% 5 });
}

// ── D=1 end-to-end verify ────────────────────────────────────────────────────
//
// The minimal proof that exercises input-tree authentication, level (round-0)
// DEEP-quotient reconstruction, claim-point computation, and the numRounds==0
// "final polynomial IS the constant codeword" tie — with NO folding.
//
// Construction, derived from first principles (NOT from the verifier's own
// reconstruct/fold helpers, to avoid a tautological test):
//   - Config: log_codeword=1, log_plaintext=0, log_final=0 ⇒ numRounds=0.
//   - One batch, one size-2^0 table, one base row. Its plaintext is a CONSTANT
//     c, so its RS codeword over the size-2 domain is [c, c].
//   - Open at shift 0 ⇒ claim point = zeta, claimed value y = f(zeta) = c.
//   - DEEP quotient (f(x) - y)/(x - zeta) = (c - c)/… = 0 at every codeword
//     position ⇒ the reconstructed round-0 pair is {0, 0}.
//   - Hence final_poly = [0] (size 2^0), and both the D=1 self/sibling ties hold
//     because the constant final poly evaluates to 0 everywhere.

const D1System = verify.System{
    .params = .{
        .log_codeword_size = 1,
        .log_plaintext_size = 0,
        .num_queries = 1,
        .log_final_poly_size = 0,
    },
    .layout = buildD1Layout(),
    .shapes = &d1_shapes,
    .num_batches = 1,
};

const d1_shapes = [_]layout_mod.Shape{
    &.{.{ .base_width = 1, .ext_width = 0 }}, // batch 0, size 2^0: 1 base row
};
const d1_shifts = [_]layout_mod.BatchShifts{
    &.{.{ .base = &.{&.{0}}, .ext = &.{} }}, // open base row 0 at shift 0
};

fn buildD1Layout() layout_mod.Layout {
    return comptime layout_mod.buildLayout(&d1_shapes, &d1_shifts) catch unreachable;
}

// Bottom-leaf digest for the constant codeword cell `c` (base width 1, ext 0).
fn d1Leaf(c: field.Element) tree.Octuplet {
    const row = pl.RowOpening{ .base = &.{c}, .ext = &.{} };
    return pl.hashRowOpening(row);
}

// Regression for the D=1 (numRounds==0) Fiat-Shamir schedule. prover-ray's
// `pcs.go verify` squeezes the final fold challenge UNCONDITIONALLY (pcs.go:331),
// even when there are no round roots, so for D=1 the query positions are drawn
// from a transcript that has absorbed: [one discarded squeeze] → final_poly.
// This asserts `deriveChallenges` reproduces that exact sequence: the derived
// position must match a manual replay that performs the extra squeeze. Without
// the unconditional squeeze the two would diverge, silently desynchronising the
// verifier-ray positions from the reference.
test "verify: D=1 challenge schedule squeezes the final alpha (matches prover-ray)" {
    const fiat_shamir = verifier_ray.crypto.fiat_shamir;

    const final_poly = [_]ext.Ext{ext.Ext.zero()};
    const proof = verify.Proof{
        .input_queries = &.{},
        .fri = .{
            .round_roots = &.{}, // D=1: no fold rounds
            .final_poly = &final_poly,
            .running_queries = &.{},
        },
    };

    // What deriveChallenges (via replayWithTranscript) produces.
    var t_actual = fiat_shamir.Transcript.init();
    const coins = verify.replayWithTranscript(D1System, &t_actual, proof);

    // Manual reference: the prover-ray D=1 sequence — one unconditional final
    // squeeze (discarded, since there is no fold), then absorb final_poly, then
    // draw the position(s) in [0, codeword_size).
    var t_ref = fiat_shamir.Transcript.init();
    _ = t_ref.randomExt(); // pcs.go:331 — squeezed even with zero round roots
    t_ref.updateExt(&final_poly);
    const codeword_size: usize = @as(usize, 1) << D1System.params.log_codeword_size;
    var want: [1]usize = undefined;
    t_ref.randomManyIntegers(&want, codeword_size);

    try std.testing.expectEqual(want[0], coins.positions[0]);
}

test "verify: accepts a valid D=1 proof, rejects mutations" {
    const c = le(5);
    const zeta = e(777);

    // Merkle root over the constant codeword [c, c].
    const leaf = d1Leaf(c);
    const root = tree.hashNode(leaf, leaf, null);

    // Row openings: both rows carry the constant c.
    const row = pl.RowOpening{ .base = &.{c}, .ext = &.{} };
    // Bottom pair for query s=0: (self=row0, sib=row1); both are `row`.
    const bottom_pair = pl.RowPair{ row, row };
    const opening = pl.InputTreeOpening{
        .siblings = &.{}, // 2-leaf tree: bottom sibling derived from the pair
        .leaves = &.{bottom_pair},
    };

    const input_queries = [_][]const pl.InputTreeOpening{
        &.{opening}, // query 0, batch 0
    };

    // Claimed value: y = f(zeta) = c (constant polynomial).
    const y = ext.Ext.lift(c);
    const entry_claims = [_][]const ext.Ext{
        &.{y}, // column 0 (base row 0), one shift
    };

    // numRounds==0 ⇒ final_poly = [0].
    const final_poly = [_]ext.Ext{ext.Ext.zero()};

    const proof = verify.Proof{
        .input_queries = &input_queries,
        .fri = .{
            .round_roots = &.{},
            .final_poly = &final_poly,
            .running_queries = &.{&.{}}, // one query, zero running layers
        },
    };

    const inputs = verify.Inputs{
        .roots = &.{root},
        .entry_claims = &entry_claims,
        .zeta = zeta,
    };

    // Fold challenges and the query position are derived from the transcript
    // (num_rounds==0 ⇒ no fold alphas; one position squeezed in [0, 2)). The
    // codeword [c, c] and the hashNode(leaf, leaf) root are position-symmetric,
    // so the honest proof verifies whichever of {0, 1} is derived. A fresh
    // transcript stands in for the (empty) protocol prefix of this unit fixture.
    {
        var t = verifier_ray.crypto.fiat_shamir.Transcript.init();
        const coins = verify.replayWithTranscript(D1System, &t, proof);
        try verify.verify(D1System, inputs, proof, coins);
    }

    // ── Mutations must be rejected ────────────────────────────────────────────

    // 1. Wrong Merkle root.
    {
        var bad = inputs;
        bad.roots = &.{tree.hashNode(leaf, d1Leaf(le(6)), null)};
        var t = verifier_ray.crypto.fiat_shamir.Transcript.init();
        const coins = verify.replayWithTranscript(D1System, &t, proof);
        try std.testing.expectError(verify.Error.InputTreeAuthFailed, verify.verify(D1System, bad, proof, coins));
    }

    // 2. Tampered claimed value ⇒ DEEP quotient no longer zero ⇒ D=1 tie fails.
    {
        const bad_claims = [_][]const ext.Ext{&.{y.add(ext.Ext.one())}};
        var bad = inputs;
        bad.entry_claims = &bad_claims;
        var t = verifier_ray.crypto.fiat_shamir.Transcript.init();
        const coins = verify.replayWithTranscript(D1System, &t, proof);
        try std.testing.expectError(verify.Error.RoundZeroSelfMismatch, verify.verify(D1System, bad, proof, coins));
    }

    // 3. Non-zero final poly ⇒ tie fails.
    {
        const bad_final = [_]ext.Ext{ext.Ext.one()};
        var bad_proof = proof;
        bad_proof.fri.final_poly = &bad_final;
        var t = verifier_ray.crypto.fiat_shamir.Transcript.init();
        const coins = verify.replayWithTranscript(D1System, &t, bad_proof);
        try std.testing.expectError(verify.Error.RoundZeroSelfMismatch, verify.verify(D1System, inputs, bad_proof, coins));
    }

    // 4. zeta inside the codeword domain ⇒ rejected up front.
    {
        // The size-2^0 codeword domain has cardinality 2; a base zeta with
        // zeta^2 == 1 lands in it. zeta = 1 works (1^2 == 1).
        var bad = inputs;
        bad.zeta = ext.Ext.one();
        var t = verifier_ray.crypto.fiat_shamir.Transcript.init();
        const coins = verify.replayWithTranscript(D1System, &t, proof);
        try std.testing.expectError(verify.Error.ClaimPointInDomain, verify.verify(D1System, bad, proof, coins));
    }
}
