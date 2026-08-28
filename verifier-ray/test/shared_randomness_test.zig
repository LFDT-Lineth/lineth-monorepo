const std = @import("std");
const verifier_ray = @import("verifier_ray");

const protocol = verifier_ray.protocol;
const shared_randomness = verifier_ray.query.shared_randomness;
const poseidon2 = verifier_ray.crypto.poseidon2;
const field = verifier_ray.field.koalabear;

// Hand-built cases pinning the shared-randomness contribution check directly
// via ScalarRef/round lookups into a runtime ctx, mirroring grandproduct_test.zig
// / logderivativesum_test.zig — so an adversary cannot bypass it by altering
// proof cells or commitments.
//
// The sponge digest itself is computed at RUNTIME (not comptime): Poseidon2's
// permutation is far too expensive to evaluate within the compiler's default
// backwards-branch quota, same reason grandproduct/logderivativesum's own
// fixtures avoid hashing at comptime.

fn baseScalar(v: u32) protocol.Scalar {
    return .{ .base = field.Element.init(v) };
}

fn octuplet(seed: u32) [8]field.Element {
    var out: [8]field.Element = undefined;
    for (&out, 0..) |*e, i| e.* = field.Element.init(seed + @as(u32, @intCast(i)));
    return out;
}

// expectedDigest computes the Poseidon2 MD sponge digest over the given
// committed-round octuplets, exactly as the checker itself does — used to
// build a HONEST contribution for the positive test.
fn expectedDigest(coms: []const [8]field.Element) poseidon2.Digest {
    var hasher = poseidon2.MDHasher.init();
    for (coms) |c| hasher.writeElements(&c);
    return hasher.sumDigest();
}

fn digestToScalars(d: poseidon2.Digest) [8]protocol.Scalar {
    var out: [8]protocol.Scalar = undefined;
    for (&out, d) |*s, limb| s.* = .{ .base = limb };
    return out;
}

// makeCtx builds a 3-round Context: round 0 and round 1 each carry a
// commitment octuplet (no cells), round 2 carries the 8 contribution cells
// (the "public input" cells, already merged in by verifier.verify's
// bindRoundMessages by the time any sub-verifier runs).
fn makeCtx(
    com0: [8]field.Element,
    com1: [8]field.Element,
    contribution: *const [8]protocol.Scalar,
    rounds_buf: *[3]protocol.RoundMessage,
) protocol.Context {
    rounds_buf.* = .{
        .{ .commitment = com0, .cells = &.{} },
        .{ .commitment = com1, .cells = &.{} },
        .{ .commitment = null, .cells = contribution },
    };
    return .{ .all_coins = &.{}, .rounds = rounds_buf };
}

// twoRoundSystem describes both round 0 and round 1 as committed rounds
// feeding the sponge, with the contribution cells living at round 2, indices
// 0..8 — mirroring BuildSharedRandomnessSystem's output shape for a coin round
// at index 2 with two preceding committed rounds.
const two_round_system_rounds = [_]shared_randomness.Round{
    .{ .round = 0, .has_commitment = true },
    .{ .round = 1, .has_commitment = true },
};
const two_round_system_refs = [_]shared_randomness.ScalarRef{
    .{ .round = 2, .index = 0 },
    .{ .round = 2, .index = 1 },
    .{ .round = 2, .index = 2 },
    .{ .round = 2, .index = 3 },
    .{ .round = 2, .index = 4 },
    .{ .round = 2, .index = 5 },
    .{ .round = 2, .index = 6 },
    .{ .round = 2, .index = 7 },
};
const two_round_system = shared_randomness.System{
    .rounds = &two_round_system_rounds,
    .contribution_refs = &two_round_system_refs,
};

test "shared randomness accepts a correctly recomputed contribution" {
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    const digest = expectedDigest(&[_][8]field.Element{ com0, com1 });
    const contribution = digestToScalars(digest);

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try shared_randomness.verify(two_round_system, makeCtx(com0, com1, &contribution, &rounds_buf));
}

test "shared randomness rejects a tampered committed-round octuplet" {
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    // Digest computed over the HONEST commitments...
    const digest = expectedDigest(&[_][8]field.Element{ com0, com1 });
    const contribution = digestToScalars(digest);

    // ...but the ctx now carries a DIFFERENT round-1 commitment (as if the
    // prover swapped in a different batch's root after the fact). The claimed
    // contribution cells still reflect the original, honest octuplets, so the
    // recomputed digest must now disagree with them.
    const tampered_com1 = octuplet(999);

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ContributionMismatch,
        shared_randomness.verify(two_round_system, makeCtx(com0, tampered_com1, &contribution, &rounds_buf)),
    );
}

test "shared randomness rejects a tampered claimed contribution digest" {
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    const digest = expectedDigest(&[_][8]field.Element{ com0, com1 });
    var contribution = digestToScalars(digest);
    // Flip one limb of the claimed digest away from the honestly recomputed
    // value (as if the prover just wrote a made-up contribution).
    contribution[3] = baseScalar(0xDEAD);

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ContributionMismatch,
        shared_randomness.verify(two_round_system, makeCtx(com0, com1, &contribution, &rounds_buf)),
    );
}

test "shared randomness skips a round with has_commitment = false" {
    // Round 0 is flagged as uncommitted, so its octuplet must NOT enter the
    // sponge preimage — only round 1's should, mirroring prover-ray's
    // `if !rt.System.Rounds[i].HasCommitment { continue }`.
    const com0 = octuplet(7); // must be ignored
    const com1 = octuplet(100);
    const digest = expectedDigest(&[_][8]field.Element{com1}); // com0 excluded
    const contribution = digestToScalars(digest);

    const rounds = [_]shared_randomness.Round{
        .{ .round = 0, .has_commitment = false },
        .{ .round = 1, .has_commitment = true },
    };
    const system = shared_randomness.System{
        .rounds = &rounds,
        .contribution_refs = &two_round_system_refs,
    };

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try shared_randomness.verify(system, makeCtx(com0, com1, &contribution, &rounds_buf));
}

test "empty shared-randomness system verifies trivially" {
    const empty_system = shared_randomness.System{};
    const empty_ctx: protocol.Context = .{ .all_coins = &.{}, .rounds = &[_]protocol.RoundMessage{} };
    try shared_randomness.verify(empty_system, empty_ctx);
}
