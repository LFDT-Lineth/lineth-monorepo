const std = @import("std");
const verifier_ray = @import("verifier_ray");

const protocol = verifier_ray.protocol;
const verifier = verifier_ray.verifier;
const vanishing = verifier_ray.query.vanishing;
const pcs = verifier_ray.query.pcs;
const fri = verifier_ray.query.fri;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;
const ext = verifier_ray.field.koalabear_ext;
const poseidon2 = verifier_ray.crypto.poseidon2;

test "verify completes replay, routing, and dispatch on a minimal proof" {
    const spec = protocol.Spec{
        .round_coin_counts = &[_]usize{0},
        .round_coin_offsets = &[_]usize{0},
        .total_round_coins = 0,
    };
    const systems = verifier.Systems{
        .vanishing = vanishing.System{ .modules = &.{} },
    };
    try verifier.verify(spec, systems, .{
        .rounds = &.{},
        .witness_claims = &.{},
        .quotient_claims = &.{},
    });
}

test "verify rejects proof with wrong round count" {
    const spec = protocol.Spec{
        .round_coin_counts = &[_]usize{ 0, 1 },
        .round_coin_offsets = &[_]usize{ 0, 0 },
        .total_round_coins = 1,
    };
    const systems = verifier.Systems{
        .vanishing = vanishing.System{ .modules = &.{} },
    };
    try std.testing.expectError(
        error.InvalidRoundCount,
        verifier.verify(spec, systems, .{
            .rounds = &.{},
            .witness_claims = &.{},
            .quotient_claims = &.{},
        }),
    );
}

// ── PCS challenge derivation ──────────────────────────────────────────────────
//
// `deriveChallenges` squeezes the FRI fold challenges + query positions from a
// caller-owned transcript. There is no golden vector for these on this branch
// (the pcs.zig fixtures carry synthetic challenges, not transcript-derived
// ones), so these tests pin the properties that must hold regardless of the
// exact values: correct shape, determinism, and sensitivity to the absorbed
// transcript state.

// numRounds = log_plaintext_size - log_final_poly_size = 2, so fold_alphas has
// length 2 and the FRI proof carries numRounds-1 = 1 running-layer root.
const challenge_system = pcs.System{
    .params = .{ .log_codeword_size = 4, .log_plaintext_size = 2, .num_queries = 3 },
    .layout = &.{},
};

fn digest(seed: u32) poseidon2.Digest {
    var d: poseidon2.Digest = undefined;
    for (&d, 0..) |*limb, i| limb.* = verifier_ray.field.koalabear.Element.init(seed +% @as(u32, @intCast(i)));
    return d;
}

// A well-shaped FRI proof for `challenge_system`: exactly num_rounds-1 == 1
// running-layer root.
fn challengeFriProof(root_seed: u32) fri.Proof {
    const S = struct {
        var round_roots: [1]poseidon2.Digest = undefined;
        var final_poly = [_]ext.Ext{ext.Ext.zero()};
    };
    S.round_roots[0] = digest(root_seed);
    return .{ .round_roots = &S.round_roots, .final_poly = &S.final_poly, .running_queries = &.{} };
}

test "deriveChallenges produces the comptime-sized shape" {
    var transcript = fiat_shamir.Transcript.init();
    const challenges = try pcs.deriveChallenges(challenge_system, &transcript, challengeFriProof(1));
    try std.testing.expectEqual(@as(usize, 2), challenges.fold_alphas.len);
    try std.testing.expectEqual(@as(usize, 3), challenges.query_positions.len);
    // Query positions are reduced into the codeword domain (2^4 = 16).
    for (challenges.query_positions) |p| try std.testing.expect(p < 16);
}

test "deriveChallenges is deterministic for the same transcript and proof" {
    var t1 = fiat_shamir.Transcript.init();
    var t2 = fiat_shamir.Transcript.init();
    const a = try pcs.deriveChallenges(challenge_system, &t1, challengeFriProof(7));
    const b = try pcs.deriveChallenges(challenge_system, &t2, challengeFriProof(7));
    for (a.fold_alphas, b.fold_alphas) |x, y| try std.testing.expect(x.eql(y));
    try std.testing.expectEqualSlices(usize, &a.query_positions, &b.query_positions);
}

test "deriveChallenges depends on the absorbed transcript state" {
    // Two transcripts diverging before derivation must yield different
    // challenges: they are a function of the live Fiat-Shamir state, not just
    // the proof. (Absorb one differing element up front.)
    var t1 = fiat_shamir.Transcript.init();
    var t2 = fiat_shamir.Transcript.init();
    t1.updateExt(&.{ext.Ext.fromUints(.{ 1, 0, 0, 0, 0, 0 })});
    t2.updateExt(&.{ext.Ext.fromUints(.{ 2, 0, 0, 0, 0, 0 })});
    const a = try pcs.deriveChallenges(challenge_system, &t1, challengeFriProof(9));
    const b = try pcs.deriveChallenges(challenge_system, &t2, challengeFriProof(9));
    var any_alpha_differs = false;
    for (a.fold_alphas, b.fold_alphas) |x, y| {
        if (!x.eql(y)) any_alpha_differs = true;
    }
    try std.testing.expect(any_alpha_differs);
}
