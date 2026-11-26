const std = @import("std");
const verifier_ray = @import("verifier_ray");

const protocol = verifier_ray.protocol;
const e6check = verifier_ray.query.e6check;
const field = verifier_ray.field.koalabear;
const ext = verifier_ray.field.koalabear_ext;

// Hand-built cases pinning the E6 public-input check directly via ScalarRef
// lookups into a runtime ctx, mirroring shared_randomness_test.zig. The
// end-to-end case (a real recursion program's proof, honest and with a forged
// E6 public input) is the RecursionE6PublicInput fixture in verify.zig.

fn base(v: u32) protocol.Scalar {
    return .{ .base = field.Element.init(v) };
}

fn e6(v: [6]u32) ext.Ext {
    return .{
        .B0 = .{ .a0 = field.Element.init(v[0]), .a1 = field.Element.init(v[1]) },
        .B1 = .{ .a0 = field.Element.init(v[2]), .a1 = field.Element.init(v[3]) },
        .B2 = .{ .a0 = field.Element.init(v[4]), .a1 = field.Element.init(v[5]) },
    };
}

const lane_values = [6]u32{ 11, 12, 13, 14, 15, 16 };

// Round 0 carries the six lane openings at indices 1..6 behind an unrelated
// cell at index 0; round 1 carries the E6 public input at index 0, as
// verifier.verify's bindRoundMessages leaves it before any sub-verifier runs.
fn laneCells() [7]protocol.Scalar {
    var cells: [7]protocol.Scalar = undefined;
    cells[0] = base(999);
    for (lane_values, 1..) |v, i| cells[i] = base(v);
    return cells;
}

const one_check = [_]e6check.Check{.{
    .cell = .{ .round = 1, .index = 0 },
    .lanes = .{
        .{ .round = 0, .index = 1 }, .{ .round = 0, .index = 2 }, .{ .round = 0, .index = 3 },
        .{ .round = 0, .index = 4 }, .{ .round = 0, .index = 5 }, .{ .round = 0, .index = 6 },
    },
}};
const system = e6check.System{ .checks = &one_check };

fn ctxOf(rounds: []const protocol.RoundMessage) protocol.Context {
    return .{ .all_coins = &.{}, .rounds = rounds };
}

test "e6check accepts the E6 value of its lanes" {
    const lanes = laneCells();
    const public = [_]protocol.Scalar{.{ .ext = e6(lane_values) }};
    const rounds = [_]protocol.RoundMessage{ .{ .cells = &lanes }, .{ .cells = &public } };
    try e6check.verify(system, ctxOf(&rounds));
}

test "e6check rejects a forged public input" {
    const lanes = laneCells();
    // Each lane flipped in turn: the comparison covers all six coordinates.
    for (0..6) |l| {
        var forged = lane_values;
        forged[l] += 1;
        const public = [_]protocol.Scalar{.{ .ext = e6(forged) }};
        const rounds = [_]protocol.RoundMessage{ .{ .cells = &lanes }, .{ .cells = &public } };
        try std.testing.expectError(error.E6Mismatch, e6check.verify(system, ctxOf(&rounds)));
    }
}

test "e6check rejects permuted lanes" {
    // B0.a1 and B1.a0 swapped: pins the (B0.a0, B0.a1, B1.a0, B1.a1, B2.a0,
    // B2.a1) lane order of recursion.CellE6.
    const lanes = laneCells();
    const public = [_]protocol.Scalar{.{ .ext = e6(.{ 11, 13, 12, 14, 15, 16 }) }};
    const rounds = [_]protocol.RoundMessage{ .{ .cells = &lanes }, .{ .cells = &public } };
    try std.testing.expectError(error.E6Mismatch, e6check.verify(system, ctxOf(&rounds)));
}

test "e6check rejects an extension lane" {
    var lanes = laneCells();
    lanes[3] = .{ .ext = ext.Ext.lift(field.Element.init(lane_values[2])) };
    const public = [_]protocol.Scalar{.{ .ext = e6(lane_values) }};
    const rounds = [_]protocol.RoundMessage{ .{ .cells = &lanes }, .{ .cells = &public } };
    try std.testing.expectError(error.E6LaneNotBaseField, e6check.verify(system, ctxOf(&rounds)));
}

test "e6check compares a base public input as its lift" {
    // Go's E6Check compares AsExt(), which lifts a base value: a public input
    // tagged base equals the E6 value of lanes (v, 0, 0, 0, 0, 0) only.
    var cells = [_]protocol.Scalar{ base(0), base(7), base(0), base(0), base(0), base(0), base(0) };
    const public = [_]protocol.Scalar{base(7)};
    const rounds = [_]protocol.RoundMessage{ .{ .cells = &cells }, .{ .cells = &public } };
    try e6check.verify(system, ctxOf(&rounds));

    cells[2] = base(1);
    try std.testing.expectError(error.E6Mismatch, e6check.verify(system, ctxOf(&rounds)));
}

test "e6check rejects out-of-range refs" {
    const lanes = laneCells();
    const short = [_]protocol.RoundMessage{.{ .cells = &lanes }};
    try std.testing.expectError(error.CellRefOutOfRange, e6check.verify(system, ctxOf(&short)));

    const public = [_]protocol.Scalar{.{ .ext = e6(lane_values) }};
    const truncated = [_]protocol.RoundMessage{ .{ .cells = lanes[0..4] }, .{ .cells = &public } };
    try std.testing.expectError(error.CellRefOutOfRange, e6check.verify(system, ctxOf(&truncated)));
}

test "e6check with no checks verifies trivially" {
    try e6check.verify(.{}, ctxOf(&.{}));
}
