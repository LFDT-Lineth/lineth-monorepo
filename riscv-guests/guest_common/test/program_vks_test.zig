const std = @import("std");
const program_vks = @import("guest_common").program_vks;

fn vk(byte: u8) [32]u8 {
    return @splat(byte);
}

const LeafProof = struct {
    proof: struct { public_inputs: struct {} = .{} } = .{},
    program_vk: [32]u8,
};

const BubblingProof = struct {
    proof: struct { public_inputs: struct { program_vks: struct { items: []const [32]u8 } } },
    program_vk: [32]u8,
};

fn bubbling(own: u8, inner: []const [32]u8) BubblingProof {
    return .{ .proof = .{ .public_inputs = .{ .program_vks = .{ .items = inner } } }, .program_vk = vk(own) };
}

test "proofs that bubble no VKs yield their own VKs, distinct and ascending" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proofs = [_]LeafProof{ .{ .program_vk = vk(3) }, .{ .program_vk = vk(1) }, .{ .program_vk = vk(3) } };
    const vks = try program_vks.collect(arena.allocator(), &proofs);
    try std.testing.expectEqualDeep(&[_][32]u8{ vk(1), vk(3) }, vks);
}

test "bubbled VKs join the VKs the proofs are verified under" {
    var arena = std.heap.ArenaAllocator.init(std.testing.allocator);
    defer arena.deinit();

    const proofs = [_]BubblingProof{
        bubbling(9, &.{ vk(2), vk(5) }),
        bubbling(8, &.{}),
        bubbling(9, &.{ vk(5), vk(1) }),
    };
    const vks = try program_vks.collect(arena.allocator(), &proofs);
    try std.testing.expectEqualDeep(&[_][32]u8{ vk(1), vk(2), vk(5), vk(8), vk(9) }, vks);
}

test "no proofs have no VKs" {
    const vks = try program_vks.collect(std.testing.allocator, &[_]LeafProof{});
    try std.testing.expectEqual(@as(usize, 0), vks.len);
}
