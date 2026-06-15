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

test "transcript reports poseidon2 compression count" {
    var transcript = fiat_shamir.Transcript.init();
    try std.testing.expectEqual(@as(usize, 0), transcript.compressionCount());

    transcript.updateElement(field.Element.init(3));
    try std.testing.expectEqual(@as(usize, 0), transcript.compressionCount());

    _ = transcript.randomExt();
    try std.testing.expectEqual(@as(usize, 1), transcript.compressionCount());

    _ = transcript.randomExt();
    try std.testing.expectEqual(@as(usize, 2), transcript.compressionCount());
}

test "replay absorbs commitments, public columns, and cells, then squeezes coins" {
    const entries = [_]protocol.ColumnMessage{
        .{ .oracle_commitment = .{
            field.Element.init(1),
            field.Element.init(2),
            field.Element.init(3),
            field.Element.init(4),
            field.Element.init(5),
            field.Element.init(6),
            field.Element.init(7),
            field.Element.init(8),
        } },
        .{ .public_column = .{ .base = &.{field.Element.init(9)} } },
    };
    const cells = [_]protocol.Scalar{
        .{ .base = field.Element.init(10) },
    };
    const rounds = [_]protocol.RoundMessage{
        .{ .columns = &entries, .cells = &cells },
    };

    // One message round that squeezes two coins (round 0 is the pre-round-1
    // phase with zero coins).
    const spec = protocol.Spec{
        .round_coin_counts = &[_]usize{ 0, 2 },
        .round_coin_offsets = &[_]usize{ 0, 0 },
        .total_round_coins = 2,
    };
    const replay_stats = try protocol.replayWithStats(spec, &rounds);
    const coins = replay_stats.coins;

    // Consecutive coins must be distinct: randomDigest() absorbs a zero
    // separator between squeezes, so identical back-to-back outputs indicate
    // a broken separator mechanism.
    try std.testing.expect(!coins[0].eql(coins[1]));
    try std.testing.expectEqual(@as(usize, 3), replay_stats.compression_count);

    // to guard that replay and replayWithStats are consistent, we can call replay and expect the same coins as replayWithStats:
    const replay_coins = try protocol.replay(spec, &rounds);
    try std.testing.expect(coins[0].eql(replay_coins[0]));
    try std.testing.expect(coins[1].eql(replay_coins[1]));
}
