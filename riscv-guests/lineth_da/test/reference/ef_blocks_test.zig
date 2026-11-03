//! The truncation of the blocks of the Ethereum Foundation's zkevm fixtures: every block that the
//! fixtures accept is truncated to the hash, the parent hash, the prevRandao, the timestamp and the
//! senders the fixtures list for it, except a block with a blob transaction, which is rejected.

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

/// Truncates the block of `fact`, which carries a blob transaction, and expects its rejection.
fn checkRejected(arena: std.mem.Allocator, scratch: std.mem.Allocator, chain_id: u64, fact: ef_blocks.Fact) !void {
    try std.testing.expectError(error.MalformedBlock, truncation.truncateBlockRlp(arena, scratch, fact.rlp, chain_id));
}

test "every block the fixtures accept truncates to the facts they list" {
    var corpus = try ef_blocks.Corpus.open(gpa, io, ef_fixtures.amsterdam_blocks);
    defer corpus.deinit();

    var truncated: usize = 0;
    var rejected: usize = 0;
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
            if (fact.blob_transaction) rejected += 1 else truncated += 1;
            const checked = if (fact.blob_transaction)
                checkRejected(arena.allocator(), scratch.allocator(), case.chain_id, fact)
            else
                checkBlock(arena.allocator(), scratch.allocator(), case.chain_id, fact);
            checked catch |err| {
                failures += 1;
                if (failures <= 20) std.debug.print("{s}: {s}: {s}\n", .{ case.path, case.name, @errorName(err) });
            };
            _ = scratch.reset(.retain_capacity);
        }
    }
    std.debug.print("{d} blocks truncated, {d} blocks with a blob transaction rejected, {d} invalid or undecodable blocks skipped\n", .{ truncated, rejected, skipped });
    try std.testing.expect(truncated > 1000);
    try std.testing.expect(rejected > 0);
    try std.testing.expectEqual(0, failures);
}
