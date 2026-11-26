//! The L2-to-L1 roots: groups of 32 messages, the last one zero-padded.

const std = @import("std");
const l2_l1_roots = @import("rollup_aggregation").l2_l1_roots;

fn message(n: usize) [32]u8 {
    return std.mem.toBytes(std.mem.nativeToBig(u256, n + 1));
}

fn messages(alloc: std.mem.Allocator, count: usize) ![][32]u8 {
    const out = try alloc.alloc([32]u8, count);
    for (out, 0..) |*m, i| m.* = message(i);
    return out;
}

fn expectRoots(count: usize, want_roots: usize) ![][32]u8 {
    const input = try messages(std.testing.allocator, count);
    defer std.testing.allocator.free(input);
    const roots = try l2_l1_roots.build(std.testing.allocator, input);
    try std.testing.expectEqual(want_roots, roots.len);
    return roots;
}

test "no messages have no roots" {
    std.testing.allocator.free(try expectRoots(0, 0));
}

test "four messages pack into one root" {
    const input = [_][32]u8{ @splat(0x77), @splat(0x88), @splat(0x77), @splat(0x88) };
    const roots = try l2_l1_roots.build(std.testing.allocator, &input);
    defer std.testing.allocator.free(roots);
    var want: [32]u8 = undefined;
    _ = try std.fmt.hexToBytes(&want, "653b4727bb7c5a399279c43baa435a3f9eaf400982d6206e569662ea02666ab2");
    try std.testing.expectEqual(@as(usize, 1), roots.len);
    try std.testing.expectEqualSlices(u8, &want, &roots[0]);
}

test "a full group is one root and a message beyond it opens a second group" {
    const full = try expectRoots(32, 1);
    defer std.testing.allocator.free(full);
    const spilled = try expectRoots(33, 2);
    defer std.testing.allocator.free(spilled);
    try std.testing.expectEqualSlices(u8, &full[0], &spilled[0]);

    const lone = [_][32]u8{message(32)};
    const lone_root = try l2_l1_roots.build(std.testing.allocator, &lone);
    defer std.testing.allocator.free(lone_root);
    try std.testing.expectEqualSlices(u8, &lone_root[0], &spilled[1]);
}

test "a short group is the full group padded with zero messages" {
    const short = try expectRoots(1, 1);
    defer std.testing.allocator.free(short);
    var padded: [32][32]u8 = @splat(@splat(0));
    padded[0] = message(0);
    const padded_root = try l2_l1_roots.build(std.testing.allocator, &padded);
    defer std.testing.allocator.free(padded_root);
    try std.testing.expectEqualSlices(u8, &padded_root[0], &short[0]);
}

test "a root depends on the order of its messages" {
    const forward = [_][32]u8{ message(0), message(1) };
    const backward = [_][32]u8{ message(1), message(0) };
    const a = try l2_l1_roots.build(std.testing.allocator, &forward);
    defer std.testing.allocator.free(a);
    const b = try l2_l1_roots.build(std.testing.allocator, &backward);
    defer std.testing.allocator.free(b);
    try std.testing.expect(!std.mem.eql(u8, &a[0], &b[0]));
}
