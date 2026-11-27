//! The L2-to-L1 message roots of a finalization: the messages in groups of `1 << tree_depth`
//! leaves, the last group zero-padded, each group's root the keccak256 fold of its leaves.

const std = @import("std");
const zkvm_accelerators = @import("zkvm_accelerators");

const Allocator = std.mem.Allocator;

pub const tree_depth: u64 = 5;
const leaves_per_tree = 1 << tree_depth;

/// One root per group of `messages`; no messages, no roots.
pub fn build(allocator: Allocator, messages: []align(1) const [32]u8) Allocator.Error![][32]u8 {
    const roots = try allocator.alloc([32]u8, (messages.len + leaves_per_tree - 1) / leaves_per_tree);
    for (roots, 0..) |*root, group| {
        var layer: [leaves_per_tree][32]u8 = @splat(@splat(0));
        const leaves = messages[group * leaves_per_tree ..][0..@min(leaves_per_tree, messages.len - group * leaves_per_tree)];
        @memcpy(layer[0..leaves.len], leaves);

        var width: usize = leaves_per_tree;
        while (width > 1) : (width /= 2) {
            for (0..width / 2) |i| {
                var pair: [64]u8 = undefined;
                pair[0..32].* = layer[2 * i];
                pair[32..].* = layer[2 * i + 1];
                zkvm_accelerators.keccak256(&pair, &layer[i]);
            }
        }
        root.* = layer[0];
    }
    return roots;
}
