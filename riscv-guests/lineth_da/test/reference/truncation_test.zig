//! What `encoding.truncation` rejects, on real blocks of the EF zkevm fixtures: each case edits one
//! thing in the bytes of a selected block. An edit that changes the length of an item re-encodes
//! the lists around it, and an edit of a transaction rewrites the transactions root of the header,
//! so that the edit itself decides the result.

const std = @import("std");

const ef_conflations = @import("ef_conflations");
const ef_fixtures = @import("ef_fixtures");
const executor = @import("zesu_executor");
const lineth_da = @import("lineth_da");
const mpt = @import("zesu_mpt");

const truncation = lineth_da.encoding.truncation;
const rlp = mpt.rlp;
const rlp_encode = executor.executor_rlp_encode;

const Allocator = std.mem.Allocator;

const gpa = std.testing.allocator;

/// The positions of the header fields the edits address.
const header_field = struct {
    const parent_hash = 0;
    const transactions_root = 4;
    const number = 8;
    const timestamp = 11;
    const prev_randao = 13;
    const blob_gas_used = 17;
    const excess_blob_gas = 18;
    const slot_number = 22;
};

/// The encodings of the items of the list `encoded`.
fn itemsOf(a: Allocator, encoded: []const u8) ![]const []const u8 {
    var rest = switch ((try rlp.decodeItem(encoded)).item) {
        .list => |payload| payload,
        .bytes => return error.NotAList,
    };
    var items: std.ArrayList([]const u8) = .empty;
    while (rest.len > 0) {
        const item = try rlp.decodeItem(rest);
        try items.append(a, rest[0..item.consumed]);
        rest = rest[item.consumed..];
    }
    return items.items;
}

fn replaced(a: Allocator, items: []const []const u8, index: usize, item: []const u8) ![]const []const u8 {
    const copy = try a.dupe([]const u8, items);
    copy[index] = item;
    return copy;
}

fn appended(a: Allocator, items: []const []const u8, item: []const u8) ![]const []const u8 {
    return std.mem.concat(a, []const u8, &.{ items, &.{item} });
}

fn without(a: Allocator, items: []const []const u8, index: usize) ![]const []const u8 {
    return std.mem.concat(a, []const u8, &.{ items[0..index], items[index + 1 ..] });
}

fn string(a: Allocator, bytes: []const u8) ![]u8 {
    return rlp_encode.encodeBytes(a, bytes);
}

fn integer(a: Allocator, value: u256) ![]u8 {
    return rlp_encode.encodeU256(a, value);
}

/// A block as the encodings of the items of its four lists: an edit replaces some items and
/// `encode` rebuilds the lists around them.
const Block = struct {
    header: []const []const u8,
    transactions: []const []const u8,
    ommers: []const []const u8,
    withdrawals: []const []const u8,

    fn parse(a: Allocator, encoded: []const u8) !Block {
        const lists = try itemsOf(a, encoded);
        if (lists.len != 4) return error.NotABlock;
        return .{
            .header = try itemsOf(a, lists[0]),
            .transactions = try itemsOf(a, lists[1]),
            .ommers = try itemsOf(a, lists[2]),
            .withdrawals = try itemsOf(a, lists[3]),
        };
    }

    fn parts(self: Block, a: Allocator) ![]const []const u8 {
        return a.dupe([]const u8, &.{
            try rlp_encode.encodeList(a, self.header),
            try rlp_encode.encodeList(a, self.transactions),
            try rlp_encode.encodeList(a, self.ommers),
            try rlp_encode.encodeList(a, self.withdrawals),
        });
    }

    fn encode(self: Block, a: Allocator) ![]u8 {
        return rlp_encode.encodeList(a, try self.parts(a));
    }

    /// The block whose header commits to its transactions: the 32 bytes of the transactions root
    /// are replaced by the root of the transactions trie.
    fn committed(self: Block, a: Allocator) !Block {
        const wire = try a.alloc([]const u8, self.transactions.len);
        for (self.transactions, wire) |entry, *form| form.* = switch ((try rlp.decodeItem(entry)).item) {
            .list => entry,
            .bytes => |envelope| envelope,
        };
        const root = try executor.computeRawTxRoot(a, wire);
        var block = self;
        block.header = try replaced(a, self.header, header_field.transactions_root, try string(a, &root));
        return block;
    }
};

/// A transaction of a block's list as its field items: a legacy transaction is a list of them, a
/// typed one a byte string of its type and a list of them.
const Transaction = struct {
    tx_type: ?u8,
    fields: []const []const u8,

    fn parse(a: Allocator, entry: []const u8) !Transaction {
        return switch ((try rlp.decodeItem(entry)).item) {
            .list => .{ .tx_type = null, .fields = try itemsOf(a, entry) },
            .bytes => |envelope| .{ .tx_type = envelope[0], .fields = try itemsOf(a, envelope[1..]) },
        };
    }

    fn encode(self: Transaction, a: Allocator) ![]u8 {
        const list = try rlp_encode.encodeList(a, self.fields);
        const tx_type = self.tx_type orelse return list;
        return string(a, try std.mem.concat(a, u8, &.{ &.{tx_type}, list }));
    }

    /// The transaction with the field that is `from_end` fields from the end replaced; 1 is the last.
    fn withLastField(self: Transaction, a: Allocator, from_end: usize, item: []const u8) !Transaction {
        return .{ .tx_type = self.tx_type, .fields = try replaced(a, self.fields, self.fields.len - from_end, item) };
    }
};

/// The truncation of `block_rlp` under the chain `chain_id`.
fn truncate(a: Allocator, chain_id: u64, block_rlp: []const u8) truncation.Error!truncation.TruncatedBlock {
    var scratch = std.heap.ArenaAllocator.init(gpa);
    defer scratch.deinit();
    return truncation.truncateBlockRlp(a, scratch.allocator(), block_rlp, chain_id);
}

fn expectRejected(a: Allocator, expected: truncation.Error, chain_id: u64, block: Block) !void {
    try std.testing.expectError(expected, truncate(a, chain_id, try block.encode(a)));
}

fn expectAccepted(a: Allocator, chain_id: u64, block: Block) !truncation.TruncatedBlock {
    return truncate(a, chain_id, try block.encode(a));
}

fn selection() !*const ef_conflations.Selection {
    return ef_conflations.shared(std.testing.io, ef_fixtures.amsterdam_blocks);
}

test "bytes after the block are rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.empty_block);

    _ = try truncate(a, entry.chain_id, entry.rlp);
    try std.testing.expectError(error.MalformedBlock, truncate(a, entry.chain_id, try std.mem.concat(a, u8, &.{ entry.rlp, &.{0} })));
}

test "a block of other than four parts is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.empty_block);
    const parts = try (try Block.parse(a, entry.rlp)).parts(a);

    try std.testing.expectEqualSlices(u8, entry.rlp, try rlp_encode.encodeList(a, parts));
    try std.testing.expectError(error.MalformedBlock, truncate(a, entry.chain_id, try rlp_encode.encodeList(a, parts[0..3])));
    try std.testing.expectError(error.MalformedBlock, truncate(a, entry.chain_id, try rlp_encode.encodeList(a, try appended(a, parts, &.{0xc0}))));
}

test "a header has Osaka's fields or Amsterdam's" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.set_code);
    var block = try Block.parse(a, entry.rlp);
    const fields = block.header;

    const amsterdam = try expectAccepted(a, entry.chain_id, block);
    try std.testing.expect(amsterdam.froms.len > 0);
    block.header = fields[0 .. fields.len - 2];
    const osaka = try expectAccepted(a, entry.chain_id, block);
    try std.testing.expectEqualSlices(u8, &amsterdam.parent_hash, &osaka.parent_hash);
    try std.testing.expectEqualSlices(u8, &amsterdam.prev_randao, &osaka.prev_randao);
    try std.testing.expectEqual(amsterdam.timestamp, osaka.timestamp);
    try std.testing.expectEqualDeep(amsterdam.transactions, osaka.transactions);
    try std.testing.expectEqualDeep(amsterdam.froms, osaka.froms);

    block.header = fields[0 .. fields.len - 1];
    try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
    block.header = try appended(a, fields, &.{0x01});
    try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
    block.header = fields[0 .. fields.len - 3];
    try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
}

test "a header field of the wrong width is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.empty_block);
    const base = try Block.parse(a, entry.rlp);
    const narrow_bytes = [_]u8{0x11} ** 31;
    const fields_of_64_bits = [_]usize{ header_field.timestamp, header_field.blob_gas_used, header_field.excess_blob_gas, header_field.slot_number };

    var largest = base;
    for (fields_of_64_bits) |index| largest.header = try replaced(a, largest.header, index, try integer(a, std.math.maxInt(u64)));
    const accepted = try expectAccepted(a, entry.chain_id, largest);
    try std.testing.expectEqual(std.math.maxInt(u64), accepted.timestamp);

    for (fields_of_64_bits) |index| {
        var wide = base;
        wide.header = try replaced(a, base.header, index, try integer(a, 1 << 64));
        try expectRejected(a, error.MalformedBlock, entry.chain_id, wide);
    }
    var timestamp_list = base;
    timestamp_list.header = try replaced(a, base.header, header_field.timestamp, &.{0xc0});
    try expectRejected(a, error.MalformedBlock, entry.chain_id, timestamp_list);
    for ([_]usize{ header_field.parent_hash, header_field.prev_randao }) |index| {
        var narrow = base;
        narrow.header = try replaced(a, base.header, index, try string(a, &narrow_bytes));
        try expectRejected(a, error.MalformedBlock, entry.chain_id, narrow);
    }
}

test "non-canonical RLP is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.empty_block);
    var block = try Block.parse(a, entry.rlp);

    // An integer with a leading zero byte.
    const number = (try rlp.decodeItem(block.header[header_field.number])).item.bytes;
    var padded = block;
    padded.header = try replaced(a, block.header, header_field.number, try string(a, try std.mem.concat(a, u8, &.{ &.{0}, number })));
    try expectRejected(a, error.MalformedBlock, entry.chain_id, padded);

    // A long-form length for an item of 32 bytes, which the short form holds.
    const parent_hash = block.header[header_field.parent_hash];
    const long_form = try std.mem.concat(a, u8, &.{ &.{ 0xb8, 32 }, parent_hash[1..] });
    block.header = try replaced(a, block.header, header_field.parent_hash, long_form);
    try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
}

/// An item whose long-form header declares the payload length `maxInt(u64) - 8 + excess`: added to
/// the nine bytes of the header, the length wraps around to `excess`.
fn wrappingLength(first: u8, excess: u3) [9]u8 {
    var item: [9]u8 = undefined;
    item[0] = first;
    std.mem.writeInt(u64, item[1..9], std.math.maxInt(u64) - 8 + @as(u64, excess), .big);
    return item;
}

test "an item declaring a length that wraps around is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const selected = try selection();
    const entry = selected.entries.get(.empty_block);
    const legacy = selected.entries.get(.legacy);
    const header_block = try Block.parse(a, entry.rlp);
    const legacy_block = try Block.parse(a, legacy.rlp);
    const legacy_tx = try Transaction.parse(a, legacy_block.transactions[0]);

    for ([_]u8{ 0xbf, 0xff }) |first| {
        for (0..8) |excess| {
            const wrapping = wrappingLength(first, @intCast(excess));

            // The block itself, a header field, an entry of the transactions and a field of a transaction.
            try std.testing.expectError(error.MalformedBlock, truncate(a, entry.chain_id, &wrapping));
            var in_header = header_block;
            in_header.header = try replaced(a, header_block.header, header_field.number, &wrapping);
            try expectRejected(a, error.MalformedBlock, entry.chain_id, in_header);
            var in_transactions = legacy_block;
            in_transactions.transactions = try appended(a, legacy_block.transactions, &wrapping);
            try expectRejected(a, error.MalformedBlock, legacy.chain_id, in_transactions);
            const data_field = 5;
            const nested: Transaction = .{ .tx_type = null, .fields = try replaced(a, legacy_tx.fields, data_field, &wrapping) };
            var in_transaction = legacy_block;
            in_transaction.transactions = &.{try nested.encode(a)};
            try expectRejected(a, error.MalformedBlock, legacy.chain_id, in_transaction);
        }
    }
}

test "ommers that are not headers are rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.empty_block);
    var block = try Block.parse(a, entry.rlp);
    const fields = block.header;

    block.ommers = &.{try rlp_encode.encodeList(a, fields)};
    _ = try expectAccepted(a, entry.chain_id, block);
    block.ommers = &.{&.{0x01}};
    try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
    block.ommers = &.{try rlp_encode.encodeList(a, fields[0 .. fields.len - 1])};
    try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
}

test "withdrawals of the wrong shape are rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.empty_block);
    var block = try Block.parse(a, entry.rlp);

    // Index, validator index, address and amount.
    const address = try string(a, &([_]u8{0xc3} ** 20));
    const largest = [_][]const u8{ try integer(a, std.math.maxInt(u64)), try integer(a, std.math.maxInt(u64)), address, try integer(a, std.math.maxInt(u256)) };
    block.withdrawals = &.{try rlp_encode.encodeList(a, &largest)};
    _ = try expectAccepted(a, entry.chain_id, block);

    const wrong = [_][]const []const u8{
        largest[0..3],
        &.{ try integer(a, 1 << 64), largest[1], largest[2], largest[3] },
        &.{ largest[0], largest[1], try string(a, &([_]u8{0xc3} ** 19)), largest[3] },
        &.{ largest[0], largest[1], largest[2], try string(a, &([_]u8{1} ** 33)) },
    };
    for (wrong) |fields| {
        block.withdrawals = &.{try rlp_encode.encodeList(a, fields)};
        try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
    }
}

test "a legacy transaction carried as a byte string is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.legacy);
    var block = try Block.parse(a, entry.rlp);

    _ = try expectAccepted(a, entry.chain_id, block);
    block.transactions = &.{try string(a, block.transactions[0])};
    try expectRejected(a, error.MalformedBlock, entry.chain_id, block);
}

test "an envelope of a type outside access-list, fee-market and set-code, or without a body, is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.fee_market);
    var block = try Block.parse(a, entry.rlp);
    const envelope = (try rlp.decodeItem(block.transactions[0])).item.bytes;

    _ = try expectAccepted(a, entry.chain_id, block);
    for ([_]u8{ 0x00, 0x05, 0x7f, 0xff }) |tx_type| {
        block.transactions = &.{try string(a, try std.mem.concat(a, u8, &.{ &.{tx_type}, envelope[1..] }))};
        try expectRejected(a, error.MalformedBlock, entry.chain_id, try block.committed(a));
    }
    block.transactions = &.{try string(a, envelope[0..1])};
    try expectRejected(a, error.MalformedBlock, entry.chain_id, try block.committed(a));
}

test "transactions other than the ones the header commits to are rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.many_transactions);
    const base = try Block.parse(a, entry.rlp);
    const transactions = base.transactions;

    var swapped = base;
    swapped.transactions = try replaced(a, try replaced(a, transactions, 0, transactions[1]), 1, transactions[0]);
    var dropped = base;
    dropped.transactions = try without(a, transactions, 0);
    var added = base;
    added.transactions = try appended(a, transactions, transactions[transactions.len - 1]);

    for ([_]Block{ swapped, dropped, added }) |block| {
        try expectRejected(a, error.MalformedBlock, entry.chain_id, block);

        // Under a header that commits to them, the same transactions are accepted.
        const accepted = try expectAccepted(a, entry.chain_id, try block.committed(a));
        try std.testing.expectEqual(block.transactions.len, accepted.transactions.len);
    }
    const swapped_result = try expectAccepted(a, entry.chain_id, try swapped.committed(a));
    try std.testing.expectEqualSlices(u8, entry.truncated.transactions[1], swapped_result.transactions[0]);
    try std.testing.expectEqualSlices(u8, entry.truncated.transactions[0], swapped_result.transactions[1]);
}

test "a blob transaction is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const selected = try selection();
    const blob = selected.blob_transaction;
    const entry = selected.entries.get(.fee_market);

    try std.testing.expectError(error.MalformedBlock, truncate(a, blob.chain_id, blob.fact.rlp));

    // The blob transaction among accepted ones, under a header that commits to them.
    var block = try Block.parse(a, entry.rlp);
    const blob_entry = (try Block.parse(a, blob.fact.rlp)).transactions[0];
    block.transactions = try appended(a, block.transactions, block.transactions[0]);
    _ = try expectAccepted(a, entry.chain_id, try block.committed(a));
    block.transactions = try replaced(a, block.transactions, 1, blob_entry);
    try expectRejected(a, error.MalformedBlock, entry.chain_id, try block.committed(a));
}

test "the v of a legacy transaction is 27, 28 or one of the chain's two EIP-155 values" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.legacy);
    const base = try Block.parse(a, entry.rlp);
    const legacy_tx = try Transaction.parse(a, base.transactions[0]);

    const protected: u256 = 35 + 2 * @as(u256, entry.chain_id);
    const accepted = [_]u256{ 27, 28, protected, protected + 1 };
    const rejected = [_]u256{
        0,
        1,
        26,
        29,
        34,
        protected - 1,
        protected + 2,
        // Beyond the chain IDs of a 64-bit integer, and the widest integer there is.
        2 * (@as(u256, 1) << 64) + 35,
        std.math.maxInt(u256),
    };
    const v_from_end = 3;
    for (accepted ++ rejected, 0..) |v, i| {
        var block = base;
        block.transactions = &.{try (try legacy_tx.withLastField(a, v_from_end, try integer(a, v))).encode(a)};
        block = try block.committed(a);
        if (i < accepted.len) {
            _ = try expectAccepted(a, entry.chain_id, block);
        } else {
            try expectRejected(a, error.SenderRecoveryFailed, entry.chain_id, block);
        }
    }
}

test "a chain ID of zero is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const selected = try selection();

    for ([_]ef_conflations.Kind{ .empty_block, .legacy, .fee_market }) |kind| {
        try std.testing.expectError(error.SenderRecoveryFailed, truncate(a, 0, selected.entries.get(kind).rlp));
    }
}

test "a signature that recovers no sender is rejected" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.fee_market);
    var block = try Block.parse(a, entry.rlp);
    const fee_market_tx = try Transaction.parse(a, block.transactions[0]);

    // The signature closes the fields with its y parity, r and s.
    const r_from_end = 2;
    block.transactions = &.{try (try fee_market_tx.withLastField(a, r_from_end, &.{0x80})).encode(a)};
    try expectRejected(a, error.SenderRecoveryFailed, entry.chain_id, try block.committed(a));
}

test "a heap too small for a valid block is reported as exhausted whichever step runs out" {
    var arena = std.heap.ArenaAllocator.init(gpa);
    defer arena.deinit();
    const a = arena.allocator();
    const entry = (try selection()).entries.get(.set_code);

    var buffer: [1 << 20]u8 = undefined;
    var size: usize = 0;
    const truncated = while (size <= buffer.len) : (size += 16) {
        var heap = std.heap.FixedBufferAllocator.init(buffer[0..size]);
        break truncation.truncateBlockRlp(a, heap.allocator(), entry.rlp, entry.chain_id) catch |err| {
            try std.testing.expect(err == error.OutOfMemory);
            continue;
        };
    } else return error.TestUnexpectedResult;
    try std.testing.expect(size > 0);
    try std.testing.expectEqual(entry.truncated.transactions.len, truncated.transactions.len);
}
