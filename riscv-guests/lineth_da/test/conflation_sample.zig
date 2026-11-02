//! Sample conflations: the canonical payload of blocks with signed random transactions: each block
//! truncated by `encoding.truncation`, the truncated blocks encoded by `encodeBlocks`.

const std = @import("std");

const lineth_da = @import("lineth_da");
const support = @import("support");

const truncation = lineth_da.encoding.truncation;
const blocks = support.blocks;
const rlp = support.rlp;

const Allocator = std.mem.Allocator;
const Hash = [32]u8;

pub const chain_id = 59144;

fn secretKey(index: u8) [32]u8 {
    var secret: [32]u8 = undefined;
    std.crypto.hash.sha2.Sha256.hash(&.{ 'k', 'e', 'y', 'a', index }, &secret, .{});
    return secret;
}

/// The canonical payload of a conflation of `count` blocks, each with one to five signed random
/// transactions, drawn from `random`.
pub fn conflationPayload(a: Allocator, random: std.Random, count: usize) ![]u8 {
    const kinds = std.enums.values(blocks.Kind);
    const truncated = try a.alloc(truncation.TruncatedBlock, count);
    for (truncated, 0..) |*slot, number| {
        const items = try a.alloc(rlp.Item, 1 + random.uintLessThan(usize, 5));
        for (items) |*item| {
            const tx = try blocks.randomTx(a, random, kinds[random.uintLessThan(usize, kinds.len)]);
            item.* = try blocks.signed(a, tx, chain_id, secretKey(random.uintLessThan(u8, 4)));
        }
        const header: blocks.Header = .{
            .parent_hash = blocks.randomBytes(Hash, random),
            .prev_randao = blocks.randomBytes(Hash, random),
            .number = 100 + number,
            .timestamp = 1_700_000_000 + 12 * number,
        };
        var scratch = std.heap.ArenaAllocator.init(a);
        defer scratch.deinit();
        const committed = try blocks.withTransactions(a, header, items);
        slot.* = try truncation.truncateBlockRlp(a, scratch.allocator(), try blocks.block(a, committed, items, &.{}), chain_id);
    }
    return truncation.encodeBlocks(a, truncated);
}

/// The payload of a fixed conflation of three blocks.
pub fn fixedConflationPayload(a: Allocator) ![]u8 {
    var prng = std.Random.DefaultPrng.init(2026);
    return conflationPayload(a, prng.random(), 3);
}
