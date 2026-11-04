//! The truncation of the blocks of the Ethereum Foundation's zkevm fixtures: every block that the
//! fixtures accept is truncated to the hash, the parent hash, the prevRandao, the timestamp, the
//! senders and the signature-stripped transactions that the fixtures list for it, and the blocks of
//! a case encode to the payload of the specified layout; a block with a blob transaction is
//! rejected.

const std = @import("std");

const ef_blocks = @import("ef_blocks");
const ef_fixtures = @import("ef_fixtures");
const executor = @import("zesu_executor");
const lineth_da = @import("lineth_da");

const truncation = lineth_da.encoding.truncation;
const rlp_encode = executor.executor_rlp_encode;

const Allocator = std.mem.Allocator;

const gpa = std.testing.allocator;
const io = std.testing.io;

/// The legacy field list of `tx`, or its type byte and typed field list, without the signature and
/// the chain ID; the authorizations of a set-code transaction stay whole.
fn strippedOf(a: Allocator, tx: ef_blocks.Transaction) ![]u8 {
    var fields: std.ArrayList([]const u8) = .empty;
    try fields.append(a, try rlp_encode.encodeU64(a, tx.nonce));
    if (tx.max_priority_fee_per_gas) |tip| try fields.append(a, try rlp_encode.encodeU256(a, tip));
    try fields.append(a, try rlp_encode.encodeU256(a, tx.gas_price));
    try fields.append(a, try rlp_encode.encodeU64(a, tx.gas_limit));
    try fields.append(a, try rlp_encode.encodeBytes(a, if (tx.to) |*to| to else &.{}));
    try fields.append(a, try rlp_encode.encodeU256(a, tx.value));
    try fields.append(a, try rlp_encode.encodeBytes(a, tx.data));
    if (tx.type != 0) {
        var access_list: std.ArrayList([]const u8) = .empty;
        for (tx.access_list) |entry| {
            var keys: std.ArrayList([]const u8) = .empty;
            for (entry.storage_keys) |*key| try keys.append(a, try rlp_encode.encodeBytes(a, key));
            const item = [_][]const u8{ try rlp_encode.encodeBytes(a, &entry.address), try rlp_encode.encodeList(a, keys.items) };
            try access_list.append(a, try rlp_encode.encodeList(a, &item));
        }
        try fields.append(a, try rlp_encode.encodeList(a, access_list.items));
    }
    if (tx.type == 4) {
        var authorizations: std.ArrayList([]const u8) = .empty;
        for (tx.authorization_list) |authorization| {
            const item = [_][]const u8{
                try rlp_encode.encodeU256(a, authorization.chain_id),
                try rlp_encode.encodeBytes(a, &authorization.address),
                try rlp_encode.encodeU64(a, authorization.nonce),
                try rlp_encode.encodeU256(a, authorization.y_parity),
                try rlp_encode.encodeU256(a, authorization.r),
                try rlp_encode.encodeU256(a, authorization.s),
            };
            try authorizations.append(a, try rlp_encode.encodeList(a, &item));
        }
        try fields.append(a, try rlp_encode.encodeList(a, authorizations.items));
    }
    const list = try rlp_encode.encodeList(a, fields.items);
    if (tx.type == 0) return list;
    return std.mem.concat(a, u8, &.{ &.{tx.type}, list });
}

/// A block the fixtures accept, with its stripped transactions derived from the fields they list.
const Expected = struct {
    fact: ef_blocks.Fact,
    stripped: []const []const u8,
};

fn expectedOf(a: Allocator, fact: ef_blocks.Fact) !Expected {
    const stripped = try a.alloc([]const u8, fact.transactions.len);
    for (fact.transactions, stripped) |tx, *item| item.* = try strippedOf(a, tx);
    return .{ .fact = fact, .stripped = stripped };
}

/// `[[timestamp, blockHash, prevRandao, [transaction…], [from…]]…]`, from what the fixtures list.
fn payloadOf(a: Allocator, blocks: []const Expected) ![]u8 {
    var entries: std.ArrayList([]const u8) = .empty;
    for (blocks) |block| {
        var transactions: std.ArrayList([]const u8) = .empty;
        for (block.stripped) |transaction| try transactions.append(a, try rlp_encode.encodeBytes(a, transaction));
        var froms: std.ArrayList([]const u8) = .empty;
        for (block.fact.senders) |*sender| try froms.append(a, try rlp_encode.encodeBytes(a, sender));
        const entry = [_][]const u8{
            try rlp_encode.encodeU64(a, block.fact.timestamp),
            try rlp_encode.encodeBytes(a, &block.fact.hash),
            try rlp_encode.encodeBytes(a, &block.fact.prev_randao),
            try rlp_encode.encodeList(a, transactions.items),
            try rlp_encode.encodeList(a, froms.items),
        };
        try entries.append(a, try rlp_encode.encodeList(a, &entry));
    }
    return rlp_encode.encodeList(a, entries.items);
}

/// Truncates the block of `expected` and compares the result with what the fixture lists for it.
fn checkBlock(scratch: Allocator, arena: Allocator, chain_id: u64, expected: Expected) !truncation.TruncatedBlock {
    const fact = expected.fact;
    const truncated = try truncation.truncateBlockRlp(arena, scratch, fact.rlp, chain_id);

    try std.testing.expectEqualSlices(u8, &fact.hash, &truncated.block_hash);
    try std.testing.expectEqualSlices(u8, &fact.parent_hash, &truncated.parent_hash);
    try std.testing.expectEqualSlices(u8, &fact.prev_randao, &truncated.prev_randao);
    try std.testing.expectEqual(fact.timestamp, truncated.timestamp);

    try std.testing.expectEqual(fact.senders.len, truncated.froms.len);
    for (fact.senders, truncated.froms) |sender, from| try std.testing.expectEqualSlices(u8, &sender, &from);
    try std.testing.expectEqual(expected.stripped.len, truncated.transactions.len);
    for (expected.stripped, truncated.transactions) |stripped, transaction| try std.testing.expectEqualSlices(u8, stripped, transaction);
    return truncated;
}

/// Truncates the block of `fact`, which carries a blob transaction, and expects its rejection.
fn checkRejected(scratch: Allocator, arena: Allocator, chain_id: u64, fact: ef_blocks.Fact) !void {
    try std.testing.expectError(error.MalformedBlock, truncation.truncateBlockRlp(arena, scratch, fact.rlp, chain_id));
}

/// The blocks of one case, truncated, encode to the payload of the blocks' listed facts.
fn checkPayload(scratch: Allocator, arena: Allocator, expected: []const Expected, truncated: []const truncation.TruncatedBlock) !void {
    const payload = try truncation.encodeBlocks(arena, scratch, truncated);
    try std.testing.expectEqualSlices(u8, try payloadOf(arena, expected), payload);
}

/// The mismatches found so far; the first twenty are printed.
const Failures = struct {
    count: usize = 0,

    fn record(self: *Failures, case: ef_blocks.Case, what: []const u8, err: anyerror) void {
        self.count += 1;
        if (self.count <= 20) std.debug.print("{s}: {s}: {s}{s}\n", .{ case.path, case.name, what, @errorName(err) });
    }
};

test "every block the fixtures accept truncates to the facts they list" {
    var corpus = try ef_blocks.Corpus.open(gpa, io, ef_fixtures.amsterdam_blocks);
    defer corpus.deinit();

    var truncated: usize = 0;
    var rejected: usize = 0;
    var skipped: usize = 0;
    var failures: Failures = .{};
    while (try corpus.next()) |case| {
        var arena = std.heap.ArenaAllocator.init(gpa);
        defer arena.deinit();
        var scratch = std.heap.ArenaAllocator.init(gpa);
        defer scratch.deinit();
        var expected_blocks: std.ArrayList(Expected) = .empty;
        var truncated_blocks: std.ArrayList(truncation.TruncatedBlock) = .empty;

        for (case.blocks) |block| {
            _ = scratch.reset(.retain_capacity);
            const fact = block orelse {
                skipped += 1;
                continue;
            };
            if (fact.blob_transaction) {
                rejected += 1;
                checkRejected(scratch.allocator(), arena.allocator(), case.chain_id, fact) catch |err| failures.record(case, "", err);
                continue;
            }
            truncated += 1;
            const expected = try expectedOf(arena.allocator(), fact);
            const result = checkBlock(scratch.allocator(), arena.allocator(), case.chain_id, expected) catch |err| {
                failures.record(case, "", err);
                continue;
            };
            try expected_blocks.append(arena.allocator(), expected);
            try truncated_blocks.append(arena.allocator(), result);
        }
        _ = scratch.reset(.retain_capacity);
        checkPayload(scratch.allocator(), arena.allocator(), expected_blocks.items, truncated_blocks.items) catch |err| failures.record(case, "payload: ", err);
    }
    std.debug.print("{d} blocks truncated, {d} blocks with a blob transaction rejected, {d} invalid or undecodable blocks skipped\n", .{ truncated, rejected, skipped });
    try std.testing.expect(truncated > 1000);
    try std.testing.expect(rejected > 0);
    try std.testing.expectEqual(0, failures.count);
}
