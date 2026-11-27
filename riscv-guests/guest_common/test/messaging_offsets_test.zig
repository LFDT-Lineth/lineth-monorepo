//! The messaging-block offsets of a proof rebased onto the first block of the range containing it.

const std = @import("std");

const messaging_offsets = @import("guest_common").messaging_offsets;

test "an offset rebases to 65535 but not to 65536" {
    // The proof starts at block 65535, which is 65534 blocks after the first block of the range.
    var proof: messaging_offsets.Proof = .{
        .start_block_number = 65_535,
        .end_block_number = 65_536,
        .block_count = 2,
        .offsets = &.{1},
    };

    var out: [2]u16 = undefined;
    try messaging_offsets.rebase(proof, 1, out[0..1]);
    try std.testing.expectEqual(@as(u16, 65_535), out[0]);

    proof.offsets = &.{ 1, 2 };
    try std.testing.expectError(error.InvalidMessagingOffsets, messaging_offsets.rebase(proof, 1, &out));
}

test "offsets are shifted by the distance from the range start" {
    const proof: messaging_offsets.Proof = .{ .start_block_number = 12, .end_block_number = 15, .block_count = 4, .offsets = &.{ 1, 4 } };
    var out: [2]u16 = undefined;
    try messaging_offsets.rebase(proof, 10, &out);
    try std.testing.expectEqualSlices(u16, &.{ 3, 6 }, &out);
}

test "a block count other than the range's is rejected" {
    var out: [0]u16 = undefined;
    const proof: messaging_offsets.Proof = .{ .start_block_number = 1, .end_block_number = 3, .block_count = 4, .offsets = &.{} };
    try std.testing.expectError(error.InvalidMessagingOffsets, messaging_offsets.rebase(proof, 1, &out));
}

test "a proof starting before the range is rejected" {
    var out: [1]u16 = undefined;
    const proof: messaging_offsets.Proof = .{ .start_block_number = 1, .end_block_number = 1, .block_count = 1, .offsets = &.{1} };
    try std.testing.expectError(error.InvalidMessagingOffsets, messaging_offsets.rebase(proof, 2, &out));
}
