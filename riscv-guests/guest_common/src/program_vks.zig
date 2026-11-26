//! The program VKs behind a proof over other proofs: the VK each inner proof is verified under and
//! the VKs its public input bubbles up.

const std = @import("std");
const lists = @import("lists.zig");

const Allocator = std.mem.Allocator;

/// The program VKs behind `proofs`, distinct and ascending bytewise. `proofs` is a slice of
/// verifiable proofs: each holds its own VK at `program_vk` and, when its public input has a
/// `program_vks` list, the VKs it bubbles up at `proof.public_inputs.program_vks.items`.
pub fn collect(allocator: Allocator, proofs: anytype) Allocator.Error![][32]u8 {
    var count: usize = proofs.len;
    for (proofs) |verifiable| count += bubbled(verifiable).len;
    const vks = try allocator.alloc([32]u8, count);
    var next: usize = 0;
    for (proofs) |verifiable| {
        const inner = bubbled(verifiable);
        @memcpy(vks[next..][0..inner.len], inner);
        next += inner.len;
        vks[next] = verifiable.program_vk;
        next += 1;
    }
    return lists.sortedDistinct(32, vks);
}

fn bubbled(verifiable: anytype) []align(1) const [32]u8 {
    const public_inputs = verifiable.proof.public_inputs;
    if (!@hasField(@TypeOf(public_inputs), "program_vks")) return &.{};
    return public_inputs.program_vks.items;
}
