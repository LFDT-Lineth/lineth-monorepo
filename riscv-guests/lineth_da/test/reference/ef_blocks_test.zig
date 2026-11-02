//! The truncation of the blocks of the Ethereum Foundation's zkevm fixtures: every block that the
//! fixtures accept is truncated to the hash, the parent hash, the prevRandao, the timestamp and the
//! senders the fixtures list for it.

const std = @import("std");

const ef_blocks = @import("ef_blocks");
const ef_fixtures = @import("ef_fixtures");
const lineth_da = @import("lineth_da");

const truncation = lineth_da.encoding.truncation;

const gpa = std.testing.allocator;
const io = std.testing.io;

/// Truncates the block of `fact` and compares the result with the facts the fixture lists for it.
fn checkBlock(arena: std.mem.Allocator, scratch: std.mem.Allocator, chain_id: u64, fact: ef_blocks.Fact) !void {
    const truncated = try truncation.truncateBlockRlp(arena, scratch, fact.rlp, chain_id);

    try std.testing.expectEqualSlices(u8, &fact.hash, &truncated.block_hash);
    try std.testing.expectEqualSlices(u8, &fact.parent_hash, &truncated.parent_hash);
    try std.testing.expectEqualSlices(u8, &fact.prev_randao, &truncated.prev_randao);
    try std.testing.expectEqual(fact.timestamp, truncated.timestamp);

    try std.testing.expectEqual(fact.senders.len, truncated.froms.len);
    try std.testing.expectEqual(fact.senders.len, truncated.transactions.len);
    for (fact.senders, truncated.froms) |sender, from| try std.testing.expectEqualSlices(u8, &sender, &from);
}

test "every block the fixtures accept truncates to the facts they list" {
    var corpus = try ef_blocks.Corpus.open(gpa, io, ef_fixtures.amsterdam_blocks);
    defer corpus.deinit();

    var blocks: usize = 0;
    var skipped: usize = 0;
    var failures: usize = 0;
    while (try corpus.next()) |case| {
        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        var scratch = std.heap.ArenaAllocator.init(gpa);
        defer scratch.deinit();

        for (case.blocks) |block| {
            const fact = block orelse {
                skipped += 1;
                continue;
            };
            blocks += 1;
            checkBlock(arena.allocator(), scratch.allocator(), case.chain_id, fact) catch |err| {
                failures += 1;
                if (failures <= 20) std.debug.print("{s}: {s}: {s}\n", .{ case.path, case.name, @errorName(err) });
            };
            _ = scratch.reset(.retain_capacity);
        }
    }
    std.debug.print("{d} blocks truncated, {d} invalid or undecodable blocks skipped\n", .{ blocks, skipped });
    try std.testing.expect(blocks > 1000);
    try std.testing.expectEqual(0, failures);
}
