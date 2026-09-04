const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;
const protocol = verifier_ray.protocol;

test "transcript absorbs elements deterministically" {
    var transcript = fiat_shamir.Transcript.init();
    transcript.updateElements(&.{
        field.Element.init(3),
        field.Element.init(4),
    });

    const challenge = transcript.randomExt();
    try std.testing.expect(!challenge.isZero());
}

test "replay absorbs a commitment and cells, then squeezes coins" {
    const commitment: protocol.Commitment = .{
        field.Element.init(1),
        field.Element.init(2),
        field.Element.init(3),
        field.Element.init(4),
        field.Element.init(5),
        field.Element.init(6),
        field.Element.init(7),
        field.Element.init(8),
    };
    const cells = [_]protocol.Scalar{
        .{ .base = field.Element.init(10) },
    };
    const rounds = [_]protocol.RoundMessage{
        .{ .commitment = commitment, .cells = &cells },
    };

    // One message round that squeezes two coins (round 0 is the pre-round-1
    // phase with zero coins).
    const spec = protocol.Spec{
        .round_coin_counts = &[_]usize{ 0, 2 },
        .round_coin_offsets = &[_]usize{ 0, 0 },
        .total_round_coins = 2,
    };
    var transcript = fiat_shamir.Transcript.init();
    const coins = try protocol.replayWithTranscript(&transcript, spec, &rounds, &.{});

    // Consecutive coins must be distinct: randomDigest() absorbs a zero
    // separator between squeezes, so identical back-to-back outputs indicate
    // a broken separator mechanism.
    try std.testing.expect(!coins[0].eql(coins[1]));
}

// A two-round spec where round 0 carries an 8-limb γ octuplet as its round
// cells, and round 1 (index 2 in round_coin_counts, since index 0 is the
// pre-round-1 phase) is the shared-randomness coin round that must derive its
// coin from γ instead of from the transcript state accumulated so far.
const shared_randomness_spec = protocol.Spec{
    .round_coin_counts = &[_]usize{ 0, 0, 1 },
    .round_coin_offsets = &[_]usize{ 0, 0, 0 },
    .total_round_coins = 1,
    .shared_randomness_coin_round = 2,
    .shared_randomness_gamma_refs = &[_]protocol.SharedRandomnessGammaRef{
        .{ .round = 0, .index = 0 },
        .{ .round = 0, .index = 1 },
        .{ .round = 0, .index = 2 },
        .{ .round = 0, .index = 3 },
        .{ .round = 0, .index = 4 },
        .{ .round = 0, .index = 5 },
        .{ .round = 0, .index = 6 },
        .{ .round = 0, .index = 7 },
    },
};

fn gammaCells(seed: u32) [8]protocol.Scalar {
    var cells: [8]protocol.Scalar = undefined;
    for (&cells, 0..) |*c, i| c.* = .{ .base = field.Element.init(seed + @as(u32, @intCast(i))) };
    return cells;
}

test "shared-randomness coin round derives its coin from gamma, not the prior transcript state" {
    const cells = gammaCells(1);
    const rounds = [_]protocol.RoundMessage{
        .{ .commitment = null, .cells = &cells },
        .{ .commitment = null, .cells = &.{} },
    };

    var transcript_a = fiat_shamir.Transcript.init();
    transcript_a.updateElement(field.Element.init(0xDEADBEEF));
    const coins_a = try protocol.replayWithTranscript(&transcript_a, shared_randomness_spec, &rounds, &.{});

    var transcript_b = fiat_shamir.Transcript.init();
    transcript_b.updateElement(field.Element.init(0xCAFEF00D));
    const coins_b = try protocol.replayWithTranscript(&transcript_b, shared_randomness_spec, &rounds, &.{});

    // Two verifiers with divergent pre-existing transcript state (as they
    // would have on independent shards) must still agree on the coin, because
    // it is derived from the shared gamma, not from either shard's own state.
    try std.testing.expect(coins_a[0].eql(coins_b[0]));
}

test "shared-randomness coin round is sensitive to gamma" {
    const cells_1 = gammaCells(1);
    const rounds_1 = [_]protocol.RoundMessage{
        .{ .commitment = null, .cells = &cells_1 },
        .{ .commitment = null, .cells = &.{} },
    };
    const cells_2 = gammaCells(2);
    const rounds_2 = [_]protocol.RoundMessage{
        .{ .commitment = null, .cells = &cells_2 },
        .{ .commitment = null, .cells = &.{} },
    };

    var transcript_1 = fiat_shamir.Transcript.init();
    const coins_1 = try protocol.replayWithTranscript(&transcript_1, shared_randomness_spec, &rounds_1, &.{});

    var transcript_2 = fiat_shamir.Transcript.init();
    const coins_2 = try protocol.replayWithTranscript(&transcript_2, shared_randomness_spec, &rounds_2, &.{});

    try std.testing.expect(!coins_1[0].eql(coins_2[0]));
}

test "shared-randomness gamma refs must resolve to base-field cells within bounds" {
    const rounds = [_]protocol.RoundMessage{
        .{ .commitment = null, .cells = &.{} }, // too few cells for the gamma refs
        .{ .commitment = null, .cells = &.{} },
    };

    var transcript = fiat_shamir.Transcript.init();
    try std.testing.expectError(
        error.InvalidSharedRandomnessGamma,
        protocol.replayWithTranscript(&transcript, shared_randomness_spec, &rounds, &.{}),
    );
}
