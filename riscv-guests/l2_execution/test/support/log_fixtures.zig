//! L2MessageService log builders shared by the guest's test suites.

const std = @import("std");

const executor = @import("zesu_executor");
const mpt = @import("zesu_mpt");

const types = executor.executor_types;

/// A log at a fixed placeholder position: the guest's message extraction only inspects
/// `address` and `topics`. `topics` is duplicated onto `alloc`, as a real per-block execution's
/// own log construction would.
pub fn makeLog(alloc: std.mem.Allocator, address: [20]u8, topics: []const [32]u8) !types.Log {
    const zero_hash: [32]u8 = @splat(0);
    return .{
        .address = address,
        .topics = try alloc.dupe([32]u8, topics),
        .data = &.{},
        .block_number = 0,
        .tx_hash = zero_hash,
        .tx_index = 0,
        .block_hash = zero_hash,
        .log_index = 0,
    };
}

/// A `RollingHashUpdated(uint256 indexed messageNumber, bytes32 indexed rollingHash)` log; topic0 is
/// the keccak256 of the event signature.
pub fn rollingHashUpdatedLog(alloc: std.mem.Allocator, address: [20]u8, number: u256, rolling_hash: [32]u8) !types.Log {
    var number_topic: [32]u8 = undefined;
    std.mem.writeInt(u256, &number_topic, number, .big);
    return makeLog(alloc, address, &.{ mpt.keccak256("RollingHashUpdated(uint256,bytes32)"), number_topic, rolling_hash });
}
