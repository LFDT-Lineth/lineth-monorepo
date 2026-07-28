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
    proof.claims.inputs.entry_claims = &claims;

    try std.testing.expectError(error.FoldMismatch, verifier.verify(co.spec, co.systems, proof));
}

// Note: there is no "reject a direct-claims proof" test. A proof cannot carry
// raw vanishing claims instead of a PCS opening — `Proof.claims` is a plain
// `PcsClaims`, not a union, so that shape is a compile error, not a runtime
// rejection.
