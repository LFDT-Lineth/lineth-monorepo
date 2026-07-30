const std = @import("std");
const verifier_ray = @import("verifier_ray");
const frip = @import("test_frip");

const ext = verifier_ray.field.koalabear_ext;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;
const pcs = verifier_ray.pcs.root;
const verify = pcs.verify;

// Cross-check the verifier against a REAL prover-ray FRI/PCS opening proof
// (testdata/generated/frip.zig). This is the authoritative gate: every value
// in the fixture is produced by the Go prover, so acceptance here proves the
// Zig verifier agrees with prover-ray byte-for-byte (Merkle roots, DEEP
// quotients, fold recurrence, the final polynomial evaluation, AND — since the
// fold challenges and query positions are now derived from the Fiat-Shamir
// transcript rather than trusted — the transcript squeeze sequence itself).
//
// The fixture emits `fs_seed_state`: the transcript state right before the
// first fold challenge was squeezed. Seeding a fresh transcript with it makes
// verifier-ray re-derive the exact same fold_alphas and query_positions the Go
// prover used, so no challenge value is taken on trust.

fn seededTranscript() fiat_shamir.Transcript {
    var t = fiat_shamir.Transcript.init();
    t.setState(frip.fs_seed_state);
    return t;
}

fn buildInputs() verify.Inputs {
    return .{
        .roots = &frip.roots,
        .entry_claims = &frip.entry_claims,
        .zeta = frip.zeta,
    };
}

fn buildProof() verify.Proof {
    return .{
        .input_queries = frip.input_queries,
        .fri = .{
            .round_roots = &frip.round_roots,
            .final_poly = &frip.final_poly,
            .running_queries = frip.running_queries,
        },
    };
}

test "frip: verifier accepts the real prover-ray opening proof" {
    var t = seededTranscript();
    const proof = buildProof();
    const coins = verify.replayWithTranscript(frip.system, &t, proof);
    try verify.verify(frip.system, buildInputs(), proof, coins);
}

test "frip: a tampered claimed value is rejected" {
    // Flip the first opened column's claimed value; the DEEP quotient no longer
    // matches, so the fold recurrence must fail somewhere downstream.
    var claims = [_][]const ext.Ext{undefined} ** frip.entry_claims.len;
    inline for (frip.entry_claims, 0..) |c, i| claims[i] = c;
    const bad_col = [_]ext.Ext{frip.entry_claims[0][0].add(ext.Ext.one())};
    claims[0] = &bad_col;

    var inputs = buildInputs();
    inputs.entry_claims = &claims;
    // The corrupted claim propagates into the reconstructed level pair, so the
    // running fold no longer matches the next layer's committed leaf.
    var t = seededTranscript();
    const proof = buildProof();
    const coins = verify.replayWithTranscript(frip.system, &t, proof);
    try std.testing.expectError(error.FoldMismatch, verify.verify(frip.system, inputs, proof, coins));
}

test "frip: a tampered input-tree opening is rejected" {
    // Corrupt the first query's first batch opening: flip a sibling coordinate.
    // The recovered Merkle root no longer matches the committed root, so input
    // authentication fails. (Query positions are FS-derived now, so tampering
    // the position is not a caller-reachable input — we tamper the proof data
    // whose authentication the derived position drives.)
    const orig = frip.input_queries[0][0];
    var bad_sibs = [_]pcs.tree.Octuplet{undefined} ** orig.siblings.len;
    inline for (orig.siblings, 0..) |sib, i| bad_sibs[i] = sib;
    bad_sibs[0][0] = bad_sibs[0][0].add(verifier_ray.field.koalabear.Element.one());

    const bad_open = pcs.paired_leaf.InputTreeOpening{ .siblings = &bad_sibs, .leaves = orig.leaves };
    const bad_batches = [_]pcs.paired_leaf.InputTreeOpening{bad_open};
    var bad_queries = [_][]const pcs.paired_leaf.InputTreeOpening{undefined} ** frip.input_queries.len;
    inline for (frip.input_queries, 0..) |q, i| bad_queries[i] = q;
    bad_queries[0] = &bad_batches;

    var proof = buildProof();
    proof.input_queries = &bad_queries;
    var t = seededTranscript();
    const coins = verify.replayWithTranscript(frip.system, &t, proof);
    try std.testing.expectError(error.InputTreeAuthFailed, verify.verify(frip.system, buildInputs(), proof, coins));
}

test "frip: a running-layer branch with the wrong sibling count is rejected" {
    // The NEW exact-shape check (verify.zig resolveRunningLayers) requires a
    // running layer j to carry exactly `log_codeword_size - j` siblings (and the
    // same number of aux siblings), porting Go's checkQueryLayerShape(
    // exactSiblings=true). Truncate the running-layer branch's sibling list so it
    // is one short; the shape check must reject BEFORE Merkle recovery runs.
    const orig = frip.running_queries[0][0];
    // Drop the first sibling → an under-length branch (orig has
    // log_codeword_size - 1 == 2 siblings for this fixture). Build the truncated
    // slices explicitly from the original.
    var sibs_buf: [8]pcs.tree.Octuplet = undefined;
    var aux_buf: [8]?pcs.tree.Octuplet = undefined;
    const keep = orig.siblings.len - 1;
    for (0..keep) |i| {
        sibs_buf[i] = orig.siblings[i + 1];
        aux_buf[i] = orig.aux_siblings[i + 1];
    }
    const bad_branch = pcs.tree.Branch{
        .leaf = orig.leaf,
        .siblings = sibs_buf[0..keep],
        .aux_siblings = aux_buf[0..keep],
    };
    const bad_q0 = [_]pcs.tree.Branch{bad_branch};
    var bad_rq = [_][]const pcs.tree.Branch{undefined} ** frip.running_queries.len;
    inline for (frip.running_queries, 0..) |q, i| bad_rq[i] = q;
    bad_rq[0] = &bad_q0;

    var proof = buildProof();
    proof.fri.running_queries = &bad_rq;
    var t = seededTranscript();
    const coins = verify.replayWithTranscript(frip.system, &t, proof);
    try std.testing.expectError(
        error.RunningLayerShapeMismatch,
        verify.verify(frip.system, buildInputs(), proof, coins),
    );
}

test "frip: a tampered running-layer sibling is rejected" {
    // Flip a coordinate of the running-layer branch's sibling: the shape is
    // still correct, but the recovered running-layer Merkle root no longer
    // matches the committed round root → RunningLayerAuthFailed.
    const orig = frip.running_queries[0][0];
    var bad_sibs = [_]pcs.tree.Octuplet{undefined} ** orig.siblings.len;
    inline for (orig.siblings, 0..) |sib, i| bad_sibs[i] = sib;
    bad_sibs[0][0] = bad_sibs[0][0].add(verifier_ray.field.koalabear.Element.one());

    const bad_branch = pcs.tree.Branch{
        .leaf = orig.leaf,
        .siblings = &bad_sibs,
        .aux_siblings = orig.aux_siblings,
    };
    const bad_q0 = [_]pcs.tree.Branch{bad_branch};
    var bad_rq = [_][]const pcs.tree.Branch{undefined} ** frip.running_queries.len;
    inline for (frip.running_queries, 0..) |q, i| bad_rq[i] = q;
    bad_rq[0] = &bad_q0;

    var proof = buildProof();
    proof.fri.running_queries = &bad_rq;
    var t = seededTranscript();
    const coins = verify.replayWithTranscript(frip.system, &t, proof);
    try std.testing.expectError(
        error.RunningLayerAuthFailed,
        verify.verify(frip.system, buildInputs(), proof, coins),
    );
}
