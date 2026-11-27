//! The L1-to-L2 bridge pair a sequence of proofs ends at.

const std = @import("std");

const l1_l2_bridge = @import("guest_common").l1_l2_bridge;

const PublicInput = struct {
    end_l1_l2_bridge_rolling_hash: [32]u8,
    end_l1_l2_bridge_rolling_hash_message_number: u64,
};

fn publicInput(hash_byte: u8, message_number: u64) PublicInput {
    return .{ .end_l1_l2_bridge_rolling_hash = @splat(hash_byte), .end_l1_l2_bridge_rolling_hash_message_number = message_number };
}

fn fold(public_inputs: []const PublicInput) l1_l2_bridge.Pair {
    var pair: l1_l2_bridge.Pair = .none;
    for (public_inputs) |public_input| pair = l1_l2_bridge.advance(pair, public_input);
    return pair;
}

test "the pair is the one of the last proof with a non-zero message number" {
    const pair = fold(&.{ publicInput(0x01, 3), publicInput(0x02, 7), publicInput(0x03, 0) });
    try std.testing.expectEqual(@as(u64, 7), pair.message_number);
    try std.testing.expectEqualSlices(u8, &@as([32]u8, @splat(0x02)), &pair.rolling_hash);
}

test "the pair is zero when no proof has a message number" {
    const pair = fold(&.{ publicInput(0x01, 0), publicInput(0x02, 0) });
    try std.testing.expectEqual(@as(u64, 0), pair.message_number);
    try std.testing.expectEqualSlices(u8, &@as([32]u8, @splat(0)), &pair.rolling_hash);
}

test "a zero message number leaves its rolling hash out of the pair" {
    const pair = fold(&.{ publicInput(0x01, 5), publicInput(0xff, 0) });
    try std.testing.expectEqual(@as(u64, 5), pair.message_number);
    try std.testing.expectEqualSlices(u8, &@as([32]u8, @splat(0x01)), &pair.rolling_hash);
}
