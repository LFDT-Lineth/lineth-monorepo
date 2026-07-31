const std = @import("std");
const verifier_ray = @import("verifier_ray");
const co = @import("test_coexist");

const ext = verifier_ray.field.koalabear_ext;
const verifier = verifier_ray.verifier;

// End-to-end coexistence gate: a REAL prover-ray protocol that carries BOTH a
// global vanishing constraint AND a FRI/PCS commitment (testdata/generated/
// coexist.zig). Running the full `verifier.verify` exercises the whole pipeline
// against ONE shared Fiat-Shamir transcript:
//
//   1. replayWithTranscript absorbs every round and squeezes all coins (incl.
//      zeta), leaving the transcript positioned for the FRI opener.
//   2. pcs.verify authenticates `entry_claims` and derives its fold challenges +
//      query positions from that live transcript.
//   3. verifier.verify re-slices the PCS-authenticated `entry_claims` into the
//      vanishing witness/quotient claims via the System's claim maps, so the
//      vanishing quotient identity is checked against the SAME authenticated
//      values PCS just verified — no independently-supplied claim slice.
//
// This is the empirical proof of the link: `witness_claims`/`quotient_claims`
// are a re-slicing of `entry_claims`. The accept case shows they are consistent;
// the mutation case shows corrupting an authenticated claim is rejected.

test "coexist: verifier accepts the honest vanishing+PCS proof" {
    try verifier.verify(co.spec, co.systems, co.proof);
}

test "coexist: corrupting an authenticated entry_claim is rejected" {
    // Flip the witness column's claimed value. Because verifier.verify feeds the
    // PCS-authenticated entry_claims INTO the vanishing check (via witness_map),
    // and also authenticates them via FRI, the corruption cannot pass either
    // sub-verifier: whichever fires first, the proof is rejected. (Without the
    // link, a prover could satisfy PCS with the honest entry_claims while feeding
    // vanishing a doctored witness_claims slice — this test would then wrongly
    // accept.)
    var claims = [_][]const ext.Ext{undefined} ** co.entry_claims.len;
    inline for (co.entry_claims, 0..) |c, i| claims[i] = c;
    const bad_col = [_]ext.Ext{co.entry_claims[0][0].add(ext.Ext.one())};
    claims[0] = &bad_col;

    var proof = co.proof;
    proof.claims.entry_claims = &claims;

    try std.testing.expectError(error.FoldMismatch, verifier.verify(co.spec, co.systems, proof));
}

// Note: there is no "forged proof roots are ignored" runtime test. The proof
// (`PcsClaims`) carries NO `roots` field at all — the Merkle trust anchor is
// rebuilt by `verifier.verify` from the transcript-bound `System.batch_roots`.
// The old attack surface (open against a forged root while zeta stays bound to
// the honest commitment) is therefore unrepresentable: a proof has no field to
// forge, a compile-time property rather than a runtime check. The test below
// exercises the one remaining way to touch a batch root — the transcript-bound
// oracle commitment itself — and shows it is rejected.

test "coexist: corrupting a round oracle commitment is rejected" {
    // The batch root IS the round's oracle commitment (batch_roots[b] = .round b),
    // and that same commitment is absorbed to derive zeta. Flipping it therefore
    // (a) changes the transcript → different FRI challenges/positions, and
    // (b) changes the root PCS authenticates against. Either way the proof must
    // be rejected — the opening can no longer match a root bound to this zeta.
    const field = verifier_ray.field.koalabear;
    const protocol = verifier_ray.protocol;

    // Rebuild round 0's single oracle commitment with one coordinate flipped.
    const orig_root = switch (co.rounds[0].columns[0]) {
        .oracle_commitment => |c| c,
        else => unreachable,
    };
    var bad_root = orig_root;
    bad_root[0] = bad_root[0].add(field.Element.one());
    const bad_cols = [_]protocol.ColumnMessage{.{ .oracle_commitment = bad_root }};
    var bad_rounds = [_]protocol.RoundMessage{undefined} ** co.rounds.len;
    inline for (co.rounds, 0..) |r, i| bad_rounds[i] = r;
    bad_rounds[0].columns = &bad_cols;

    var proof = co.proof;
    proof.rounds = &bad_rounds;

    // Rejected (via the root/transcript binding — the exact error depends on
    // whether the FS divergence or the root mismatch fires first).
    try std.testing.expect(std.meta.isError(verifier.verify(co.spec, co.systems, proof)));
}

// Note: there is no "reject a direct-claims proof" test. A proof cannot carry
// raw vanishing claims instead of a PCS opening — `Proof.claims` is a plain
// `PcsClaims`, not a union, so that shape is a compile error, not a runtime
// rejection.
