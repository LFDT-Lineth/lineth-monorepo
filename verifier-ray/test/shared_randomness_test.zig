const std = @import("std");
const verifier_ray = @import("verifier_ray");

const protocol = verifier_ray.protocol;
const shared_randomness = verifier_ray.query.shared_randomness;
const poseidon2 = verifier_ray.crypto.poseidon2;
const multiset_hashing = verifier_ray.crypto.multiset_hashing;
const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;

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

// expectedContribution expands one commitment octuplet through
// multisethashing.Hash, exactly as prover-ray's sharedRandomnessContribution
// does — used to build a HONEST contribution for the positive tests.
fn expectedContribution(com: [8]field.Element) multiset_hashing.MSetHash {
    return multiset_hashing.hash(com);
}

fn contributionToScalars(m: multiset_hashing.MSetHash) [multiset_hashing.size]protocol.Scalar {
    var out: [multiset_hashing.size]protocol.Scalar = undefined;
    for (&out, m) |*s, limb| s.* = .{ .base = limb };
    return out;
}

// makeCtx builds a 3-round Context: round 0 and round 1 each carry a
// commitment octuplet (no cells), round 2 carries the contribution cells (the
// "public input" cells, already merged in by verifier.verify's
// bindRoundMessages by the time any sub-verifier runs).
fn makeCtx(
    com0: [8]field.Element,
    com1: [8]field.Element,
    contribution: *const [multiset_hashing.size]protocol.Scalar,
    rounds_buf: *[3]protocol.RoundMessage,
) protocol.Context {
    rounds_buf.* = .{
        .{ .commitment = com0, .cells = &.{} },
        .{ .commitment = com1, .cells = &.{} },
        .{ .commitment = null, .cells = contribution },
    };
    return .{ .all_coins = &.{}, .rounds = rounds_buf };
}

// coin_round_system names round 1 as the round whose commitment is the whole
// preimage, with the contribution cells living at round 2, indices
// 0..multiset_hashing.size — mirroring BuildSharedRandomnessSystem's output for
// a coin round at index 1. Round 0's commitment is deliberately irrelevant
// here, exactly as it is to prover-ray's `rt.Commitments[coinRound.ID]`.
const coin_round_refs = blk: {
    var refs: [multiset_hashing.size]shared_randomness.ScalarRef = undefined;
    for (&refs, 0..) |*ref, i| {
        ref.* = .{ .round = 2, .index = i };
    }
    break :blk refs;
};
const coin_round_system = shared_randomness.System{
    .commitment_round = .{ .round = 1, .has_commitment = true },
    .contribution_refs = &coin_round_refs,
};

test "shared randomness accepts a correctly recomputed contribution" {
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    const contribution = contributionToScalars(expectedContribution(com1));

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try shared_randomness.verify(coin_round_system, makeCtx(com0, com1, &contribution, &rounds_buf));
}

test "shared randomness rejects a tampered committed-round octuplet" {
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    // Contribution computed over the HONEST commitments...
    const contribution = contributionToScalars(expectedContribution(com1));

    // ...but the ctx now carries a DIFFERENT round-1 commitment (as if the
    // prover swapped in a different batch's root after the fact). The claimed
    // contribution cells still reflect the original, honest octuplets, so the
    // recomputed contribution must now disagree with them.
    const tampered_com1 = octuplet(999);

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ContributionMismatch,
        shared_randomness.verify(coin_round_system, makeCtx(com0, tampered_com1, &contribution, &rounds_buf)),
    );
}

test "shared randomness rejects a tampered claimed contribution digest" {
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    var contribution = contributionToScalars(expectedContribution(com1));
    // Flip one limb of the claimed contribution away from the honestly recomputed
    // value (as if the prover just wrote a made-up contribution).
    contribution[3] = baseScalar(0xDEAD);

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ContributionMismatch,
        shared_randomness.verify(coin_round_system, makeCtx(com0, com1, &contribution, &rounds_buf)),
    );
}

test "shared randomness rejects a tampered claimed contribution limb past the first chunk" {
    // Chunk 0 of multisethashing.Hash reproduces the raw 8-limb sponge digest
    // exactly, so a mismatch confined to indices 0..8 cannot distinguish a
    // verifier that expanded the full 328-limb multiset hash from one that
    // stopped after the first SumDigest() call. Flipping limb 8 (chunk 1) only
    // fails if the chunked expansion actually ran to completion.
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    var contribution = contributionToScalars(expectedContribution(com1));
    contribution[8] = baseScalar(0xDEAD);

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ContributionMismatch,
        shared_randomness.verify(coin_round_system, makeCtx(com0, com1, &contribution, &rounds_buf)),
    );
}

test "shared randomness rejects an extension-field-encoded contribution limb" {
    const com0 = octuplet(1);
    const com1 = octuplet(100);
    var contribution = contributionToScalars(expectedContribution(com1));

    // Re-encode limb 5 as an extension scalar carrying the SAME numeric value
    // (lift of the honest base limb). The value is correct, so a verifier that
    // lifts via toExt() would wrongly accept it; the protocol requires these
    // cells to be base-field (prover-ray's contributionCell panics on
    // extension cells), so this must be rejected as ContributionNotBaseField.
    const honest_limb = contribution[5].base;
    contribution[5] = .{ .ext = ext.Ext.lift(honest_limb) };

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try std.testing.expectError(
        error.ContributionNotBaseField,
        shared_randomness.verify(coin_round_system, makeCtx(com0, com1, &contribution, &rounds_buf)),
    );
}

test "shared randomness ignores commitments outside the coin round" {
    // Only the coin round's own commitment is the preimage, so a different
    // round-0 commitment must not move the contribution — the counterpart of
    // the tampered-round-1 case above, and the property that makes the single
    // `rt.Commitments[coinRound.ID]` lookup faithful.
    const com1 = octuplet(100);
    const contribution = contributionToScalars(expectedContribution(com1));

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    try shared_randomness.verify(coin_round_system, makeCtx(octuplet(7), com1, &contribution, &rounds_buf));
    try shared_randomness.verify(coin_round_system, makeCtx(octuplet(999), com1, &contribution, &rounds_buf));
}

test "shared randomness hashes zeroes when the coin round carries no commitment" {
    // prover-ray reads rt.Commitments[coinRound.ID] from a map, so a coin round
    // that committed no column yields the zero Octuplet and the prover hashes
    // that. The verifier must agree rather than erroring, or the two sides
    // disagree on a protocol the compiler still permits. (That this check is
    // then vacuous is a prover-side concern — see the messagebus warning.)
    const zero: [8]field.Element = @splat(field.Element.zero());
    const contribution = contributionToScalars(expectedContribution(zero));

    const system = shared_randomness.System{
        .commitment_round = .{ .round = 1, .has_commitment = false },
        .contribution_refs = &coin_round_refs,
    };

    var rounds_buf: [3]protocol.RoundMessage = undefined;
    // The ctx still carries a round-1 commitment; has_commitment = false must
    // make the checker ignore it in favour of the zero octuplet.
    try shared_randomness.verify(system, makeCtx(octuplet(1), octuplet(100), &contribution, &rounds_buf));
}

test "empty shared-randomness system verifies trivially" {
    const empty_system = shared_randomness.System{};
    const empty_ctx: protocol.Context = .{ .all_coins = &.{}, .rounds = &[_]protocol.RoundMessage{} };
    try shared_randomness.verify(empty_system, empty_ctx);
}
