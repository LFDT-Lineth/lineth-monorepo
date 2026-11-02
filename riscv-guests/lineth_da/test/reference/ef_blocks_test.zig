//! The truncation of the blocks of the Ethereum Foundation's zkevm fixtures: every block that the
//! fixtures accept is truncated to the hash, the parent hash, the prevRandao, the timestamp and the
//! senders the fixtures list for it.

const std = @import("std");

const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");

const truncation = lineth_da.encoding.truncation;

const gpa = std.testing.allocator;
const io = std.testing.io;

const Header = struct {
    hash: []const u8,
    parentHash: []const u8,
    mixHash: []const u8,
    timestamp: []const u8,
};

const Transaction = struct {
    sender: []const u8,
};

const Block = struct {
    rlp: ?[]const u8 = null,
    blockHeader: ?Header = null,
    transactions: ?[]const Transaction = null,
    expectException: ?[]const u8 = null,
};

const Case = struct {
    config: struct { chainid: []const u8 } = .{ .chainid = "0x01" },
    blocks: []const Block = &.{},
};

/// The bytes of a `0x`-prefixed hex string.
fn decodeHex(allocator: std.mem.Allocator, text: []const u8) ![]u8 {
    const digits = if (std.mem.startsWith(u8, text, "0x")) text[2..] else text;
    const bytes = try allocator.alloc(u8, digits.len / 2);
    _ = try std.fmt.hexToBytes(bytes, digits);
    return bytes;
}

fn expectFixtureValue(comptime len: usize, expected: []const u8, actual: *const [len]u8) !void {
    var bytes: [len]u8 = undefined;
    _ = try std.fmt.hexToBytes(&bytes, if (std.mem.startsWith(u8, expected, "0x")) expected[2..] else expected);
    try std.testing.expectEqualSlices(u8, &bytes, actual);
}

/// Truncates `block` and compares the result with the facts the fixture lists for it.
fn checkBlock(arena: std.mem.Allocator, scratch: std.mem.Allocator, chain_id: u64, block: Block) !void {
    const header = block.blockHeader orelse return error.FixtureWithoutHeader;
    const block_rlp = try decodeHex(arena, block.rlp.?);
    const truncated = try truncation.truncateBlockRlp(arena, scratch, block_rlp, chain_id);

    try expectFixtureValue(32, header.hash, &truncated.block_hash);
    try expectFixtureValue(32, header.parentHash, &truncated.parent_hash);
    try expectFixtureValue(32, header.mixHash, &truncated.prev_randao);
    try std.testing.expectEqual(try std.fmt.parseInt(u64, header.timestamp, 0), truncated.timestamp);

    const transactions = block.transactions orelse &.{};
    try std.testing.expectEqual(transactions.len, truncated.froms.len);
    try std.testing.expectEqual(transactions.len, truncated.transactions.len);
    for (transactions, truncated.froms) |transaction, from| try expectFixtureValue(20, transaction.sender, &from);
}

test "every block the fixtures accept truncates to the facts they list" {
    var dir = try std.Io.Dir.openDirAbsolute(io, ef_fixtures.amsterdam_blocks, .{ .iterate = true });
    defer dir.close(io);
    var walker = try dir.walk(gpa);
    defer walker.deinit();

    var blocks: usize = 0;
    var skipped: usize = 0;
    var failures: usize = 0;
    while (try walker.next(io)) |entry| {
        if (entry.kind != .file or !std.mem.endsWith(u8, entry.basename, ".json")) continue;

        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        var scratch = std.heap.ArenaAllocator.init(gpa);
        defer scratch.deinit();
        const text = try dir.readFileAlloc(io, entry.path, arena.allocator(), .limited(1 << 30));
        const cases = try std.json.parseFromSliceLeaky(std.json.ArrayHashMap(Case), arena.allocator(), text, .{ .ignore_unknown_fields = true });

        for (cases.map.keys(), cases.map.values()) |name, case| {
            const chain_id = try std.fmt.parseInt(u64, case.config.chainid, 0);
            for (case.blocks) |block| {
                if (block.expectException != null or block.rlp == null) {
                    skipped += 1;
                    continue;
                }
                blocks += 1;
                checkBlock(arena.allocator(), scratch.allocator(), chain_id, block) catch |err| {
                    failures += 1;
                    if (failures <= 20) std.debug.print("{s}: {s}: {s}\n", .{ entry.path, name, @errorName(err) });
                };
                _ = scratch.reset(.retain_capacity);
            }
        }
    }
    std.debug.print("{d} blocks truncated, {d} invalid or undecodable blocks skipped\n", .{ blocks, skipped });
    try std.testing.expect(blocks > 1000);
    try std.testing.expectEqual(0, failures);
}
