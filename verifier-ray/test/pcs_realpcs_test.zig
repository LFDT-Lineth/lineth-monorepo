const std = @import("std");
const verifier_ray = @import("verifier_ray");
const rp = @import("test_realpcs");

const ext = verifier_ray.field.koalabear_ext;
const verifier = verifier_ray.verifier;

// PCS DEPLOYED ON A REAL PROTOCOL. Unlike coexist.zig (a minimal hand-built
// System), realpcs.zig is generated from a genuine prover-ray `wioptest`
// vanishing scenario ("BooleanColumn") run through the FULL arithmetization
// pipeline (rangecheck → lookup → logderiv → localvanishing → global) PLUS
// pcs.Compile. So `systems.pcs` is enabled with a real layout/shapes/claim-maps
// extracted from a real compiled IOP — this is the end-to-end proof that the
// PCS↔vanishing link works on a real protocol, not just a synthetic one.
//
// Running `verifier.verify` drives the same shared-transcript pipeline as
// coexist: replay → PCS authenticates entry_claims + derives FRI challenges →
// vanishing consumes the PCS-authenticated claims via the claim maps.

test "realpcs: verifier accepts the real-protocol vanishing+PCS proof" {
    try verifier.verify(rp.spec, rp.systems, rp.proof);
}

test "realpcs: corrupting an authenticated entry_claim is rejected" {
    // Flip an authenticated claimed value; it must fail either PCS (FRI fold)
    // or the vanishing identity, since both now read the same entry_claims.
    var claims = [_][]const ext.Ext{undefined} ** rp.entry_claims.len;
    inline for (rp.entry_claims, 0..) |c, i| claims[i] = c;
    const bad_col = [_]ext.Ext{rp.entry_claims[0][0].add(ext.Ext.one())};
    claims[0] = &bad_col;

    var proof = rp.proof;
    proof.claims.inputs.entry_claims = &claims;

    try std.testing.expectError(error.FoldMismatch, verifier.verify(rp.spec, rp.systems, proof));
}

// Note: there is no "reject a direct-claims proof" test. PCS claims are the only
// representable shape (`Proof.claims` is a plain `PcsClaims`, not a union), so a
// proof without a PCS opening is a compile error, not a runtime-rejected value.
