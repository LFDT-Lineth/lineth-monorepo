//! The L1-to-L2 bridge position a proof ends at: the rolling hash of the messages and their count.

/// A rolling hash and the number of messages it covers.
pub const Pair = struct {
    rolling_hash: [32]u8,
    message_number: u64,

    /// The position before any message.
    pub const none: Pair = .{ .rolling_hash = @splat(0), .message_number = 0 };
};

/// The pair `public_input` ends at when its message number is non-zero, else `current`. Folding
/// over proofs in order from `Pair.none` yields the pair of the last proof that processed a message.
pub fn advance(current: Pair, public_input: anytype) Pair {
    if (public_input.end_l1_l2_bridge_rolling_hash_message_number == 0) return current;
    return .{
        .rolling_hash = public_input.end_l1_l2_bridge_rolling_hash,
        .message_number = public_input.end_l1_l2_bridge_rolling_hash_message_number,
    };
}
