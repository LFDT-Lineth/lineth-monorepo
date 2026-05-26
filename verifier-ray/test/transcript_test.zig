const std = @import("std");
const verifier_ray = @import("verifier_ray");

const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;
const fiat_shamir = verifier_ray.crypto.fiat_shamir;
const runtime = verifier_ray.runtime;

test "transcript absorbs elements deterministically" {
    var transcript = fiat_shamir.Transcript.init();
    transcript.updateElements(&.{
        field.Element.init(3),
        field.Element.init(4),
    });

    const challenge = transcript.randomExt();
    try std.testing.expect(!challenge.isZero());
}

test "runtime rejects skipped and replayed rounds" {
    var rt = runtime.Runtime.initWithRoundCount(3);
    var coins: [1]runtime.Coin = undefined;

    try std.testing.expectError(
        error.UnexpectedRound,
        rt.advanceRoundWithMessage(1, .{}, &coins),
    );

    _ = try rt.advanceRoundWithMessage(0, .{}, &coins);

    try std.testing.expectError(
        error.UnexpectedRound,
        rt.advanceRoundWithMessage(0, .{}, &coins),
    );
    try std.testing.expectError(
        error.UnexpectedRound,
        rt.advanceRoundWithMessage(2, .{}, &coins),
    );
}

test "runtime rejects advancing without a next round" {
    var no_rounds = runtime.Runtime.init();
    var coins: [1]runtime.Coin = undefined;

    try std.testing.expectError(
        error.NoRounds,
        no_rounds.advanceRoundWithMessage(0, .{}, &coins),
    );

    var last_round = runtime.Runtime.initWithRoundCount(1);
    try std.testing.expectError(
        error.LastRound,
        last_round.advanceRoundWithMessage(0, .{}, &coins),
    );
}

test "runtime absorbs columns and squeezes requested extension coins" {
    var rt = runtime.Runtime.initWithRoundCount(2);
    var coins: [2]runtime.Coin = undefined;

    const columns = [_]runtime.ColumnAssignment{
        .{ .visibility = .oracle, .assignment = .{ .base = &.{field.Element.init(1)} } },
    };
    const cells = [_]runtime.Scalar{
        .{ .base = field.Element.init(2) },
    };
    const got = try rt.advanceRoundWithMessage(0, .{
        .columns = &columns,
        .cells = &cells,
        .next_round_coin_count = coins.len,
    }, &coins);

    try std.testing.expectEqual(@as(usize, 2), got.len);
    try std.testing.expectEqual(@as(usize, 1), rt.current_round);
}

test "runtime rejects undersized coin output buffer" {
    var rt = runtime.Runtime.initWithRoundCount(2);
    var coins: [0]runtime.Coin = .{};

    try std.testing.expectError(
        error.OutputTooSmall,
        rt.advanceRoundWithMessage(0, .{ .next_round_coin_count = 1 }, &coins),
    );
}
